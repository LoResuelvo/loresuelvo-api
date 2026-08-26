package consumer

import (
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "consumer"

type Consumer struct {
	*user.BaseUser
	address      Address
	location     GeoPoint
	coverageZone coveragezone.CoverageZone
}

// NewConsumer creates a consumer from already constructed value objects.
func NewConsumer(
	auth0ID, email, name, surname string,
	profilePhoto *filedomain.Image,
	address *Address,
	location GeoPoint,
	coverageZone coveragezone.CoverageZone,
) (*Consumer, error) {
	if address == nil {
		return nil, ErrAddressRequired
	}
	if err := address.Validate(); err != nil {
		return nil, err
	}
	if err := location.Validate(); err != nil {
		return nil, err
	}
	if coverageZone.ID <= 0 {
		return nil, ErrCoverageZoneNotAvailable
	}
	if err := coverageZone.ValidateSelection(); err != nil {
		return nil, ErrCoverageZoneNotAvailable
	}

	baseUser, err := user.New(auth0ID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		BaseUser:     baseUser,
		address:      *address,
		location:     location,
		coverageZone: coverageZone,
	}, nil
}

// RehydrateConsumer restores a consumer from already validated persistence data.
func RehydrateConsumer(baseUser *user.BaseUser, address Address, location GeoPoint, coverageZone coveragezone.CoverageZone) *Consumer {
	return &Consumer{
		BaseUser:     baseUser,
		address:      address,
		location:     location,
		coverageZone: coverageZone,
	}
}

func (consumer *Consumer) Address() Address {
	return consumer.address
}

func (consumer *Consumer) Location() GeoPoint {
	return consumer.location
}

func (consumer *Consumer) CoverageZone() coveragezone.CoverageZone {
	return consumer.coverageZone
}
