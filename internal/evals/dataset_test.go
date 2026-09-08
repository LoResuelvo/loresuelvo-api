package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeDatasetFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{

		"assets/manifest.json": `{"assets":[]}
`,
		"datasets/prediagnosis.jsonl": `{"id":"PD-001","task":"prediagnosis","family_id":"faucet","split":"development","input":{"available_categories":["Plumbing"]},"expected":{}}
`,
		"datasets/ranking.jsonl": `{"id":"RK-001","task":"ranking","family_id":"sink","split":"holdout","input":{"max_results":3},"expected":{}}
`,
		"datasets/service_contracts.jsonl": `{"id":"CT-01","input":{},"expected":{}}
`,
		"configs/suites.json": `{"smoke":["PD-001"],"development":["PD-001"],"holdout":["RK-001"],"critical_all":[],"rules":{"live_calls_disabled_by_default":true}}
`,
		"configs/experiment.json": `{"trials":{"smoke":1,"baseline_development":3,"release":3}}
`,
	}
	for _, name := range []string{"prediagnosis", "ranking", "service_contracts", "prediagnosis-output", "ranking-output"} {
		files["schemas/"+name+".schema.json"] = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`
	}
	manifest := datasetManifest{Version: "1.0.0", HashAlgorithm: "SHA-256", Files: make(map[string]string)}
	for name, data := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(data), 0600))
		manifest.Files[name] = digest([]byte(data))
	}

	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), data, 0600))
	return root
}
func TestLoadDatasetReadsVerifiedFixtures(t *testing.T) {
	d, err := LoadDataset(writeDatasetFixture(t))
	require.NoError(t, err)
	require.Equal(t, "PD-001", d.PD[0].ID)
	require.Equal(t, 3, d.ExperimentTrials["baseline_development"])
	require.Equal(t, []string{"RK-001"}, d.Suites["holdout"])
	require.Len(t, d.ManifestSHA256, 64)
}
func TestLoadDatasetRejectsChangedFile(t *testing.T) {
	root := writeDatasetFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "datasets/prediagnosis.jsonl"), []byte("{}"), 0600))
	_, err := LoadDataset(root)
	require.ErrorIs(t, err, ErrInvalidDataset)
}
func TestLoadDatasetRejectsEscapingSymlink(t *testing.T) {
	root := writeDatasetFixture(t)
	external := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.WriteFile(external, []byte("{}"), 0600))
	path := filepath.Join(root, "configs/suites.json")
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Symlink(external, path))
	_, err := LoadDataset(root)
	require.Error(t, err)
}
func TestValidateReferencesRejectsFamilyCrossingSplits(t *testing.T) {
	d := &Dataset{PD: []PDCase{
		{CaseMetadata: CaseMetadata{ID: "PD-001", Task: "prediagnosis", FamilyID: "same", Split: "development"}},
		{CaseMetadata: CaseMetadata{ID: "PD-002", Task: "prediagnosis", FamilyID: "same", Split: "holdout"}},
	}}
	require.ErrorContains(t, d.validateReferences(), "crosses splits")
}
func TestValidateReferencesRejectsHoldoutInSmoke(t *testing.T) {
	d := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-001", Task: "prediagnosis", FamilyID: "same", Split: "holdout"}}}, Suites: map[string][]string{"smoke": {"PD-001"}}}
	require.ErrorContains(t, d.validateReferences(), "non-development")
}
func TestValidateReferencesRejectsUnknownAndDuplicateSuiteIDs(t *testing.T) {
	for _, ids := range [][]string{{"unknown"}, {"PD-001", "PD-001"}} {
		d := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-001", Task: "prediagnosis", FamilyID: "same", Split: "development"}}}, Suites: map[string][]string{"smoke": ids}}
		require.ErrorContains(t, d.validateReferences(), "unknown or duplicate")
	}
}

func TestLoadDatasetRejectsNonlocalManifestPath(t *testing.T) {
	root := writeDatasetFixture(t)
	path := filepath.Join(root, "manifest.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var manifest datasetManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	manifest.Files["../outside"] = "invalid"
	data, err = json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	_, err = LoadDataset(root)
	require.ErrorIs(t, err, ErrInvalidDataset)
	require.ErrorContains(t, err, "nonlocal")
}

func TestValidateReferencesRejectsMissingPartitionMember(t *testing.T) {
	d := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-001", Task: "prediagnosis", FamilyID: "faucet", Split: "development"}}}, Suites: map[string][]string{"development": {}}}
	require.ErrorContains(t, d.validateReferences(), "missing from development")
}
