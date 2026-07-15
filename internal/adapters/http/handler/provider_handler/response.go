package provider_handler

type providerSummaryResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}

type providerProfilePhotoResponse struct {
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

type providerProfileCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type providerProfileResponse struct {
	ID           int                             `json:"id"`
	Name         string                          `json:"name"`
	Surname      string                          `json:"surname"`
	ProfilePhoto providerProfilePhotoResponse    `json:"profile_photo"`
	Category     providerProfileCategoryResponse `json:"category"`
}
