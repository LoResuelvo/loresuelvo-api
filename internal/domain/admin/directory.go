package admin

import (
	"context"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
)

type ConsumerDirectoryReader interface {
	FindConsumers(ctx context.Context) ([]readmodel.Consumer, error)
}

type ProfilePhotoURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}

type DirectoryService struct {
	consumerReader          ConsumerDirectoryReader
	profilePhotoURLResolver ProfilePhotoURLResolver
}

func NewDirectoryService(
	consumerReader ConsumerDirectoryReader,
	profilePhotoURLResolver ProfilePhotoURLResolver,
) *DirectoryService {
	return &DirectoryService{
		consumerReader:          consumerReader,
		profilePhotoURLResolver: profilePhotoURLResolver,
	}
}

func (service *DirectoryService) ListConsumers(ctx context.Context) ([]readmodel.Consumer, error) {
	consumers, err := service.consumerReader.FindConsumers(ctx)
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
