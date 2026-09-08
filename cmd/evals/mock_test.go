package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// cliDataset builds a tiny synthetic fixture, not the experimental corpus.
func cliDataset(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"datasets/prediagnosis.jsonl":      `{"id":"PD-001","task":"prediagnosis","family_id":"synthetic","split":"development","input":{"user_message":"test","available_categories":[],"is_new_conversation":true}}` + "\n",
		"datasets/ranking.jsonl":           `{"id":"RK-001","task":"ranking","family_id":"synthetic_ranking","split":"holdout","input":{"problem_title":"test","problem_description":"test","max_results":3,"candidates":[]}}` + "\n",
		"datasets/service_contracts.jsonl": `{"id":"CT-01","target":"test","input":{},"expected":{}}` + "\n",
		"configs/suites.json":              `{"smoke":["PD-001"],"development":["PD-001"],"holdout":["RK-001"],"critical_all":["PD-001"],"rules":{}}`,
		"configs/metamorphic.json":         `{ "ranking": [], "prediagnosis_pairs": [] }`,
		"configs/experiment.json":          `{"trials":{"smoke":1,"baseline_development":3,"release":3}}`,
		"assets/manifest.json":             `{"assets":[]}`,
	}
	for _, name := range []string{"prediagnosis", "ranking", "service_contracts", "prediagnosis-output", "ranking-output"} {
		files["schemas/"+name+".schema.json"] = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`
	}
	hashes := make(map[string]string)
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		sum := sha256.Sum256([]byte(content))
		hashes[name] = hex.EncodeToString(sum[:])
	}
	manifest, err := json.Marshal(map[string]any{"version": "synthetic", "hash_algorithm": "SHA-256", "files_sha256": hashes})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), manifest, 0600))
	return root
}
