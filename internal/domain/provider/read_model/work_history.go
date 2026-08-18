package readmodel

import "time"

type WorkOrder struct {
	ID               int
	ScheduledOn      time.Time
	Description      string
	Status           string
	CompletionReport *CompletionReport
	Review           *Review
}

type CompletionReport struct {
	Description string
	ReportedOn  time.Time
}

type Review struct {
	Rating      int
	Description string
}
