package operation

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"

// limitations explains derived values the persisted state cannot establish.
func limitations(summary readmodel.OperationSummary) []readmodel.Limitation {
	found := []readmodel.Limitation{}
	if summary.Stage == readmodel.StageRequestAccepted && summary.LastBusinessAdvanceOn == nil {
		found = append(found, readmodel.LimitationRequestAcceptanceTimeUnavailable)
	}
	return found
}
