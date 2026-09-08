package evals

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
)

const (
	CandidatePermutation     = "candidate_permutation"
	OpaqueReferenceBijection = "opaque_reference_bijection"
	WorkHistoryOrder         = "work_history_order"
)

type MetamorphicConfig struct {
	Ranking           []RankingTransformationPlan `json:"ranking"`
	PrediagnosisPairs []PDPair                    `json:"prediagnosis_pairs"`
}
type RankingTransformationPlan struct {
	BaseCaseID      string   `json:"base_case_id"`
	FamilyID        string   `json:"family_id"`
	Split           string   `json:"split"`
	Transformations []string `json:"transformations"`
	Seeds           []int64  `json:"permutation_seeds"`
	RepeatTrials    int      `json:"repeat_trials"`
	Invariants      []string `json:"invariants"`
	NotRequired     []string `json:"not_required"`
	AggregationUnit string   `json:"aggregation_unit"`
}
type PDPair struct {
	Left     string `json:"left"`
	Right    string `json:"right"`
	Relation string `json:"relation"`
	FamilyID string `json:"family_id"`
}
type MetamorphicVariant struct {
	ID              string `json:"id"`
	BaseCaseID      string `json:"base_case_id"`
	FamilyID        string `json:"family_id"`
	Split           string `json:"split"`
	Transformation  string `json:"transformation"`
	Seed            int64  `json:"seed"`
	RepeatTrials    int    `json:"repeat_trials"`
	AggregationUnit string `json:"aggregation_unit"`
}

func VariantID(baseID, transformation string, seed int64) string {
	return baseID + "~" + transformation + "~" + strconv.FormatInt(seed, 10)
}

// ValidateMetamorphicConfig checks identity and split inheritance against the
// loaded dataset. Call it on the same verified configuration bytes as the loader.
func ValidateMetamorphicConfig(dataset *Dataset) error {
	if dataset == nil {
		return fmt.Errorf("metamorphic dataset is required")
	}
	ranking := make(map[string]CaseMetadata, len(dataset.RK))
	for _, c := range dataset.RK {
		ranking[c.ID] = c.CaseMetadata
	}
	seen := map[string]bool{}
	for _, p := range dataset.Metamorphic.Ranking {
		base, ok := ranking[p.BaseCaseID]
		if !ok || seen[p.BaseCaseID] || p.FamilyID != base.FamilyID || p.Split != base.Split {
			return fmt.Errorf("invalid metamorphic parent metadata for %q", p.BaseCaseID)
		}
		seen[p.BaseCaseID] = true
		if p.RepeatTrials <= 0 || len(p.Seeds) == 0 || len(p.Transformations) == 0 || p.AggregationUnit != "base_case_not_variant" {
			return fmt.Errorf("invalid metamorphic execution settings for %s", p.BaseCaseID)
		}
		seeds := map[int64]bool{}
		for _, seed := range p.Seeds {
			if seeds[seed] {
				return fmt.Errorf("duplicate metamorphic seed for %s", p.BaseCaseID)
			}
			seeds[seed] = true
		}
		kinds := map[string]bool{}
		for _, kind := range p.Transformations {
			if !supportedTransformation(kind) || kinds[kind] {
				return fmt.Errorf("unknown or duplicate transformation %q", kind)
			}
			kinds[kind] = true
		}
	}
	pd := map[string]CaseMetadata{}
	for _, c := range dataset.PD {
		pd[c.ID] = c.CaseMetadata
	}
	pairs := map[string]bool{}
	for _, p := range dataset.Metamorphic.PrediagnosisPairs {
		left, lok := pd[p.Left]
		right, rok := pd[p.Right]
		key := p.Left + "~" + p.Right
		if !lok || !rok || p.Left == p.Right || pairs[key] || left.Split != right.Split || p.FamilyID != left.FamilyID || p.FamilyID != right.FamilyID {
			return fmt.Errorf("invalid metamorphic PD pair %s", key)
		}
		if p.Relation != "same_decision_despite_spelling" && p.Relation != "same_safety_despite_injection" {
			return fmt.Errorf("unsupported PD relation %q", p.Relation)
		}
		pairs[key] = true
	}
	return nil
}

// BuildMetamorphicVariants includes only explicitly selected base cases. It does
// not authorize holdout, add PD counterparts, or expand seeds into independent cases.
func BuildMetamorphicVariants(dataset *Dataset, baseCases []PlannedCase) ([]MetamorphicVariant, error) {
	if err := ValidateMetamorphicConfig(dataset); err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, c := range baseCases {
		selected[c.CaseID] = true
	}
	var variants []MetamorphicVariant
	for _, p := range dataset.Metamorphic.Ranking {
		if !selected[p.BaseCaseID] {
			continue
		}
		for _, kind := range p.Transformations {
			for _, seed := range p.Seeds {
				variants = append(variants, MetamorphicVariant{ID: VariantID(p.BaseCaseID, kind, seed), BaseCaseID: p.BaseCaseID, FamilyID: p.FamilyID, Split: p.Split, Transformation: kind, Seed: seed, RepeatTrials: p.RepeatTrials, AggregationUnit: p.AggregationUnit})
			}
		}
	}
	return variants, nil
}

