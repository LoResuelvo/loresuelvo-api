package job_request_handler

type jobRequestResponse struct {
	ID             int                    `json:"id"`
	ConversationID int                    `json:"conversation_id"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	Images         []messageImageResponse `json:"images"`
}

type jobRequestSummaryResponse struct {
	ID             int                         `json:"id"`
	ConversationID int                         `json:"conversation_id"`
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	Status         string                      `json:"status"`
	Requester      jobRequestRequesterResponse `json:"requester"`
	Images         []messageImageResponse      `json:"images"`
}

type jobRequestRequesterResponse struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type messageImageResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
}
