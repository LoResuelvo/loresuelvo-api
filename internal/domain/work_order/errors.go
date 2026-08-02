package workorder

import "errors"

var (
	ErrDoesNotExist                        = errors.New("Work order does not exist")
	ErrInvalidCompletionAuthorization      = errors.New("Completion authorization is invalid")
	ErrWorkOrderNotEligibleForFullPayment  = errors.New("Work order is not eligible to be marked as fully paid")
	ErrOnlyConsumerCanViewConfirmationCode = errors.New("Only the work order consumer can view the confirmation code")
	ErrConfirmationCodeNotAvailable        = errors.New("Confirmation code is not available")
)
