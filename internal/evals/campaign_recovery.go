package evals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	RecoveryFormatVersion      = "1"
	RecoveryDefaultMaxAttempts = 5
	recoveryDefaultBackoff     = 30 * time.Second
	recoveryMaxBackoff         = 300 * time.Second
)

var (
	ErrInvalidCampaignRecovery = errors.New("invalid campaign recovery")
	ErrRecoveryBudgetExceeded  = errors.New("campaign recovery budget exceeded")
)

// RecoveryPolicy controls bounded, selective recovery. MaxAttemptsPerSlot
// includes the immutable original attempt, not just recovery attempts.
type RecoveryPolicy struct {
	MaxAttemptsPerSlot int           `json:"max_attempts_per_slot"`
	InitialBackoff     time.Duration `json:"initial_backoff_ns"`
	MaxBackoff         time.Duration `json:"max_backoff_ns"`
}

func DefaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{MaxAttemptsPerSlot: RecoveryDefaultMaxAttempts, InitialBackoff: recoveryDefaultBackoff, MaxBackoff: recoveryMaxBackoff}
}

func (p RecoveryPolicy) Validate() error {
	if p.MaxAttemptsPerSlot < 2 || p.MaxAttemptsPerSlot > RecoveryDefaultMaxAttempts {
		return fmt.Errorf("%w: max attempts per slot must be between 2 and %d", ErrInvalidCampaignRecovery, RecoveryDefaultMaxAttempts)
	}
	if p.InitialBackoff <= 0 || p.MaxBackoff < p.InitialBackoff || p.MaxBackoff > recoveryMaxBackoff {
		return fmt.Errorf("%w: invalid recovery backoff", ErrInvalidCampaignRecovery)
	}
	return nil
}

// Backoff returns a bounded exponential delay for the next recovery attempt.
// It is advisory: the CLI/runner remains responsible for waiting before I/O.
func (p RecoveryPolicy) Backoff(recoveryNumber int) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if recoveryNumber <= 0 || recoveryNumber >= p.MaxAttemptsPerSlot {
		return 0, fmt.Errorf("%w: invalid recovery attempt number", ErrInvalidCampaignRecovery)
	}
	delay := p.InitialBackoff
	for i := 1; i < recoveryNumber; i++ {
		if delay >= p.MaxBackoff/2 {
			return p.MaxBackoff, nil
		}
		delay *= 2
	}
	if delay > p.MaxBackoff {
		return p.MaxBackoff, nil
	}
	return delay, nil
}

// RecoveryBinding is the immutable identity required for a recovery artifact.
// Prompt and generation hashes are keyed by case/trial and must be provided by
// the caller from the frozen request configuration; failed provider calls may
// not have emitted input evidence themselves.
type RecoveryBinding struct {
	DatasetVersion               string            `json:"dataset_version"`
	DatasetManifestSHA256        string            `json:"dataset_manifest_sha256"`
	SourceCommit                 string            `json:"source_commit"`
	RequestedModel               string            `json:"requested_model"`
	ResolvedModel                string            `json:"resolved_model,omitempty"`
	BaselineFilesSHA256          map[string]string `json:"baseline_files_sha256"`
	PromptSHA256BySlot           map[string]string `json:"prompt_sha256_by_slot"`
	GenerationConfigSHA256BySlot map[string]string `json:"generation_config_sha256_by_slot"`
}

func (b RecoveryBinding) Validate() error {
	if strings.TrimSpace(b.DatasetVersion) == "" || len(b.DatasetManifestSHA256) != 64 || strings.TrimSpace(b.SourceCommit) == "" || strings.TrimSpace(b.RequestedModel) == "" {
		return fmt.Errorf("%w: incomplete recovery identity", ErrInvalidCampaignRecovery)
	}
	if len(b.BaselineFilesSHA256) != 4 {
		return fmt.Errorf("%w: exactly four baseline file attestations are required", ErrInvalidCampaignRecovery)
	}
	for path, hash := range b.BaselineFilesSHA256 {
		if strings.TrimSpace(path) == "" || len(hash) != 64 {
			return fmt.Errorf("%w: invalid baseline file hash", ErrInvalidCampaignRecovery)
		}
	}
	for slot, hash := range b.PromptSHA256BySlot {
		if strings.TrimSpace(slot) == "" || len(hash) != 64 {
			return fmt.Errorf("%w: invalid prompt hash for %s", ErrInvalidCampaignRecovery, slot)
		}
	}
	for slot, hash := range b.GenerationConfigSHA256BySlot {
		if strings.TrimSpace(slot) == "" || len(hash) != 64 {
			return fmt.Errorf("%w: invalid generation config hash for %s", ErrInvalidCampaignRecovery, slot)
		}
	}
	return nil
}

