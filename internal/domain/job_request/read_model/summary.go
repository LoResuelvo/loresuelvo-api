package readmodel

type JobRequestSummary struct {
	ID             int
	ConversationID int
	Title          string
	Description    string
	Requester      JobRequestRequester
}

type JobRequestRequester struct {
	Name    string
	Surname string
}
