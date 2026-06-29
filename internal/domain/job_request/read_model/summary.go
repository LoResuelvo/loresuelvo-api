package readmodel

type JobRequestSummary struct {
	ID             int
	ConversationID int
	Title          string
	Description    string
	Status         string
	Requester      JobRequestRequester
	Images         []JobRequestImage
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
