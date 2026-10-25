package readmodel

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

type JobRequestSummary struct {
	ID             int
	ConversationID int
	Title          string
	Description    string
	Status         string
	Requester      JobRequestRequester
	Images         []JobRequestImage
	ServiceAddress *consumer.Address
	CreatedOn      time.Time
}

type JobRequestRequester struct {
	Name    string
	Surname string
}

type JobRequestImage struct {
	FileID       string
	OriginalName string
	URL          string
}