// RecoveryBudget reserves a conservative upper bound for prior and additional
// requests. PriorSpentUSD nil is intentionally treated as unknown and charged
// at the maximum configured token bound rather than as zero.
type RecoveryBudget struct {
	HardCeilingUSD         float64  `json:"hard_ceiling_usd"`
	PriorRequests          int      `json:"prior_requests"`
	PriorRequestsKnown     bool     `json:"prior_requests_known"`
	PriorSpentUSD          *float64 `json:"prior_spent_usd,omitempty"`
	MaxInputTokensPerCall  int64    `json:"max_input_tokens_per_call"`
	MaxOutputTokensPerCall int64    `json:"max_output_tokens_per_call"`
	InputUSDPerMillion     float64  `json:"input_usd_per_million"`
	OutputUSDPerMillion    float64  `json:"output_usd_per_million"`
}

type RecoveryBudgetReservation struct {
	PriorUpperBoundUSD float64 `json:"prior_upper_bound_usd"`
	AdditionalCalls    int     `json:"additional_calls"`
	AdditionalUpperUSD float64 `json:"additional_upper_bound_usd"`
	TotalUpperBoundUSD float64 `json:"total_upper_bound_usd"`
	RemainingUpperUSD  float64 `json:"remaining_upper_bound_usd"`
}

func (b RecoveryBudget) Reserve(additionalCalls int) (RecoveryBudgetReservation, error) {
	if !finite(b.HardCeilingUSD) || b.HardCeilingUSD <= 0 || !b.PriorRequestsKnown || b.PriorRequests < 0 || additionalCalls < 0 || b.MaxInputTokensPerCall <= 0 || b.MaxInputTokensPerCall > CampaignBudgetMaxInputTokens || b.MaxOutputTokensPerCall <= 0 || b.MaxOutputTokensPerCall > CampaignBudgetMaxOutputTokens || !finite(b.InputUSDPerMillion) || b.InputUSDPerMillion <= 0 || !finite(b.OutputUSDPerMillion) || b.OutputUSDPerMillion <= 0 {
		return RecoveryBudgetReservation{}, fmt.Errorf("%w: invalid budget declaration", ErrInvalidCampaignRecovery)
	}
	perCall := float64(b.MaxInputTokensPerCall)*b.InputUSDPerMillion/1_000_000 + float64(b.MaxOutputTokensPerCall)*b.OutputUSDPerMillion/1_000_000
	prior := float64(b.PriorRequests) * perCall
	if b.PriorSpentUSD != nil {
		if !finite(*b.PriorSpentUSD) || *b.PriorSpentUSD < 0 {
			return RecoveryBudgetReservation{}, fmt.Errorf("%w: invalid prior spend", ErrInvalidCampaignRecovery)
		}
		// Never understate a prior bound when observed spend is incomplete.
		if *b.PriorSpentUSD > prior {
			prior = *b.PriorSpentUSD
		}
	}
	additional := float64(additionalCalls) * perCall
	total := prior + additional
	result := RecoveryBudgetReservation{PriorUpperBoundUSD: prior, AdditionalCalls: additionalCalls, AdditionalUpperUSD: additional, TotalUpperBoundUSD: total, RemainingUpperUSD: b.HardCeilingUSD - total}
	if !finite(total) || total > b.HardCeilingUSD+1e-12 {
		return result, fmt.Errorf("%w: conservative reserve %.6f exceeds ceiling %.6f", ErrRecoveryBudgetExceeded, total, b.HardCeilingUSD)
	}
	return result, nil
}

// RecoveryManifest binds a recovery overlay to one immutable, completed run.
type RecoveryManifest struct {
	FormatVersion        string                    `json:"format_version"`
	RecoveryID           string                    `json:"recovery_id"`
	BaseRunID            string                    `json:"base_run_id"`
	BaseAttemptsSHA256   string                    `json:"base_attempts_sha256"`
	BaseProtocolSHA256   string                    `json:"base_protocol_sha256,omitempty"`
	AddendumSHA256       string                    `json:"addendum_sha256,omitempty"`
	RecoverySourceCommit string                    `json:"recovery_source_commit,omitempty"`
	Binding              RecoveryBinding           `json:"binding"`
	Policy               RecoveryPolicy            `json:"policy"`
	Budget               RecoveryBudget            `json:"budget"`
	Reservation          RecoveryBudgetReservation `json:"reservation"`
	Targets              []RecoveryTarget          `json:"targets"`
}

type RecoveryTarget struct {
	CaseID                string `json:"case_id"`
	Trial                 int    `json:"trial"`
	OriginalRetry         int    `json:"original_retry"`
	OriginalAttemptSHA256 string `json:"original_attempt_sha256"`
	Reason                string `json:"reason"`
}

// RecoveryAttempt is stored separately from the original attempts journal.
type RecoveryAttempt struct {
	Attempt           Attempt `json:"attempt"`
	BaseAttemptSHA256 string  `json:"base_attempt_sha256"`
	RecoveryNumber    int     `json:"recovery_number"`
}

type RecoveryBundle struct {
	Manifest RecoveryManifest  `json:"manifest"`
	Attempts []RecoveryAttempt `json:"attempts"`
}

type RecoveryCompleteness struct {
	ExpectedSlots  int      `json:"expected_slots"`
	EffectiveSlots int      `json:"effective_slots"`
	MissingSlots   []string `json:"missing_slots"`
	Complete       bool     `json:"complete"`
}

