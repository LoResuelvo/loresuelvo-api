package readmodel

import "time"

type Consumer struct {
	ID                 int
	Name               string
	Surname            string
	Email              string
	ProfilePhotoFileID string
	ProfilePhotoURL    string
	CreatedOn          time.Time
}
