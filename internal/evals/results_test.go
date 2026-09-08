package evals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJournalRefusesOverwriteAndRetainsInterruptedEvidence(t *testing.T) {
	_, record, attempt := replayEvidence(t)
	record.FinishedOn = nil
	path := filepath.Join(t.TempDir(), "run")
	journal, err := NewJournal(path, record)
	require.NoError(t, err)
	require.NoError(t, journal.Append(attempt))
	require.NoError(t, journal.Close())
	_, err = NewJournal(path, record)
	require.Error(t, err)
	_, _, err = ReadRun(path)
	require.ErrorContains(t, err, "incomplete")
	content, err := os.ReadFile(filepath.Join(path, "attempts.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(content), "RK-test")
	info, err := os.Stat(filepath.Join(path, "attempts.jsonl"))
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0077)
}

func TestReadRunRejectsJournalTampering(t *testing.T) {
	_, record, attempt := replayEvidence(t)
	path := persistReplayEvidence(t, record, attempt)
	require.NoError(t, os.WriteFile(filepath.Join(path, "attempts.jsonl"), []byte("{}\n"), 0600))
	_, _, err := ReadRun(path)
	require.ErrorContains(t, err, "checksum mismatch")
}
