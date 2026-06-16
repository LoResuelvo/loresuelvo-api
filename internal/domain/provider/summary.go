package provider

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"

func SummariesWithProfilePhotoURLs(providers []Provider, profilePhotoURLs map[string]string) []readmodel.ProviderSummary {
	providerSummaries := make([]readmodel.ProviderSummary, 0, len(providers))
	for _, provider := range providers {
		providerSummaries = append(providerSummaries, readmodel.ProviderSummary{
			ID:              provider.ID,
			Name:            provider.Name(),
			Surname:         provider.Surname(),
			CategoryName:    provider.Category.Name,
			ProfilePhotoURL: profilePhotoURLs[provider.ProfilePhotoFileID],
		})
	}

	return providerSummaries
}
