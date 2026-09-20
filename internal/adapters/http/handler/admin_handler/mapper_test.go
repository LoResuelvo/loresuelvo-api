package admin_handler

import (
	"testing"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerDirectoryResponsesMapPublicContract(t *testing.T) {
	createdOn := time.Date(2026, 9, 19, 12, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	responses := consumerDirectoryResponsesFromReadModel([]readmodel.Consumer{
		{
			ID:              10,
			Name:            "Ana",
			Surname:         "Pérez",
			Email:           "ana@example.com",
			ProfilePhotoURL: "https://cdn.example/profile.jpg",
			CreatedOn:       createdOn,
		},
		{
			ID:        11,
			Name:      "Beatriz",
			Surname:   "Suárez",
			Email:     "beatriz@example.com",
			CreatedOn: createdOn,
		},
	})

	require.Len(t, responses, 2)
	assert.Equal(t, consumerDirectoryResponse{
		ID:              10,
		Role:            consumer.Role,
		Name:            "Ana",
		Surname:         "Pérez",
		Email:           "ana@example.com",
		ProfilePhotoURL: stringPointer("https://cdn.example/profile.jpg"),
		CreatedOn:       createdOn.UTC(),
	}, responses[0])
	assert.Nil(t, responses[1].ProfilePhotoURL)
}

func TestConsumerDirectoryResponsesReturnNonNilEmptySlice(t *testing.T) {
	responses := consumerDirectoryResponsesFromReadModel(nil)

	assert.NotNil(t, responses)
	assert.Empty(t, responses)
}

func stringPointer(value string) *string {
	return &value
}
