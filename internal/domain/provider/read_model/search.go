package readmodel

import (
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type ProviderSearchResult struct {
	ID               int
	Name             string
	Surname          string
	CategoryName     string
	ProfilePhoto     *filedomain.Image
	CoverageZones    []coveragezone.CoverageZone
	RatingAverage    float64
	RatingCount      int
	IdentityVerified bool
}
