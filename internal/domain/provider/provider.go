package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "provider"

type Provider struct {
	*user.BaseUser
	Category      *category.Category
	CoverageZones []coveragezone.CoverageZone
}

func NewProvider(
	auth0ID string,
	email string,
	name string,
	surname string,
	providerCategory *category.Category,
	profilePhoto *filedomain.Image,
	coverageZones []coveragezone.CoverageZone,
) (*Provider, error) {
	if providerCategory == nil {
		return nil, category.ErrDoesNotExist
	}
	if len(coverageZones) == 0 {
		return nil, coveragezone.ErrAtLeastOneRequired
	}

	coverageZoneIDs := make([]int, 0, len(coverageZones))
	for _, zone := range coverageZones {
		if err := zone.ValidateSelection(); err != nil {
			return nil, err
		}
		coverageZoneIDs = append(coverageZoneIDs, zone.ID)
	}
	if err := coveragezone.ValidateUniqueIDs(coverageZoneIDs); err != nil {
		return nil, err
	}

	providerUser, err := user.New(auth0ID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}

	return &Provider{
		BaseUser:      providerUser,
		Category:      providerCategory,
		CoverageZones: append([]coveragezone.CoverageZone{}, coverageZones...),
	}, nil
}

func (p Provider) Categoryname() string {
	return p.Category.Name
}

func (p Provider) HasCategory(categoryID int) bool {
	return p.Category.ID == categoryID
}