type TransformedRanking struct {
	Input             RKInput           `json:"input"`
	InverseReferences map[string]string `json:"inverse_references,omitempty"`
	Changed           bool              `json:"changed"`
}

// TransformRanking changes one factor at a time and never reads expectations.
// Ordering uses SHA-256 keys instead of an implementation-dependent PRNG. Ties
// retain their original index. The input and nested evidence are copied first.
func TransformRanking(input RKInput, kind string, seed int64) (TransformedRanking, error) {
	var result TransformedRanking
	if !supportedTransformation(kind) {
		return result, fmt.Errorf("unsupported transformation %q", kind)
	}
	original, err := json.Marshal(input)
	if err != nil {
		return result, fmt.Errorf("encode ranking input: %w", err)
	}
	if err = json.Unmarshal(original, &result.Input); err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for _, c := range result.Input.Candidates {
		if c.Reference == "" || seen[c.Reference] {
			return result, fmt.Errorf("duplicate or empty input reference")
		}
		seen[c.Reference] = true
	}
	switch kind {
	case CandidatePermutation:
		if len(result.Input.Candidates) < 2 {
			break
		}
		order := metamorphicOrder(len(result.Input.Candidates), kind, seed, "")
		reordered := make([]CandidateInput, len(order))
		for i, index := range order {
			reordered[i] = result.Input.Candidates[index]
		}
		result.Input.Candidates = reordered
	case OpaqueReferenceBijection:
		result.InverseReferences = make(map[string]string, len(result.Input.Candidates))
		for i := range result.Input.Candidates {
			old := result.Input.Candidates[i].Reference
			key := metamorphicKey(kind, seed, old, i)
			replacement := "candidate-" + hex.EncodeToString(key[:16])
			if seen[replacement] || result.InverseReferences[replacement] != "" {
				return result, fmt.Errorf("generated reference collision")
			}
			result.InverseReferences[replacement] = old
			result.Input.Candidates[i].Reference = replacement
		}
	case WorkHistoryOrder:
		for i := range result.Input.Candidates {
			c := &result.Input.Candidates[i]
			if len(c.Evidence.WorkHistory) < 2 {
				continue
			}
			order := metamorphicOrder(len(c.Evidence.WorkHistory), kind, seed, c.Reference)
			reordered := make([]WorkInput, len(order))
			for j, index := range order {
				reordered[j] = c.Evidence.WorkHistory[index]
			}
			c.Evidence.WorkHistory = reordered
		}
	}
	transformed, err := json.Marshal(result.Input)
	if err != nil {
		return result, err
	}
	result.Changed = !bytes.Equal(original, transformed)
	return result, nil
}

func supportedTransformation(kind string) bool {
	return kind == CandidatePermutation || kind == OpaqueReferenceBijection || kind == WorkHistoryOrder
}
func metamorphicKey(kind string, seed int64, scope string, index int) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("us60-transform-v1\x00%s\x00%d\x00%s\x00%d", kind, seed, scope, index)))
}
func metamorphicOrder(count int, kind string, seed int64, scope string) []int {
	order := make([]int, count)
	keys := make([][32]byte, count)
	for i := range order {
		order[i] = i
		keys[i] = metamorphicKey(kind, seed, scope, i)
	}
	slices.SortStableFunc(order, func(a, b int) int { return bytes.Compare(keys[a][:], keys[b][:]) })
	return order
}

// RestoreRankingOutput inverts only recommendation references. All other fields
// survive so normalization cannot hide schema violations or rewrite reason text.
func RestoreRankingOutput(raw json.RawMessage, inverse map[string]string) (json.RawMessage, error) {
	if len(inverse) == 0 {
		return slices.Clone(raw), nil
	}
	values := map[string]bool{}
	for from, to := range inverse {
		if from == "" || to == "" || values[to] {
			return nil, fmt.Errorf("invalid inverse reference bijection")
		}
		values[to] = true
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	var recommendations []map[string]json.RawMessage
	if err := json.Unmarshal(output["recommendations"], &recommendations); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, recommendation := range recommendations {
		var ref string
		if err := json.Unmarshal(recommendation["reference"], &ref); err != nil {
			return nil, err
		}
		original, ok := inverse[ref]
		if !ok || seen[ref] {
			return nil, fmt.Errorf("unknown or duplicate transformed reference %q", ref)
		}
		seen[ref] = true
		encoded, err := json.Marshal(original)
		if err != nil {
			return nil, err
		}
		recommendation["reference"] = encoded
	}
	encoded, err := json.Marshal(recommendations)
	if err != nil {
		return nil, err
	}
	output["recommendations"] = encoded
	return json.Marshal(output)
}
