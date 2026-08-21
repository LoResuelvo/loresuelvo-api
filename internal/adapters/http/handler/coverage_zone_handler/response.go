package coverage_zone_handler

type coverageZoneListItemResponse struct {
	ID       int                          `json:"id"`
	Name     string                       `json:"name"`
	Boundary coverageZoneBoundaryResponse `json:"boundary"`
}

type coverageZoneBoundaryResponse struct {
	Type    string `json:"type"`
	PlaceID string `json:"place_id"`
}
