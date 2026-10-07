package installation

import "errors"

var (
	ErrInvalidInstallation = errors.New("invalid installation")
	ErrForbidden           = errors.New("installation access denied")
	ErrConflict            = errors.New("installation registration conflict")
	ErrNotFound            = errors.New("installation not found")
)