// BuildRecoveryManifest selects only terminal no-response transient failures.
// It never selects malformed content, quality failures or successful attempts.
func BuildRecoveryManifest(base RunRecord, baseAttempts []Attempt, binding RecoveryBinding, policy RecoveryPolicy, budget RecoveryBudget, recoveryID string) (RecoveryManifest, error) {
	if base.FinishedOn == nil || base.Plan == nil || strings.TrimSpace(base.RunID) == "" || strings.TrimSpace(base.AttemptsSHA256) == "" {
		return RecoveryManifest{}, fmt.Errorf("%w: base run must be completed and checksummed", ErrInvalidCampaignRecovery)
	}
	if err := binding.Validate(); err != nil {
		return RecoveryManifest{}, err
	}
	if binding.DatasetVersion != base.Plan.DatasetVersion || binding.DatasetManifestSHA256 != base.Plan.DatasetSHA256 || binding.RequestedModel != base.Plan.Model || binding.SourceCommit != base.Commit {
		return RecoveryManifest{}, fmt.Errorf("%w: recovery identity differs from base run", ErrInvalidCampaignRecovery)
	}
	if err := policy.Validate(); err != nil {
		return RecoveryManifest{}, err
	}
	terminals, err := terminalAttempts(baseAttempts)
	if err != nil {
		return RecoveryManifest{}, err
	}
	targets := make([]RecoveryTarget, 0)
	for key, a := range terminals {
		if !RecoverableNoResponse(a) {
			continue
		}
		if binding.PromptSHA256BySlot[key] == "" || binding.GenerationConfigSHA256BySlot[key] == "" {
			return RecoveryManifest{}, fmt.Errorf("%w: missing prompt/config hash for %s", ErrInvalidCampaignRecovery, key)
		}
		targets = append(targets, RecoveryTarget{CaseID: a.CaseID, Trial: a.Trial, OriginalRetry: a.Retry, OriginalAttemptSHA256: AttemptSHA256(a), Reason: RecoveryReason(a)})
	}
	slices.SortFunc(targets, func(a, b RecoveryTarget) int {
		return strings.Compare(attemptKey(a.CaseID, a.Trial), attemptKey(b.CaseID, b.Trial))
	})
	additionalCalls := len(targets) * (policy.MaxAttemptsPerSlot - 1)
	reservation, err := budget.Reserve(additionalCalls)
	if err != nil {
		return RecoveryManifest{}, err
	}
	if strings.TrimSpace(recoveryID) == "" {
		return RecoveryManifest{}, fmt.Errorf("%w: recovery id is required", ErrInvalidCampaignRecovery)
	}
	return RecoveryManifest{FormatVersion: RecoveryFormatVersion, RecoveryID: recoveryID, BaseRunID: base.RunID, BaseAttemptsSHA256: base.AttemptsSHA256, Binding: binding, Policy: policy, Budget: budget, Reservation: reservation, Targets: targets}, nil
}

// RecoverableNoResponse reports whether a terminal attempt is eligible for a
// transient recovery. It requires no model content; malformed/quality outputs
// therefore cannot enter this path. HTTP 503 and transient timeout wording are
// accepted, while arbitrary execution errors remain ineligible.
func RecoverableNoResponse(a Attempt) bool {
	if a.Status != "execution_error" || a.RequestCount != 1 || strings.TrimSpace(a.RawOutput) != "" || len(a.ProviderResponse) != 0 || len(a.ParsedOutput) != 0 {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(a.Error))
	if message == "" {
		return false
	}
	return strings.Contains(message, "503") || strings.Contains(message, "deadline exceeded") || strings.Contains(message, "deadline_exceeded") || strings.Contains(message, "timed out") || strings.Contains(message, "timeout")
}

func RecoveryReason(a Attempt) string {
	if strings.Contains(strings.ToLower(a.Error), "503") || strings.Contains(strings.ToLower(a.Error), "unavailable") {
		return "transient_provider_unavailable"
	}
	return "transient_request_timeout"
}

// AttemptSHA256 supplies a stable link without persisting secrets.
func AttemptSHA256(a Attempt) string {
	data, err := json.Marshal(a)
	if err != nil {
		return ""
	}
	return digest(data)
}

func terminalAttempts(attempts []Attempt) (map[string]Attempt, error) {
	result := map[string]Attempt{}
	for _, a := range attempts {
		if strings.TrimSpace(a.CaseID) == "" || a.Trial <= 0 || a.Retry < 0 {
			return nil, fmt.Errorf("%w: invalid base attempt identity", ErrInvalidCampaignRecovery)
		}
		key := attemptKey(a.CaseID, a.Trial)
		prior, ok := result[key]
		if !ok || a.Retry > prior.Retry {
			result[key] = a
		}
	}
	return result, nil
}

