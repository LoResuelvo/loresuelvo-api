package provider_handler

type registerProviderRequest struct {
	Email              string `json:"email"`
	Name               string `json:"name"`
	Surname            string `json:"surname"`
	CategoryID         int    `json:"category_id"`
	CoverageZoneIDs    []int  `json:"coverage_zone_ids"`
	ProfilePhotoFileID string `json:"profile_photo_file_id"`
}
