package user_handler

type currentUserProfilePhotoResponse struct {
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

type currentUserCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type currentUserResponse struct {
	ID                       int                              `json:"id"`
	Name                     string                           `json:"name"`
	Surname                  string                           `json:"surname"`
	Email                    string                           `json:"email"`
	Role                     string                           `json:"role"`
	CalendarConnectionStatus string                           `json:"calendar_connection_status"`
	ProfilePhoto             *currentUserProfilePhotoResponse `json:"profile_photo,omitempty"`
}

type consumerAddressResponse struct {
	Street         string  `json:"street"`
	StreetNumber   string  `json:"street_number"`
	Floor          string  `json:"floor,omitempty"`
	Unit           string  `json:"unit,omitempty"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	CoverageZoneID int     `json:"coverage_zone_id"`
}

type consumerCurrentUserResponse struct {
	currentUserResponse
	Address consumerAddressResponse `json:"address"`
}

type providerCurrentUserResponse struct {
	currentUserResponse
	Category currentUserCategoryResponse `json:"category"`
}