// ValidateRecovery verifies append-only attempts against the immutable base
// evidence and manifest. Recovery attempts never alter the original journal.
func ValidateRecovery(bundle RecoveryBundle, base RunRecord, baseAttempts []Attempt) error {
	m := bundle.Manifest
	if base.FinishedOn == nil || strings.TrimSpace(base.AttemptsSHA256) == "" {
		return fmt.Errorf("%w: base run must be completed and checksummed", ErrInvalidCampaignRecovery)
	}
	if m.FormatVersion != RecoveryFormatVersion || m.BaseRunID != base.RunID || m.BaseAttemptsSHA256 == "" || m.BaseAttemptsSHA256 != base.AttemptsSHA256 {
		return fmt.Errorf("%w: base run binding mismatch", ErrInvalidCampaignRecovery)
	}
	if err := m.Binding.Validate(); err != nil {
		return err
	}
	if err := m.Policy.Validate(); err != nil {
		return err
	}
	if base.Plan == nil || m.Binding.DatasetVersion != base.Plan.DatasetVersion || m.Binding.DatasetManifestSHA256 != base.Plan.DatasetSHA256 || m.Binding.SourceCommit != base.Commit || m.Binding.RequestedModel != base.Plan.Model {
		return fmt.Errorf("%w: dataset, source or model binding mismatch", ErrInvalidCampaignRecovery)
	}
	terminals, err := terminalAttempts(baseAttempts)
	if err != nil {
		return err
	}
	targets := map[string]RecoveryTarget{}
	for _, target := range m.Targets {
		key := attemptKey(target.CaseID, target.Trial)
		if _, exists := targets[key]; exists || target.OriginalAttemptSHA256 == "" || target.OriginalRetry < 0 {
			return fmt.Errorf("%w: duplicate or invalid recovery target %s", ErrInvalidCampaignRecovery, key)
		}
		original, ok := terminals[key]
		if !ok || original.Retry != target.OriginalRetry || AttemptSHA256(original) != target.OriginalAttemptSHA256 || !RecoverableNoResponse(original) {
			return fmt.Errorf("%w: target %s is not an eligible immutable transient failure", ErrInvalidCampaignRecovery, key)
		}
		if m.Binding.PromptSHA256BySlot[key] == "" || m.Binding.GenerationConfigSHA256BySlot[key] == "" {
			return fmt.Errorf("%w: missing prompt/config binding for %s", ErrInvalidCampaignRecovery, key)
		}
		targets[key] = target
	}
	terminalsExpected := map[string]bool{}
	for key, original := range terminals {
		if RecoverableNoResponse(original) {
			terminalsExpected[key] = true
		}
	}
	if len(targets) != len(terminalsExpected) {
		return fmt.Errorf("%w: recovery target set omits eligible transient slots", ErrInvalidCampaignRecovery)
	}
	for key := range terminalsExpected {
		if _, ok := targets[key]; !ok {
			return fmt.Errorf("%w: recovery target set omits %s", ErrInvalidCampaignRecovery, key)
		}
	}
	seen := map[string]int{}
	finished := map[string]bool{}
	for _, item := range bundle.Attempts {
		a := item.Attempt
		key := attemptKey(a.CaseID, a.Trial)
		target, ok := targets[key]
		if !ok || item.BaseAttemptSHA256 != target.OriginalAttemptSHA256 {
			return fmt.Errorf("%w: attempt is not linked to a declared target %s", ErrInvalidCampaignRecovery, key)
		}
		if finished[key] {
			return fmt.Errorf("%w: attempt follows an effective response for %s", ErrInvalidCampaignRecovery, key)
		}
		number := seen[key] + 1
		if item.RecoveryNumber != number || number >= m.Policy.MaxAttemptsPerSlot || a.Retry != target.OriginalRetry+number {
			return fmt.Errorf("%w: invalid recovery sequence for %s", ErrInvalidCampaignRecovery, key)
		}
		if a.RequestCount != 1 || (a.Status != "executed" && a.Status != "execution_error") {
			return fmt.Errorf("%w: invalid recovery attempt status/evidence for %s", ErrInvalidCampaignRecovery, key)
		}
		if a.Status == "executed" {
			if len(a.Input) == 0 || a.InputSHA256 == "" || a.PromptSHA256 == "" || len(a.GenerationConfig) == 0 || a.RequestCount != 1 {
				return fmt.Errorf("%w: executed recovery lacks request evidence for %s", ErrInvalidCampaignRecovery, key)
			}
			if a.InputSHA256 != digest(a.Input) || a.PromptSHA256 != m.Binding.PromptSHA256BySlot[key] || digest(a.GenerationConfig) != m.Binding.GenerationConfigSHA256BySlot[key] {
				return fmt.Errorf("%w: recovery provenance mismatch for %s", ErrInvalidCampaignRecovery, key)
			}
			finished[key] = true
		} else if !RecoverableNoResponse(a) {
			finished[key] = true
		}
		seen[key] = number
	}
	if len(bundle.Attempts) > m.Reservation.AdditionalCalls {
		return fmt.Errorf("%w: recovery attempts exceed conservative reservation", ErrRecoveryBudgetExceeded)
	}
	if err := validateRecoveryReservation(m); err != nil {
		return err
	}
	return nil
}

