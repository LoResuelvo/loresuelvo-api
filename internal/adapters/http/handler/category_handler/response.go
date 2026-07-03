package category_handler

type categoryResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
}

type categoryListItemResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
