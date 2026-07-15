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
	ID           int                              `json:"id"`
	Name         string                           `json:"name"`
	Surname      string                           `json:"surname"`
	Email        string                           `json:"email"`
	Role         string                           `json:"role"`
	ProfilePhoto *currentUserProfilePhotoResponse `json:"profile_photo,omitempty"`
}

type consumerCurrentUserResponse struct {
	currentUserResponse
}

type providerCurrentUserResponse struct {
	currentUserResponse
	Category currentUserCategoryResponse `json:"category"`
}
