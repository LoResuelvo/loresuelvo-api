package provider_handler

import (
	"strings"

	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

func normalizeRegisterProviderRequest(req registerProviderRequest) registerProviderRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.ProfilePhotoFileID = strings.TrimSpace(req.ProfilePhotoFileID)

	return req
}

func providerSummaryResponseFromDomain(provider providerreadmodel.ProviderSummary) providerSummaryResponse {
	return providerSummaryResponse{
		ID:              provider.ID,
		Name:            provider.Name,
		Surname:         provider.Surname,
		CategoryName:    provider.CategoryName,
		ProfilePhotoURL: provider.ProfilePhotoURL,
	}
}

func providerSummaryResponsesFromDomain(providers []providerreadmodel.ProviderSummary) []providerSummaryResponse {
	response := make([]providerSummaryResponse, 0, len(providers))
	for _, provider := range providers {
		response = append(response, providerSummaryResponseFromDomain(provider))
	}

	return response
}
