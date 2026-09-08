package main

import (
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

func summarizeRun(dataset *evals.Dataset, directory string) (evals.Summary, error) {
	record, report, err := evals.Replay(dataset, directory)
	if err != nil {
		return evals.Summary{}, err
	}
	observed, attempts, err := evals.ReadRun(directory)
	if err != nil {
		return evals.Summary{}, err
	}
	if observed.AttemptsSHA256 != record.AttemptsSHA256 {
		return evals.Summary{}, fmt.Errorf("run changed during summary")
	}
	return evals.Summarize(dataset, record.Plan, attempts, report.Attempts)
}
