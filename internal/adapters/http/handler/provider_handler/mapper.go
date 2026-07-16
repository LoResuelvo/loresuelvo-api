package provider_handler

import (
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

func normalizeRegisterProviderRequest(req registerProviderRequest) registerProviderRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.ProfilePhotoFileID = strings.TrimSpace(req.ProfilePhotoFileID)

	return req
}

func providerSummaryResponseFromDomain(provider provider.Provider) providerSummaryResponse {
	profilePhotoURL := ""
	if provider.ProfilePhoto() != nil {
		profilePhotoURL = provider.ProfilePhoto().URL
	}
	return providerSummaryResponse{
		ID:              provider.ID(),
		Name:            provider.Name(),
		Surname:         provider.Surname(),
		CategoryName:    provider.Categoryname(),
		ProfilePhotoURL: profilePhotoURL,
	}
}

func providerSummaryResponsesFromDomain(providers []provider.Provider) []providerSummaryResponse {
	response := make([]providerSummaryResponse, 0, len(providers))
	for _, provider := range providers {
		response = append(response, providerSummaryResponseFromDomain(provider))
	}

	return response
}

func providerProfileResponseFromDomain(foundProvider provider.Provider) providerProfileResponse {
	return providerProfileResponse{
		ID:      foundProvider.ID(),
		Name:    foundProvider.Name(),
		Surname: foundProvider.Surname(),
		ProfilePhoto: providerProfilePhotoResponse{
			OriginalName: foundProvider.ProfilePhoto().OriginalName,
			URL:          foundProvider.ProfilePhoto().URL,
		},
		Category: providerProfileCategoryResponse{
			ID:   foundProvider.Category.ID,
			Name: foundProvider.Category.Name,
		},
	}
}
