// Package evals contains the local, offline evaluation harness.
package evals

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrInvalidDataset = errors.New("invalid evaluation dataset")

type CaseMetadata struct {
	ID       string `json:"id"`
	Task     string `json:"task"`
	FamilyID string `json:"family_id"`
	Split    string `json:"split"`
}
type PDCase struct {
	CaseMetadata
	Input    PDInput         `json:"input"`
	Expected json.RawMessage `json:"expected"`
}
type RKCase struct {
	CaseMetadata
	Input    RKInput         `json:"input"`
	Expected json.RawMessage `json:"expected"`
}
type CTCase struct {
	CaseMetadata
	Target   string          `json:"target"`
	Input    json.RawMessage `json:"input"`
	Expected json.RawMessage `json:"expected"`
}
type Dataset struct {
	Root             string
	Version          string
	ManifestSHA256   string
	PD               []PDCase
	RK               []RKCase
	CT               []CTCase
	Suites           map[string][]string
	ExperimentTrials map[string]int
	assets           map[string]ImageInput
}
type datasetManifest struct {
	Version       string            `json:"version"`
	HashAlgorithm string            `json:"hash_algorithm"`
	Files         map[string]string `json:"files_sha256"`
}

// LoadDataset checks manifest integrity and harness references. JSON Schema and
// editorial policy validation remain the responsibility of the frozen evalpack tool.
func LoadDataset(root string) (*Dataset, error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open dataset: %w", err)
	}
	defer directory.Close()
	raw, err := directory.ReadFile("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest datasetManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Version == "" || manifest.HashAlgorithm != "SHA-256" || len(manifest.Files) == 0 {
		return nil, fmt.Errorf("%w: missing manifest metadata", ErrInvalidDataset)
	}
	// Decode the same bytes we verified, never a second filesystem read.
	verifiedFiles := make(map[string][]byte, len(manifest.Files))
	for name, hash := range manifest.Files {
		if !filepath.IsLocal(name) {
			return nil, fmt.Errorf("%w: nonlocal manifest path %q", ErrInvalidDataset, name)
		}
		data, readErr := directory.ReadFile(name)
		if readErr != nil {
			return nil, fmt.Errorf("read manifest file %q: %w", name, readErr)
		}
		if digest(data) != hash {
			return nil, fmt.Errorf("%w: checksum mismatch for %q", ErrInvalidDataset, name)
		}
		verifiedFiles[name] = data
	}
	for _, required := range []string{"datasets/prediagnosis.jsonl", "datasets/ranking.jsonl", "datasets/service_contracts.jsonl", "configs/suites.json", "configs/experiment.json", "assets/manifest.json"} {
		if _, ok := manifest.Files[required]; !ok {
			return nil, fmt.Errorf("%w: untracked required file %q", ErrInvalidDataset, required)
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve dataset root: %w", err)
	}
	d := &Dataset{Root: absolute, Version: manifest.Version, ManifestSHA256: digest(raw)}
	if d.PD, err = readCases[PDCase](verifiedFiles["datasets/prediagnosis.jsonl"], "datasets/prediagnosis.jsonl"); err != nil {
		return nil, err
	}
	if d.RK, err = readCases[RKCase](verifiedFiles["datasets/ranking.jsonl"], "datasets/ranking.jsonl"); err != nil {
		return nil, err
	}
	if d.CT, err = readCases[CTCase](verifiedFiles["datasets/service_contracts.jsonl"], "datasets/service_contracts.jsonl"); err != nil {
		return nil, err
	}
	var suites map[string]json.RawMessage
	if err = readJSON(verifiedFiles["configs/suites.json"], "configs/suites.json", &suites); err != nil {
		return nil, err
	}
	d.Suites = make(map[string][]string)
	for _, name := range []string{"smoke", "development", "holdout", "critical_all"} {
		raw, ok := suites[name]
		if !ok {
			return nil, fmt.Errorf("%w: missing suite %s", ErrInvalidDataset, name)
		}
		var ids []string
		if err = json.Unmarshal(raw, &ids); err != nil {
			return nil, fmt.Errorf("decode suite %s: %w", name, err)
		}
		d.Suites[name] = ids
	}
	var experiment struct {
		Trials struct {
			Smoke               int `json:"smoke"`
			BaselineDevelopment int `json:"baseline_development"`
			Release             int `json:"release"`
		} `json:"trials"`
	}
	if err = readJSON(verifiedFiles["configs/experiment.json"], "configs/experiment.json", &experiment); err != nil {
		return nil, err
	}
	d.ExperimentTrials = map[string]int{"smoke": experiment.Trials.Smoke, "baseline_development": experiment.Trials.BaselineDevelopment, "release": experiment.Trials.Release}
	for name, count := range d.ExperimentTrials {
		if count <= 0 {
			return nil, fmt.Errorf("%w: nonpositive trial count %s", ErrInvalidDataset, name)
		}
	}
	var assets struct {
		Assets []ImageInput `json:"assets"`
	}
	if err = readJSON(verifiedFiles["assets/manifest.json"], "assets/manifest.json", &assets); err != nil {
		return nil, err
	}
	d.assets = make(map[string]ImageInput, len(assets.Assets))
	for _, asset := range assets.Assets {
		if asset.AssetID == "" || asset.FileID == "" || asset.MimeType != "image/png" || manifest.Files[asset.Path] != asset.SHA256 || asset.SHA256 == "" {
			return nil, fmt.Errorf("%w: invalid asset %q", ErrInvalidDataset, asset.AssetID)
		}
		if _, ok := d.assets[asset.AssetID]; ok {
			return nil, fmt.Errorf("%w: duplicate asset %q", ErrInvalidDataset, asset.AssetID)
		}
		d.assets[asset.AssetID] = asset
	}
	if err = d.validateReferences(); err != nil {
		return nil, err
	}
	for _, c := range d.PD {
		if _, _, err = d.MapPD(c.Input); err != nil {
			return nil, fmt.Errorf("map case %s: %w", c.ID, err)
		}
	}
	for _, c := range d.RK {
		if _, err = c.Input.DomainRequest(); err != nil {
			return nil, fmt.Errorf("map case %s: %w", c.ID, err)
		}
	}
	return d, nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func readJSON(data []byte, name string, destination any) error {
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}
func readCases[T any](data []byte, name string) ([]T, error) {
	var result []T
	for index, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, fmt.Errorf("decode %s line %d: %w", name, index+1, err)
		}
		result = append(result, item)
	}
	return result, nil
}
func (d *Dataset) validateReferences() error {
	cases := make(map[string]CaseMetadata)
	families := make(map[string]string)
	add := func(meta CaseMetadata, task string) error {
		if meta.ID == "" {
			return fmt.Errorf("%w: empty case ID", ErrInvalidDataset)
		}
		if _, ok := cases[meta.ID]; ok {
			return fmt.Errorf("%w: duplicate case %s", ErrInvalidDataset, meta.ID)
		}
		if task != "" {
			if meta.Task != task || meta.FamilyID == "" || (meta.Split != "development" && meta.Split != "holdout") {
				return fmt.Errorf("%w: invalid metadata for %s", ErrInvalidDataset, meta.ID)
			}
			key := meta.FamilyID
			if split, ok := families[key]; ok && split != meta.Split {
				return fmt.Errorf("%w: family %s crosses splits", ErrInvalidDataset, meta.FamilyID)
			}
			families[key] = meta.Split
		}
		cases[meta.ID] = meta
		return nil
	}
	for _, c := range d.PD {
		if err := add(c.CaseMetadata, "prediagnosis"); err != nil {
			return err
		}
		for _, img := range c.Input.Images {
			if err := d.validateImage(img); err != nil {
				return fmt.Errorf("case %s: %w", c.ID, err)
			}
		}
	}
	for _, c := range d.RK {
		if err := add(c.CaseMetadata, "ranking"); err != nil {
			return err
		}
	}
	for _, c := range d.CT {
		if err := add(c.CaseMetadata, ""); err != nil {
			return err
		}
	}
	if len(d.Suites) == 0 {
		return fmt.Errorf("%w: no suites", ErrInvalidDataset)
	}
	for suite, ids := range d.Suites {
		seen := make(map[string]bool)
		for _, id := range ids {
			c, ok := cases[id]
			if !ok || seen[id] {
				return fmt.Errorf("%w: unknown or duplicate case %s in suite %s", ErrInvalidDataset, id, suite)
			}
			if suite == "critical_all" && c.Task != "prediagnosis" {
				return fmt.Errorf("%w: critical suite includes non-prediagnosis case %s", ErrInvalidDataset, id)
			}
			if (suite == "development" || suite == "holdout") && c.Split != suite {
				return fmt.Errorf("%w: case %s has wrong split for %s", ErrInvalidDataset, id, suite)
			}
			if suite == "smoke" && c.Split != "development" {
				return fmt.Errorf("%w: smoke includes non-development case %s", ErrInvalidDataset, id)
			}
			seen[id] = true
		}
	}
	for _, split := range []string{"development", "holdout"} {
		members := make(map[string]bool)
		for _, id := range d.Suites[split] {
			members[id] = true
		}
		for _, c := range cases {
			if c.Split == split && !members[c.ID] {
				return fmt.Errorf("%w: case %s missing from %s suite", ErrInvalidDataset, c.ID, split)
			}
		}
	}
	return nil
}
func (d *Dataset) validateImage(img ImageInput) error {
	asset, ok := d.assets[img.AssetID]
	if !ok || asset != img {
		return fmt.Errorf("%w: image %q does not match asset manifest", ErrInvalidDataset, img.AssetID)
	}
	return nil
}
