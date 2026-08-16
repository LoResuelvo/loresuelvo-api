package workorder

import "errors"

var (
	ErrDoesNotExist                       = errors.New("Work order does not exist")
	ErrWorkOrderNotEligibleForFullPayment = errors.New("Work order is not eligible to be marked as fully paid")
	ErrInvalidWorkOrderState              = errors.New("invalid work order state")
	ErrInvalidWorkOrderIdentity           = errors.New("invalid work order identity")
)
