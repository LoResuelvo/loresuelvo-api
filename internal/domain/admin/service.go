package admin

import (
	"context"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
)

type Service struct {
	consumerReader          ConsumerDirectoryReader
	providerReader          ProviderDirectoryReader
	profilePhotoURLResolver ProfilePhotoURLResolver
}

func NewService(
	consumerReader ConsumerDirectoryReader,
	providerReader ProviderDirectoryReader,
	profilePhotoURLResolver ProfilePhotoURLResolver,
) *Service {
	return &Service{
		consumerReader:          consumerReader,
		providerReader:          providerReader,
		profilePhotoURLResolver: profilePhotoURLResolver,
	}
}

func (service *Service) ListConsumers(ctx context.Context, query string) ([]readmodel.Consumer, error) {
	consumers, err := service.consumerReader.FindConsumers(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("finding consumers for administrative directory: %w", err)
	}
	if len(consumers) == 0 {
		return []readmodel.Consumer{}, nil
	}

	profilePhotoFileIDs := make([]string, 0, len(consumers))
	for _, consumer := range consumers {
		if consumer.ProfilePhotoFileID != "" {
			profilePhotoFileIDs = append(profilePhotoFileIDs, consumer.ProfilePhotoFileID)
		}
	}
	if len(profilePhotoFileIDs) == 0 {
		return consumers, nil
	}

	profilePhotoURLs, err := service.profilePhotoURLResolver.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving consumer profile photo URLs: %w", err)
	}
	for index := range consumers {
		consumers[index].ProfilePhotoURL = profilePhotoURLs[consumers[index].ProfilePhotoFileID]
	}

	return consumers, nil
}

func (service *Service) ListProviders(ctx context.Context, query string) ([]readmodel.Provider, error) {
	providers, err := service.providerReader.FindProviders(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("finding providers for administrative directory: %w", err)
	}
	if len(providers) == 0 {
		return []readmodel.Provider{}, nil
	}

	profilePhotoFileIDs := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider.ProfilePhotoFileID != "" {
			profilePhotoFileIDs = append(profilePhotoFileIDs, provider.ProfilePhotoFileID)
		}
	}
	if len(profilePhotoFileIDs) == 0 {
		return providers, nil
	}

	profilePhotoURLs, err := service.profilePhotoURLResolver.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving provider profile photo URLs: %w", err)
	}
	for index := range providers {
		providers[index].ProfilePhotoURL = profilePhotoURLs[providers[index].ProfilePhotoFileID]
	}

	return providers, nil
}
