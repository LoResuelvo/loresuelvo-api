package admin_handler

import (
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
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

func providerDirectoryResponsesFromReadModel(providers []readmodel.Provider) []providerDirectoryResponse {
	responses := make([]providerDirectoryResponse, 0, len(providers))
	for _, found := range providers {
		coverageZones := make([]providerDirectoryCoverageZone, 0, len(found.CoverageZones))
		for _, zone := range found.CoverageZones {
			coverageZones = append(coverageZones, providerDirectoryCoverageZone{
				ID:           zone.ID,
				MarketID:     zone.MarketID,
				Code:         zone.Code,
				Name:         zone.Name,
				Kind:         string(zone.Kind),
				ParentZoneID: zone.ParentZoneID,
				Enabled:      zone.Enabled,
			})
		}

		responses = append(responses, providerDirectoryResponse{
			ID:              found.ID,
			Role:            provider.Role,
			Name:            found.Name,
			Surname:         found.Surname,
			Email:           found.Email,
			ProfilePhotoURL: optionalString(found.ProfilePhotoURL),
			CreatedOn:       found.CreatedOn.UTC(),
			Category: providerDirectoryCategory{
				ID:   found.Category.ID,
				Name: found.Category.Name,
			},
			CoverageZones:              coverageZones,
			IdentityVerificationStatus: string(found.IdentityVerificationStatus),
			IdentityVerifiedOn:         optionalTimeUTC(found.IdentityVerifiedOn),
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

func optionalTimeUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	timestamp := value.UTC()
	return &timestamp
}