func validateRecoveryReservation(m RecoveryManifest) error {
	worstCaseCalls := len(m.Targets) * (m.Policy.MaxAttemptsPerSlot - 1)
	if m.Reservation.AdditionalCalls != worstCaseCalls {
		return fmt.Errorf("%w: reservation target count mismatch", ErrInvalidCampaignRecovery)
	}
	reservation, err := m.Budget.Reserve(worstCaseCalls)
	if err != nil {
		return err
	}
	if math.Abs(reservation.TotalUpperBoundUSD-m.Reservation.TotalUpperBoundUSD) > 1e-9 || math.Abs(reservation.RemainingUpperUSD-m.Reservation.RemainingUpperUSD) > 1e-9 {
		return fmt.Errorf("%w: reservation changed", ErrInvalidCampaignRecovery)
	}
	return nil
}

// EffectiveRecoveryAttempts overlays the first successful recovery response on
// an eligible original failure. It returns one terminal attempt per base slot;
// all source attempts remain available in the original and recovery bundles.
func EffectiveRecoveryAttempts(baseAttempts []Attempt, recovery []RecoveryAttempt) ([]Attempt, RecoveryCompleteness, error) {
	terminals, err := terminalAttempts(baseAttempts)
	if err != nil {
		return nil, RecoveryCompleteness{}, err
	}
	byKey := map[string][]RecoveryAttempt{}
	for _, item := range recovery {
		key := attemptKey(item.Attempt.CaseID, item.Attempt.Trial)
		byKey[key] = append(byKey[key], item)
	}
	keys := make([]string, 0, len(terminals))
	for key := range terminals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]Attempt, 0, len(keys))
	missing := make([]string, 0)
	for _, key := range keys {
		original := terminals[key]
		effective := original
		if RecoverableNoResponse(original) {
			for _, item := range byKey[key] {
				if item.Attempt.Status == "executed" {
					effective = item.Attempt
					break
				}
				effective = item.Attempt
			}
		}
		result = append(result, effective)
		if effective.Status != "executed" {
			missing = append(missing, key)
		}
	}
	return result, RecoveryCompleteness{ExpectedSlots: len(keys), EffectiveSlots: len(keys) - len(missing), MissingSlots: missing, Complete: len(missing) == 0}, nil
}

// CampaignRecoveryEvidence is the durable JSON document for one immutable
// recovery overlay. It is intentionally separate from attempts.jsonl.
type CampaignRecoveryEvidence struct {
	Manifest RecoveryManifest  `json:"manifest"`
	Attempts []RecoveryAttempt `json:"attempts"`
}

// RecoveredAttempt retains both source and effective evidence for report
// overlays. Effective is the first successful response, or the terminal
// failure when no response was recovered.
type RecoveredAttempt struct {
	CaseID                string            `json:"case_id"`
	Trial                 int               `json:"trial"`
	Original              Attempt           `json:"original"`
	RecoveryAttempts      []RecoveryAttempt `json:"recovery_attempts"`
	Effective             Attempt           `json:"effective"`
	OriginalAttemptSHA256 string            `json:"original_attempt_sha256"`
}

// WriteCampaignRecovery creates a recovery document once. Existing documents
// are never overwritten, preserving append-only provenance.
func WriteCampaignRecovery(path string, evidence CampaignRecoveryEvidence) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: recovery path is required", ErrInvalidCampaignRecovery)
	}
	canonical, err := canonicalizeRecoveryEvidence(evidence)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return fmt.Errorf("encode recovery evidence: %w", err)
	}
	// MarshalIndent formats embedded RawMessage values. Re-read that exact
	// representation and refresh hashes so readers validate the bytes actually
	// persisted, without mutating the caller's evidence bundle.
	var persisted CampaignRecoveryEvidence
	if err := json.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("normalize recovery evidence: %w", err)
	}
	for i := range persisted.Attempts {
		if len(persisted.Attempts[i].Attempt.Input) > 0 {
			persisted.Attempts[i].Attempt.InputSHA256 = digest(persisted.Attempts[i].Attempt.Input)
		}
		if len(persisted.Attempts[i].Attempt.GenerationConfig) > 0 {
			// GenerationConfig is also a RawMessage and receives indentation.
			// Keep the persisted hash coupled to its final bytes.
			persisted.Attempts[i].Attempt.GenerationConfig = append(json.RawMessage(nil), persisted.Attempts[i].Attempt.GenerationConfig...)
			key := attemptKey(persisted.Attempts[i].Attempt.CaseID, persisted.Attempts[i].Attempt.Trial)
			configHash := digest(persisted.Attempts[i].Attempt.GenerationConfig)
			persisted.Manifest.Binding.GenerationConfigSHA256BySlot[key] = configHash
			// Keep the caller's binding aligned with the exact persisted bytes.
			evidence.Manifest.Binding.GenerationConfigSHA256BySlot[key] = configHash
		}
	}
	data, err = json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("encode normalized recovery evidence: %w", err)
	}
	var stable CampaignRecoveryEvidence
	if err := json.Unmarshal(data, &stable); err != nil {
		return fmt.Errorf("normalize recovery hashes: %w", err)
	}
	for i := range stable.Attempts {
		if len(stable.Attempts[i].Attempt.Input) > 0 {
			stable.Attempts[i].Attempt.InputSHA256 = digest(stable.Attempts[i].Attempt.Input)
		}
	}
	data, err = json.MarshalIndent(stable, "", "  ")
	if err != nil {
		return fmt.Errorf("encode stable recovery evidence: %w", err)
	}
	if err := ensureRecoveryDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create recovery evidence: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("write recovery evidence: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync recovery evidence: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close recovery evidence: %w", err)
	}
	ok = true
	return nil
}

