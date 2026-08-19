package provider_handler

import "time"

type providerSummaryResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}

type providerSearchResponse struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Surname         string  `json:"surname"`
	CategoryName    string  `json:"category_name"`
	ProfilePhotoURL string  `json:"profile_photo_url"`
	RatingAverage   float64 `json:"rating_average"`
	RatingCount     int     `json:"rating_count"`
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
	ID            int                                `json:"id"`
	Name          string                             `json:"name"`
	Surname       string                             `json:"surname"`
	ProfilePhoto  providerProfilePhotoResponse       `json:"profile_photo"`
	Category      providerProfileCategoryResponse    `json:"category"`
	RatingAverage float64                            `json:"rating_average"`
	RatingCount   int                                `json:"rating_count"`
	WorkOrders    []providerProfileWorkOrderResponse `json:"work_orders"`
}

type providerProfileWorkOrderResponse struct {
	ID               int                                      `json:"id"`
	ScheduledOn      time.Time                                `json:"scheduled_on"`
	Description      string                                   `json:"description"`
	Status           string                                   `json:"status"`
	CompletionReport *providerProfileCompletionReportResponse `json:"completion_report"`
	Review           *providerProfileReviewResponse           `json:"review,omitempty"`
}

type providerProfileCompletionReportResponse struct {
	Description string    `json:"description"`
	ReportedOn  time.Time `json:"reported_on"`
}

type providerProfileReviewResponse struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
