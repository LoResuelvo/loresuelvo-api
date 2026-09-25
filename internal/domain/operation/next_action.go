package operation

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"

func nextActionOwner(summary readmodel.OperationSummary) *readmodel.Owner {
	var owner readmodel.Owner
	switch summary.Stage {
	case readmodel.StageRequestPending, readmodel.StageRequestAccepted, readmodel.StageWorkOrderScheduled:
		owner = readmodel.OwnerProvider
	case readmodel.StageProposalPending:
		if summary.HasAlert(readmodel.AlertBookingDeadlinePassed) {
			return nil
		}
		owner = readmodel.OwnerConsumer
	case readmodel.StageWorkOrderAwaitingPayment:
		owner = readmodel.OwnerConsumer
	case readmodel.StageWorkOrderPaid:
		owner = readmodel.OwnerNone
	default:
		return nil
	}
	return &owner
}
