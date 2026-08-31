package readmodel

import filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"

type ProviderSearchResult struct {
	ID               int
	Name             string
	Surname          string
	CategoryName     string
	ProfilePhoto     *filedomain.Image
	RatingAverage    float64
	RatingCount      int
	IdentityVerified bool
}
