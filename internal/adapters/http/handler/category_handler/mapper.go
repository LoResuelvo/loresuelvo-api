package category_handler

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/category"

func categoryResponseFromDomain(category category.Category) categoryResponse {
	return categoryResponse{
		ID:             category.ID,
		Name:           category.Name,
		NormalizedName: category.NormalizedName,
	}
}

func categoryListItemResponsesFromDomain(categories []category.Category) []categoryListItemResponse {
	response := make([]categoryListItemResponse, 0, len(categories))
	for _, category := range categories {
		response = append(response, categoryListItemResponse{
			ID:   category.ID,
			Name: category.Name,
		})
	}

	return response
}
