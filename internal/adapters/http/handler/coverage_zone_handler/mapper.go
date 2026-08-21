package coverage_zone_handler

import coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"

const googlePlaceBoundaryType = "google_place"

func coverageZoneListItemResponsesFromDomain(entries []coveragezone.CatalogEntry) []coverageZoneListItemResponse {
	response := make([]coverageZoneListItemResponse, 0, len(entries))
	for _, entry := range entries {
		response = append(response, coverageZoneListItemResponse{
			ID:   entry.Zone.ID,
			Name: entry.Zone.Name,
			Boundary: coverageZoneBoundaryResponse{
				Type:    googlePlaceBoundaryType,
				PlaceID: entry.BoundaryReference.ExternalID,
			},
		})
	}

	return response
}
