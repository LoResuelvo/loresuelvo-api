package workorder

import "errors"

var (
	ErrDoesNotExist                         = errors.New("Work order does not exist")
	ErrWorkOrderNotEligibleForFullPayment   = errors.New("Work order is not eligible to be marked as fully paid")
	ErrInvalidWorkOrderState                = errors.New("invalid work order state")
	ErrInvalidWorkOrderIdentity             = errors.New("invalid work order identity")
	ErrCompletionReportRequired             = errors.New("completion report is required")
	ErrCompletionReportDescriptionRequired  = errors.New("completion report description is required")
	ErrCompletionReportImageCount           = errors.New("completion report must contain between one and three images")
	ErrCompletionReportImageRequired        = errors.New("completion report image file id is required")
	ErrCompletionReportDuplicateImage       = errors.New("completion report cannot contain duplicate images")
	ErrCompletionReportImageNotAvailable    = errors.New("completion report image is not available")
	ErrCompletionReportReportedOnRequired   = errors.New("completion report reported_on is required")
	ErrCompletionReportAlreadyExists        = errors.New("work order already has a completion report")
	ErrOnlyAssignedProviderCanReport        = errors.New("only the assigned provider can report work completion")
	ErrWorkOrderNotReadyForCompletion       = errors.New("work order is not ready for completion")
	ErrWorkOrderNotAwaitingPayment          = errors.New("work order is not awaiting payment")
	ErrWorkOrderAlreadyPaid                 = errors.New("work order is already paid")
	ErrOnlyWorkOrderConsumerCanCheckout     = errors.New("only the work order consumer can start checkout")
	ErrWorkOrderNotScheduledYet             = errors.New("work order is not scheduled yet")
	ErrPaidOnRequired                       = errors.New("paid_on is required")
	ErrWorkOrderCompletionImageNotAvailable = errors.New("work order completion image is not available")
	ErrWorkOrderUnitOfWorkRequired          = errors.New("work order unit of work is required")
	ErrOnlyWorkOrderParticipantCanView      = errors.New("only a work order participant can view its detail")
)