func ensureRecoveryDirectory(directory string) error {
	if directory == "." || directory == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create recovery directory: %w", err)
	}
	return nil
}

// ReadCampaignRecovery reads a bounded immutable evidence document. Binding to
// the original run is completed by ApplyCampaignRecovery, which has access to
// the checksummed source journal.
func ReadCampaignRecovery(path string) (CampaignRecoveryEvidence, error) {
	var evidence CampaignRecoveryEvidence
	if strings.TrimSpace(path) == "" {
		return evidence, fmt.Errorf("%w: recovery path is required", ErrInvalidCampaignRecovery)
	}
	file, err := os.Open(path)
	if err != nil {
		return evidence, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return evidence, err
	}
	const maxRecoveryEvidenceBytes = 64 * 1024 * 1024
	if info.Size() <= 0 || info.Size() > maxRecoveryEvidenceBytes {
		return evidence, fmt.Errorf("%w: recovery evidence size is invalid", ErrInvalidCampaignRecovery)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRecoveryEvidenceBytes+1))
	if err != nil {
		return evidence, fmt.Errorf("read recovery evidence: %w", err)
	}
	if len(data) > maxRecoveryEvidenceBytes || json.Unmarshal(data, &evidence) != nil {
		return evidence, fmt.Errorf("%w: malformed recovery evidence", ErrInvalidCampaignRecovery)
	}
	if evidence.Manifest.FormatVersion != RecoveryFormatVersion || strings.TrimSpace(evidence.Manifest.RecoveryID) == "" {
		return evidence, fmt.Errorf("%w: unsupported recovery evidence", ErrInvalidCampaignRecovery)
	}
	return evidence, nil
}

// ApplyCampaignRecovery verifies the original run and returns a report-ready
// slot map. It does not modify the run or recovery evidence on disk.
func ApplyCampaignRecovery(dataset *Dataset, originalRunDirectory string, evidence CampaignRecoveryEvidence) (map[string]RecoveredAttempt, error) {
	if dataset == nil {
		return nil, fmt.Errorf("%w: dataset is required", ErrInvalidCampaignRecovery)
	}
	record, baseAttempts, err := ReadRun(originalRunDirectory)
	if err != nil {
		return nil, fmt.Errorf("read immutable base run: %w", err)
	}
	if err := validateRun(dataset, record, baseAttempts); err != nil {
		return nil, fmt.Errorf("validate immutable base run: %w", err)
	}
	bundle := RecoveryBundle(evidence)
	if err := ValidateRecovery(bundle, record, baseAttempts); err != nil {
		return nil, err
	}
	effective, _, err := EffectiveRecoveryAttempts(baseAttempts, evidence.Attempts)
	if err != nil {
		return nil, err
	}
	terminals, err := terminalAttempts(baseAttempts)
	if err != nil {
		return nil, err
	}
	recoveryByKey := map[string][]RecoveryAttempt{}
	for _, attempt := range evidence.Attempts {
		key := attemptKey(attempt.Attempt.CaseID, attempt.Attempt.Trial)
		recoveryByKey[key] = append(recoveryByKey[key], attempt)
	}
	result := make(map[string]RecoveredAttempt, len(terminals))
	for _, attempt := range effective {
		key := attemptKey(attempt.CaseID, attempt.Trial)
		original, ok := terminals[key]
		if !ok {
			return nil, fmt.Errorf("%w: effective slot has no original %s", ErrInvalidCampaignRecovery, key)
		}
		result[key] = RecoveredAttempt{CaseID: attempt.CaseID, Trial: attempt.Trial, Original: original, RecoveryAttempts: append([]RecoveryAttempt(nil), recoveryByKey[key]...), Effective: attempt, OriginalAttemptSHA256: AttemptSHA256(original)}
	}
	return result, nil
}

// RecoverySemanticReviewDocument is a separate review namespace for recovered
// outputs. It cannot be applied to the original review document because its
// recovery identity and output hashes are different.
type RecoverySemanticReviewDocument struct {
	FormatVersion          string           `json:"format_version"`
	BaseRunID              string           `json:"base_run_id"`
	BaseAttemptsSHA256     string           `json:"base_attempts_sha256"`
	RecoveryID             string           `json:"recovery_id"`
	RecoveryEvidenceSHA256 string           `json:"recovery_evidence_sha256"`
	Reviews                []SemanticReview `json:"reviews"`
}

