package consumer_handler

type consumerSummaryResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}
