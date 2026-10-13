package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

// verifyBuildRevision checks observed build facts, not a trusted signature.
// Offline modes deliberately do not require VCS metadata or a clean checkout.
func verifyBuildRevision(info *debug.BuildInfo, sourceCommit string) error {
	if info == nil || sourceCommit == "" {
		return fmt.Errorf("live execution requires VCS build metadata; build with go build ./cmd/evals")
	}
	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" || modified != "false" || revision != sourceCommit {
		return fmt.Errorf("live executable VCS revision must match the clean execution checkout; rebuild with go build ./cmd/evals")
	}
	return nil
}

func executionBuildProvenance(sourceCommit string) (*evals.BuildProvenance, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	if err := verifyBuildRevision(info, sourceCommit); err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate live executable: %w", err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		return nil, fmt.Errorf("hash live executable: %w", err)
	}
	result := &evals.BuildProvenance{SourceCommit: sourceCommit, GoVersion: info.GoVersion, ExecutableSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), SourceFilesSHA256: map[string]string{}}
	for _, path := range []string{"go.mod", "go.sum", "internal/adapters/chatbot/gemini_chatbot.go", "internal/adapters/chatbot/gemini_generation.go", "internal/adapters/chatbot/gemini_json.go", "internal/adapters/chatbot/gemini_schema.go", "internal/adapters/chatbot/gemini_token_count.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("hash execution source %s: %w", path, err)
		}
		result.SourceFilesSHA256[path] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return result, nil
}
