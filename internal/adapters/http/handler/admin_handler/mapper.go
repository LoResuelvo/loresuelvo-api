package admin_handler

import (
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

func consumerDirectoryResponsesFromReadModel(consumers []readmodel.Consumer) []consumerDirectoryResponse {
	responses := make([]consumerDirectoryResponse, 0, len(consumers))
	for _, found := range consumers {
		responses = append(responses, consumerDirectoryResponse{
			ID:              found.ID,
			Role:            consumer.Role,
			Name:            found.Name,
			Surname:         found.Surname,
			Email:           found.Email,
			ProfilePhotoURL: optionalString(found.ProfilePhotoURL),
			CreatedOn:       found.CreatedOn.UTC(),
		})
	}
	return responses
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
