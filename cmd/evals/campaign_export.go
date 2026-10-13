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
	summary, err := json.MarshalIndent(bundle.Summary, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode campaign summary: %w", err)
	}
	summary = append(summary, '\n')
	readme := renderCampaignExportREADME(bundle.Manifest, bundle.Summary)
	artifacts := map[string][]byte{
		"README.md":       readme,
		"attempts.jsonl":  attempts,
		"responses.jsonl": responses,
		"results.jsonl":   results,
		"summary.json":    summary,
	}
	records := map[string]*int{
		"README.md":       nil,
		"attempts.jsonl":  intPointer(len(bundle.Attempts)),
		"responses.jsonl": intPointer(len(bundle.Responses)),
		"results.jsonl":   intPointer(len(bundle.Results)),
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

func renderCampaignExportREADME(manifest evals.CampaignExportManifest, summary evals.CampaignExportSummary) []byte {
	return []byte(fmt.Sprintf("# Campaña %s — evidencia longitudinal\n\n%s\n\nLos smoke reutilizan casos development: no son casos nuevos ni observaciones independientes. Los trials miden variabilidad, no diversidad.\n\n## Archivos\n\n- **responses.jsonl:** todas las respuestas físicas recibidas, incluidas fallidas y reemplazadas; `raw_output` conserva el texto original y `parsed_output` el formato wire aceptado. Buscá `dataset_case_id`, `requested_model` y `trial`.\n- **attempts.jsonl:** cada solicitud al proveedor, con uso reportado, duración del intento, errores e identificadores disponibles. No contiene prompts, imágenes ni headers.\n- **results.jsonl:** un resultado por slot, referencia a la respuesta efectiva o ausencia explícita, métricas y resultados por criterio semántico sin identidades ni razonamientos privados.\n- **summary.json:** agregados derivados; operaciones incluyen todas las solicitudes originales y de recuperación, calidad usa el resultado efectivo. Reserva presupuestaria no equivale a costo facturado.\n- **manifest.json:** identidad, configuración deduplicada, fuentes e integridad.\n- **SHA256SUMS:** integridad de los artefactos.\n\n## Cobertura\n\n- Slots planificados: **%d**.\n- Solicitudes al proveedor: **%d**.\n- Respuestas recibidas conservadas: **%d**.\n- Respuestas malformadas: **%d**.\n- Criterios efectivos: **%d**; sin evaluar: **%d**.\n\nLa cobertura semántica no implica aprobación de un release.\n", manifest.CampaignID, renderCampaignExportScope(summary), manifest.Counts.PlannedSlots, manifest.Counts.ProviderAttempts, manifest.Counts.Responses, manifest.Counts.MalformedResponses, manifest.Counts.EffectiveReviews, manifest.Counts.UnassessedReviews))
}

func renderCampaignExportScope(summary evals.CampaignExportSummary) string {
	lines := make([]string, 0, len(summary.Phases))
	for _, phase := range summary.Phases {
		models := len(phase.Models)
		cases := 0
		if phase.Trials > 0 && models > 0 {
			cases = phase.Coverage.ExpectedSlots / (phase.Trials * models)
		}
		unitLabel := "casos"
		if phase.Metamorphic {
			unitLabel = "entradas planificadas (incluidas variantes)"
		}
		label := phase.Name
		if phase.Name == "primary" {
			label = "Fase primaria"
		} else if phase.Name == "smoke" {
			lines = append(lines, fmt.Sprintf("- **Comprobaciones smoke:** %d %s × %d modelos = %d slots (%d ejecución por caso y modelo).", cases, unitLabel, models, phase.Coverage.ExpectedSlots, phase.Trials))
			continue
		}
		lines = append(lines, fmt.Sprintf("- **%s:** %d %s × %d trials × %d modelos = %d slots.", label, cases, unitLabel, phase.Trials, models, phase.Coverage.ExpectedSlots))
	}
	return strings.Join(lines, "\n")
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