type RecoverySemanticReviewReport struct {
	Reviews               []SemanticReview `json:"reviews"`
	ResultCounts          map[string]int   `json:"result_counts"`
	PendingHumanChecks    int              `json:"pending_human_checks"`
	CriticalPendingChecks int              `json:"critical_pending_checks"`
	ReleaseApproved       bool             `json:"release_approved"`
	Warning               string           `json:"warning"`
}

// CampaignRecoveryEvidenceSHA256 hashes the canonical JSON representation of
// the recovery evidence, allowing a review to bind to one immutable overlay.
func CampaignRecoveryEvidenceSHA256(evidence CampaignRecoveryEvidence) (string, error) {
	canonical, err := canonicalizeRecoveryEvidence(evidence)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("hash recovery evidence: %w", err)
	}
	return digest(data), nil
}

// canonicalizeRecoveryEvidence makes RawMessage fields stable across a JSON
// round trip and recomputes the hashes that are defined over those bytes. Raw
// output remains untouched because its exact provider bytes are evidence.
func canonicalizeRecoveryEvidence(evidence CampaignRecoveryEvidence) (CampaignRecoveryEvidence, error) {
	result := evidence
	result.Manifest.Binding.PromptSHA256BySlot = cloneStringMap(evidence.Manifest.Binding.PromptSHA256BySlot)
	result.Manifest.Binding.GenerationConfigSHA256BySlot = cloneStringMap(evidence.Manifest.Binding.GenerationConfigSHA256BySlot)
	result.Attempts = append([]RecoveryAttempt(nil), evidence.Attempts...)
	for i := range result.Attempts {
		canonical, err := canonicalizeRecoveryAttempt(result.Attempts[i].Attempt)
		if err != nil {
			return CampaignRecoveryEvidence{}, err
		}
		result.Attempts[i].Attempt = canonical
		key := attemptKey(canonical.CaseID, canonical.Trial)
		if len(canonical.Input) > 0 {
			result.Manifest.Binding.PromptSHA256BySlot[key] = canonical.PromptSHA256
		}
		if len(canonical.GenerationConfig) > 0 {
			result.Manifest.Binding.GenerationConfigSHA256BySlot[key] = digest(canonical.GenerationConfig)
		}
	}
	return result, nil
}

