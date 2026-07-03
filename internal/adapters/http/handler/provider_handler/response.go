package provider_handler

type providerSummaryResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}
