package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

var campaignExporterFiles = []string{
	"internal/evals/campaign_export.go",
	"cmd/evals/campaign_export.go",
	"cmd/evals/main.go",
}

func writeCampaignExport(directory string, bundle evals.CampaignExport) (resultErr error) {
	if strings.TrimSpace(directory) == "" {
		return fmt.Errorf("campaign export --out is required")
	}
	artifacts, err := renderCampaignExportArtifacts(bundle)
	if err != nil {
		return err
	}
	if err = os.Mkdir(directory, 0700); err != nil {
		return fmt.Errorf("create new campaign export directory: %w", err)
	}
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		file, openErr := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if openErr != nil {
			return errors.Join(resultErr, fmt.Errorf("create campaign export artifact %s: %w", name, openErr))
		}
		if _, writeErr := file.Write(artifacts[name]); writeErr != nil {
			return errors.Join(resultErr, writeErr, file.Close())
		}
		if syncErr := file.Sync(); syncErr != nil {
			return errors.Join(resultErr, syncErr, file.Close())
		}
		if closeErr := file.Close(); closeErr != nil {
			return errors.Join(resultErr, closeErr)
		}
	}
	return nil
}

func renderCampaignExportArtifacts(bundle evals.CampaignExport) (map[string][]byte, error) {
	attempts, err := encodeJSONLines(bundle.Attempts)
	if err != nil {
		return nil, fmt.Errorf("encode campaign attempts: %w", err)
	}
	responses, err := encodeJSONLines(bundle.Responses)
	if err != nil {
		return nil, fmt.Errorf("encode campaign responses: %w", err)
	}
	results, err := encodeJSONLines(bundle.Results)
	if err != nil {
		return nil, fmt.Errorf("encode campaign results: %w", err)
	}
	reviews, err := encodeJSONLines(bundle.Reviews)
	if err != nil {
		return nil, fmt.Errorf("encode campaign reviews: %w", err)
	}
	summary, err := json.MarshalIndent(bundle.Summary, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode campaign summary: %w", err)
	}
	summary = append(summary, '\n')
	readme := renderCampaignExportREADME(bundle.Manifest)
	artifacts := map[string][]byte{
		"README.md":       readme,
		"attempts.jsonl":  attempts,
		"responses.jsonl": responses,
		"results.jsonl":   results,
		"reviews.jsonl":   reviews,
		"summary.json":    summary,
	}
	records := map[string]*int{
		"README.md":       nil,
		"attempts.jsonl":  intPointer(len(bundle.Attempts)),
		"responses.jsonl": intPointer(len(bundle.Responses)),
		"results.jsonl":   intPointer(len(bundle.Results)),
		"reviews.jsonl":   intPointer(len(bundle.Reviews)),
		"summary.json":    nil,
	}
	bundle.Manifest.Artifacts = make(map[string]evals.CampaignExportArtifact, len(artifacts))
	for name, data := range artifacts {
		if err = evals.ValidateCampaignExportArtifact(data); err != nil {
			return nil, fmt.Errorf("validate %s: %w", name, err)
		}
		bundle.Manifest.Artifacts[name] = evals.CampaignExportArtifact{SHA256: campaignExportDigest(data), Records: records[name]}
	}
	manifest, err := json.MarshalIndent(bundle.Manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode campaign manifest: %w", err)
	}
	manifest = append(manifest, '\n')
	if err = evals.ValidateCampaignExportArtifact(manifest); err != nil {
		return nil, fmt.Errorf("validate manifest.json: %w", err)
	}
	artifacts["manifest.json"] = manifest
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	slices.Sort(names)
	var checksums bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&checksums, "%s  %s\n", campaignExportDigest(artifacts[name]), name)
	}
	artifacts["SHA256SUMS"] = checksums.Bytes()
	return artifacts, nil
}

func encodeJSONLines[T any](rows []T) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return nil, err
		}
	}
	return output.Bytes(), nil
}

func renderCampaignExportREADME(manifest evals.CampaignExportManifest) []byte {
	const template = "# Campaña %s — evidencia longitudinal\n\n" +
		"Este directorio conserva los datos detallados necesarios para comparar la solución entre campañas. No contiene gráficas: las visualizaciones y métricas futuras deben derivarse de estos archivos.\n\n" +
		"## Archivos\n\n" +
		"- **responses.jsonl:** una respuesta efectiva por slot, con texto original y salida interpretada cuando fue posible. Las respuestas malformadas se conservan sin corregir.\n" +
		"- **attempts.jsonl:** cada solicitud real al proveedor, incluidos los errores transitorios y la recuperación append-only; no contiene prompts, imágenes, headers ni identificadores HTTP.\n" +
		"- **results.jsonl:** resultado por caso, modelo y trial, con esperado, observado, métricas y códigos de fallo.\n" +
		"- **reviews.jsonl:** juicios por criterio, su evidencia y procedencia; effective_for_result distingue los juicios usados en el resultado de los reemplazados por recuperación.\n" +
		"- **summary.json:** agregados derivados y limitaciones de la campaña.\n" +
		"- **manifest.json:** identidad del golden, protocolo, commits, hashes de fuentes, cobertura y datos ausentes.\n" +
		"- **SHA256SUMS:** integridad de todos los artefactos anteriores.\n\n" +
		"## Cobertura\n\n" +
		"- Slots planificados: **%d**.\n" +
		"- Solicitudes reales al proveedor: **%d**.\n" +
		"- Respuestas físicas conservadas: **%d**.\n" +
		"- Respuestas malformadas conservadas: **%d**.\n" +
		"- Revisiones efectivas: **%d**; evaluadas por agente: **%d**; evaluadas por humanos: **%d**; no evaluadas: **%d**.\n\n" +
		"Los trials repetidos corresponden a los mismos casos y no deben tratarse como observaciones poblacionales independientes. La identidad de revisores es declarada, no autenticada. Esta campaña no certifica seguridad ni aprueba un release.\n"
	return []byte(fmt.Sprintf(template, manifest.CampaignID, manifest.Counts.PlannedSlots, manifest.Counts.ProviderAttempts, manifest.Counts.Responses, manifest.Counts.MalformedResponses, manifest.Counts.EffectiveReviews, manifest.Counts.AgentAssessments, manifest.Counts.HumanAssessments, manifest.Counts.UnassessedReviews))
}

func campaignExporterCommit() (string, error) {
	args := append([]string{"status", "--porcelain", "--"}, campaignExporterFiles...)
	status, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("inspect campaign exporter source: %w", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return "", fmt.Errorf("campaign exporter files must be committed before export")
	}
	output, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolve campaign exporter commit: %w", err)
	}
	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return "", fmt.Errorf("campaign exporter commit is empty")
	}
	return commit, nil
}

func campaignExportDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func intPointer(value int) *int { return &value }