func canonicalizeRecoveryAttempt(attempt Attempt) (Attempt, error) {
	result := attempt
	if len(result.Input) > 0 {
		var compact bytes.Buffer
		if err := json.Compact(&compact, result.Input); err != nil {
			return Attempt{}, fmt.Errorf("%w: invalid recovery input JSON: %v", ErrInvalidCampaignRecovery, err)
		}
		result.Input = json.RawMessage(compact.Bytes())
		result.InputSHA256 = digest(result.Input)
		prompt, err := promptHash(result.Input)
		if err != nil {
			return Attempt{}, fmt.Errorf("%w: invalid recovery prompt: %v", ErrInvalidCampaignRecovery, err)
		}
		result.PromptSHA256 = prompt
	}
	if len(result.GenerationConfig) > 0 {
		var compact bytes.Buffer
		if err := json.Compact(&compact, result.GenerationConfig); err != nil {
			return Attempt{}, fmt.Errorf("%w: invalid recovery generation config JSON: %v", ErrInvalidCampaignRecovery, err)
		}
		result.GenerationConfig = json.RawMessage(compact.Bytes())
	}
	return result, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

// NewRecoverySemanticReviewTemplate emits criteria only for first-effective
// recovered responses. Original successful outputs are deliberately excluded.
func NewRecoverySemanticReviewTemplate(dataset *Dataset, originalRunDirectory string, evidence CampaignRecoveryEvidence) (RecoverySemanticReviewDocument, error) {
	result := RecoverySemanticReviewDocument{FormatVersion: semanticReviewVersion, Reviews: []SemanticReview{}}
	record, err := readBaseRecoveryRecord(dataset, originalRunDirectory, evidence)
	if err != nil {
		return result, err
	}
	recovered, err := ApplyCampaignRecovery(dataset, originalRunDirectory, evidence)
	if err != nil {
		return result, err
	}
	evidenceHash, err := CampaignRecoveryEvidenceSHA256(evidence)
	if err != nil {
		return result, err
	}
	result.BaseRunID = record.RunID
	result.BaseAttemptsSHA256 = record.AttemptsSHA256
	result.RecoveryID = evidence.Manifest.RecoveryID
	result.RecoveryEvidenceSHA256 = evidenceHash
	keys := make([]string, 0, len(recovered))
	for key := range recovered {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := recovered[key]
		if item.Effective.Status != "executed" || item.Effective.Retry <= item.Original.Retry {
			continue
		}
		if strings.TrimSpace(item.Effective.RawOutput) == "" {
			return RecoverySemanticReviewDocument{}, fmt.Errorf("%w: recovered output is empty for %s", ErrInvalidCampaignRecovery, key)
		}
		evaluation, evalErr := evaluatePlannedAttempt(dataset, record.Plan, item.Effective)
		if evalErr != nil {
			return RecoverySemanticReviewDocument{}, fmt.Errorf("evaluate recovered output %s: %w", key, evalErr)
		}
		for _, check := range evaluation.SemanticChecks {
			result.Reviews = append(result.Reviews, SemanticReview{CaseID: item.Effective.CaseID, Trial: item.Effective.Trial, Retry: item.Effective.Retry, OutputSHA256: digest([]byte(item.Effective.RawOutput)), CriterionID: check.ID, CriterionSHA256: semanticCriterionHash(check), Criterion: check.Criterion, Severity: check.Severity, Result: "unassessed"})
		}
	}
	return result, nil
}

// ApplyRecoverySemanticReviews validates judgments against the exact recovered
// output and criterion hashes. Agent judgments remain pending human review.
func ApplyRecoverySemanticReviews(dataset *Dataset, originalRunDirectory string, evidence CampaignRecoveryEvidence, document RecoverySemanticReviewDocument) (RecoverySemanticReviewReport, error) {
	template, err := NewRecoverySemanticReviewTemplate(dataset, originalRunDirectory, evidence)
	if err != nil {
		return RecoverySemanticReviewReport{}, err
	}
	if !supportedSemanticReviewVersion(document.FormatVersion) || document.BaseRunID != template.BaseRunID || document.BaseAttemptsSHA256 != template.BaseAttemptsSHA256 || document.RecoveryID != template.RecoveryID || document.RecoveryEvidenceSHA256 != template.RecoveryEvidenceSHA256 {
		return RecoverySemanticReviewReport{}, fmt.Errorf("%w: recovery review identity mismatch", ErrInvalidSemanticReview)
	}
	expected := make(map[string]SemanticReview, len(template.Reviews))
	for _, review := range template.Reviews {
		key := semanticReviewKey(review)
		if _, exists := expected[key]; exists {
			return RecoverySemanticReviewReport{}, fmt.Errorf("%w: duplicate recovered criterion %s", ErrInvalidSemanticReview, key)
		}
		expected[key] = review
	}
	imported := make(map[string]SemanticReview, len(document.Reviews))
	recoveredOutputs := make(map[string]string, len(evidence.Attempts))
	recoveredParsedOutputs := make(map[string]json.RawMessage, len(evidence.Attempts))
	for _, item := range evidence.Attempts {
		key := semanticAttemptKey(item.Attempt.CaseID, item.Attempt.Trial, item.Attempt.Retry)
		recoveredOutputs[key] = item.Attempt.RawOutput
		recoveredParsedOutputs[key] = item.Attempt.ParsedOutput
	}
	for _, review := range document.Reviews {
		key := semanticReviewKey(review)
		original, exists := expected[key]
		if !exists {
			return RecoverySemanticReviewReport{}, fmt.Errorf("%w: unknown recovered criterion %s", ErrInvalidSemanticReview, key)
		}
		if _, duplicate := imported[key]; duplicate {
			return RecoverySemanticReviewReport{}, fmt.Errorf("%w: duplicate recovered criterion %s", ErrInvalidSemanticReview, key)
		}
		attemptKey := semanticAttemptKey(review.CaseID, review.Trial, review.Retry)
		if err := validateSemanticReview(document.FormatVersion, original, review, "executed", recoveredOutputs[attemptKey], recoveredParsedOutputs[attemptKey]); err != nil {
			return RecoverySemanticReviewReport{}, fmt.Errorf("%s: %w", key, err)
		}
		imported[key] = review
	}
	result := RecoverySemanticReviewReport{Reviews: append([]SemanticReview(nil), document.Reviews...), ResultCounts: map[string]int{"pass": 0, "fail": 0, "unassessed": 0, "not_applicable": 0}, Warning: "Recovered outputs use a separate review namespace. Reviewer provenance is self-declared; agent judgments do not certify safety and release approval remains false."}
	for _, review := range template.Reviews {
		judgment, exists := imported[semanticReviewKey(review)]
		value := "unassessed"
		if exists {
			value = judgment.Result
		}
		result.ResultCounts[value]++
		if !exists || judgment.Result == "unassessed" || judgment.ReviewerKind != "human" {
			result.PendingHumanChecks++
			if review.Severity == "critical" {
				result.CriticalPendingChecks++
			}
		}
	}
	result.ReleaseApproved = false
	return result, nil
}

func readBaseRecoveryRecord(dataset *Dataset, originalRunDirectory string, evidence CampaignRecoveryEvidence) (RunRecord, error) {
	if dataset == nil {
		return RunRecord{}, fmt.Errorf("%w: dataset is required", ErrInvalidCampaignRecovery)
	}
	record, attempts, err := ReadRun(originalRunDirectory)
	if err != nil {
		return RunRecord{}, fmt.Errorf("read immutable base run: %w", err)
	}
	if err := validateRun(dataset, record, attempts); err != nil {
		return RunRecord{}, fmt.Errorf("validate immutable base run: %w", err)
	}
	if evidence.Manifest.BaseRunID != record.RunID || evidence.Manifest.BaseAttemptsSHA256 != record.AttemptsSHA256 {
		return RunRecord{}, fmt.Errorf("%w: base recovery identity mismatch", ErrInvalidCampaignRecovery)
	}
	return record, nil
}
