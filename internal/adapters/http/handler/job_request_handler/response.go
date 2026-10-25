package job_request_handler

import "time"

type jobRequestResponse struct {
	ID             int                     `json:"id"`
	ConversationID int                     `json:"conversation_id"`
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	Status         string                  `json:"status"`
	Images         []messageImageResponse  `json:"images"`
	ServiceAddress *serviceAddressResponse `json:"service_address"`
	CreatedOn      time.Time               `json:"created_on"`
}

type jobRequestSummaryResponse struct {
	ID             int                         `json:"id"`
	ConversationID int                         `json:"conversation_id"`
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	Status         string                      `json:"status"`
	Requester      jobRequestRequesterResponse `json:"requester"`
	Images         []messageImageResponse      `json:"images"`
	ServiceAddress *serviceAddressResponse     `json:"service_address"`
	CreatedOn      time.Time                   `json:"created_on"`
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

type serviceAddressResponse struct {
	Street       string `json:"street"`
	StreetNumber string `json:"street_number"`
	Floor        string `json:"floor,omitempty"`
	Unit         string `json:"unit,omitempty"`
}
