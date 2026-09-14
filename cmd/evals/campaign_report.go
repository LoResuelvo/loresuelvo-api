package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

func writeCampaignArtifacts(directory string, report evals.CampaignReport) (resultErr error) {
	if directory == "" {
		return fmt.Errorf("campaign report --out is required")
	}
	artifacts, err := renderCampaignArtifacts(report)
	if err != nil {
		return err
	}
	if err = os.Mkdir(directory, 0700); err != nil {
		return fmt.Errorf("create new campaign report directory: %w", err)
	}
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		data := artifacts[name]
		path := filepath.Join(directory, name)
		file, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if openErr != nil {
			return errors.Join(resultErr, fmt.Errorf("create campaign artifact %s: %w", name, openErr))
		}
		if _, writeErr := file.Write(data); writeErr != nil {
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

func renderCampaignArtifacts(report evals.CampaignReport) (map[string][]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode campaign report: %w", err)
	}
	coverage := []evals.CoverageChartSeries{}
	quality := []evals.QualityChartSeries{}
	operations := []evals.OperationsChartSeries{}
	ranking := []evals.RankingChartSeries{}
	variability := []evals.TrialVariabilitySeries{}
	for _, phase := range report.Phases {
		for _, model := range phase.Models {
			name := phase.Name + " / " + model.Provenance.RequestedModel
			coverage = append(coverage, evals.CoverageChartSeries{
				Name: name, ExpectedSlots: model.Coverage.ExpectedSlots,
				TerminalSlots: model.Coverage.TerminalSlots, ExecutedSlots: model.Coverage.ExecutedSlots,
				FailedSlots:     model.Coverage.ExecutionFailedSlots,
				SemanticUnknown: model.Coverage.SemanticUnknownSlots,
				MissingSlots:    model.Coverage.ExpectedSlots - model.Coverage.TerminalSlots,
			})
			operations = append(operations, evals.OperationsChartSeries{Name: name, Operations: model.Operations})
			if phase.Name != "primary" {
				continue
			}
			for _, item := range []struct {
				task   string
				metric string
			}{{"prediagnosis", "accepted_outcome_accuracy"}, {"prediagnosis", "category_accuracy_when_required"}, {"ranking", "ndcg_at_3"}, {"ranking", "precision_at_3_relevance_ge_2"}} {
				task, ok := model.Tasks[item.task]
				if !ok {
					continue
				}
				metric, ok := task.Metrics[item.metric]
				if !ok {
					continue
				}
				quality = append(quality, evals.QualityChartSeries{Name: model.Provenance.RequestedModel, Metric: item.metric, Summary: metric})
				if variation, exists := task.TrialVariability[item.metric]; exists {
					variability = append(variability, evals.TrialVariabilitySeries{Name: model.Provenance.RequestedModel, Metric: item.metric, ExpectedBaseCases: variation.ExpectedBaseCases, EvaluatedBaseCases: variation.EvaluatedBaseCases, ComparableBaseCases: variation.ComparableBaseCases, VariableBaseCases: variation.VariableBaseCases, MeanWithinCaseRange: variation.MeanWithinCaseRange, MaxWithinCaseRange: variation.MaxWithinCaseRange})
				}
			}
			if task, ok := model.Tasks["ranking"]; ok {
				for _, metric := range []string{"ndcg_at_3", "precision_at_3_relevance_ge_2"} {
					if summary, exists := task.Metrics[metric]; exists {
						ranking = append(ranking, evals.RankingChartSeries{Name: model.Provenance.RequestedModel, Metric: metric, Summary: summary})
					}
				}
			}
		}
	}
	for _, policy := range report.Offline.RankingPolicies {
		for _, metric := range []string{"ndcg_at_3", "precision_at_3_relevance_ge_2"} {
			if summary, ok := policy.Metrics[metric]; ok {
				ranking = append(ranking, evals.RankingChartSeries{Name: policy.Policy.Name, Metric: metric, Summary: summary, Baseline: true})
			}
		}
	}
	coverageSVG, err := evals.RenderCoverageSVG(coverage)
	if err != nil {
		return nil, err
	}
	qualitySVG, err := evals.RenderQualitySVG(quality)
	if err != nil {
		return nil, err
	}
	operationsSVG, err := evals.RenderOperationsSVG(operations)
	if err != nil {
		return nil, err
	}
	rankingSVG, err := evals.RenderRankingSVG(ranking)
	if err != nil {
		return nil, err
	}
	variabilitySVG, err := evals.RenderVariabilitySVG(variability)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"report.json":     append(data, '\n'),
		"coverage.svg":    coverageSVG,
		"quality.svg":     qualitySVG,
		"operations.svg":  operationsSVG,
		"ranking.svg":     rankingSVG,
		"variability.svg": variabilitySVG,
	}, nil
}
