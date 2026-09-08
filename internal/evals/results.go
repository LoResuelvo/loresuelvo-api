package evals

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const resultVersion = "1"

// Attempt preserves execution evidence independently from subsequent scoring.
type Attempt struct {
	RequestID        *string         `json:"provider_http_request_id"`
	CaseID           string          `json:"case_id"`
	Trial            int             `json:"trial"`
	Retry            int             `json:"retry"`
	Status           string          `json:"status"`
	StartedOn        time.Time       `json:"started_on"`
	LatencyMillis    int64           `json:"latency_ms"`
	Error            string          `json:"error,omitempty"`
	Input            json.RawMessage `json:"input,omitempty"`
	InputSHA256      string          `json:"input_sha256,omitempty"`
	PromptSHA256     string          `json:"prompt_sha256,omitempty"`
	GenerationConfig json.RawMessage `json:"generation_config,omitempty"`
	RawOutput        string          `json:"raw_output,omitempty"`
	ParsedOutput     json.RawMessage `json:"parsed_output,omitempty"`
	ProviderResponse json.RawMessage `json:"provider_response,omitempty"`
	RequestCount     int             `json:"request_count"`
}

type RunRecord struct {
	UnknownDefaults string          `json:"unknown_defaults"`
	FormatVersion   string          `json:"format_version"`
	RunID           string          `json:"run_id"`
	Mode            string          `json:"mode"`
	Commit          string          `json:"commit"`
	StartedOn       time.Time       `json:"started_on"`
	FinishedOn      *time.Time      `json:"finished_on"`
	Plan            *Plan           `json:"plan"`
	Limits          ExecutionLimits `json:"limits"`
	Status          string          `json:"status"`
	AttemptsSHA256  string          `json:"attempts_sha256"`
	ReleaseApproved bool            `json:"release_approved"`
}

type Journal struct {
	directory string
	file      *os.File
}

// NewJournal refuses to overwrite any previous run, including partial runs.
func NewJournal(directory string, record RunRecord) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		return nil, fmt.Errorf("create runs directory: %w", err)
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, fmt.Errorf("create unique run directory: %w", err)
	}
	if err := writeJSONAtomic(filepath.Join(directory, "run.json"), record); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "attempts.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("create attempt journal: %w", err)
	}
	return &Journal{directory: directory, file: file}, nil
}
func (j *Journal) Append(attempt Attempt) error {
	data, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("encode attempt: %w", err)
	}
	if _, err = j.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append attempt: %w", err)
	}
	if err = j.file.Sync(); err != nil {
		return fmt.Errorf("sync attempt: %w", err)
	}
	return nil
}
func (j *Journal) Finish(record *RunRecord) error {
	data, err := os.ReadFile(filepath.Join(j.directory, "attempts.jsonl"))
	if err != nil {
		return fmt.Errorf("hash attempts: %w", err)
	}
	record.AttemptsSHA256 = digest(data)
	return writeJSONAtomic(filepath.Join(j.directory, "run.json"), record)
}
func (j *Journal) Close() error { return j.file.Close() }

func writeJSONAtomic(path string, value any) (resultErr error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".eval-*")
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	name := temp.Name()
	defer func() {
		if cleanupErr := os.Remove(name); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}()
	if _, err = temp.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write report: %w", errors.Join(err, temp.Close()))
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync report: %w", errors.Join(err, temp.Close()))
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close report: %w", err)
	}
	if err = os.Rename(name, path); err != nil {
		return fmt.Errorf("replace report: %w", err)
	}
	return nil
}

// ReadRun verifies the journal before replay. An interrupted, unfinalized run is
// retained on disk but cannot be presented as a complete replayable experiment.
func ReadRun(directory string) (RunRecord, []Attempt, error) {
	var record RunRecord
	root, err := os.OpenRoot(directory)
	if err != nil {
		return record, nil, err
	}
	defer root.Close()
	data, err := root.ReadFile("run.json")
	if err != nil {
		return record, nil, err
	}
	if err = json.Unmarshal(data, &record); err != nil {
		return record, nil, err
	}
	if record.FormatVersion != resultVersion || record.FinishedOn == nil || record.Plan == nil || record.AttemptsSHA256 == "" {
		return record, nil, fmt.Errorf("incomplete or unsupported run")
	}
	data, err = root.ReadFile("attempts.jsonl")
	if err != nil {
		return record, nil, err
	}
	if digest(data) != record.AttemptsSHA256 {
		return record, nil, fmt.Errorf("attempt journal checksum mismatch")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 32*1024*1024)
	var attempts []Attempt
	for scanner.Scan() {
		var a Attempt
		if err = json.Unmarshal(scanner.Bytes(), &a); err != nil {
			return record, nil, err
		}
		attempts = append(attempts, a)
	}
	if err = scanner.Err(); err != nil {
		return record, nil, err
	}
	return record, attempts, nil
}

// WriteReport stores only a derived report next to a completed run.
func WriteReport(directory string, report Report) error {
	return writeJSONAtomic(filepath.Join(directory, "report.json"), report)
}
