package readmodel

import (
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type CompletionReport struct {
	ID          int
	Description string
	ReportedOn  time.Time
	Images      []filedomain.Image
}
