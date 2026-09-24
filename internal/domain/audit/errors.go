package audit

import "errors"

var (
	ErrInvalidEvent = errors.New("invalid audit event")
	ErrPersistence  = errors.New("audit persistence failure")
	ErrNotFound     = errors.New("audit event not found")
	ErrInvalidQuery = errors.New("invalid audit query")
)
