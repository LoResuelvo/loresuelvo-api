package operation

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"

func limitations(summary readmodel.OperationSummary) []readmodel.Limitation {
	found := []readmodel.Limitation{}
	if summary.Stage == readmodel.StageRequestAccepted && summary.LastBusinessAdvanceOn == nil {
		found = append(found, readmodel.LimitationRequestAcceptanceTimeUnavailable)
	}
	return found
}
