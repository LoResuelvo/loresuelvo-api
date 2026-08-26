package consumer

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "consumer"

type Consumer struct {
	*user.BaseUser
	address        Address
	location       GeoPoint
	coverageZoneID int
}

// NewConsumer creates a consumer from already constructed value objects.
func NewConsumer(
	auth0ID, email, name, surname string,
	profilePhoto *filedomain.Image,
	address *Address,
	location GeoPoint,
	coverageZoneID int,
) (*Consumer, error) {
	if address == nil {
		return nil, ErrAddressRequired
	}
	if coverageZoneID <= 0 {
		return nil, ErrCoverageZoneNotAvailable
	}

	baseUser, err := user.New(auth0ID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		BaseUser:       baseUser,
		address:        *address,
		location:       location,
		coverageZoneID: coverageZoneID,
	}, nil
}

// RehydrateConsumer restores a consumer from already validated persistence data.
func RehydrateConsumer(baseUser *user.BaseUser, address Address, location GeoPoint, coverageZoneID int) *Consumer {
	return &Consumer{
		BaseUser:       baseUser,
		address:        address,
		location:       location,
		coverageZoneID: coverageZoneID,
	}
}

func (consumer *Consumer) Address() Address {
	return consumer.address
}

func (consumer *Consumer) Location() GeoPoint {
	return consumer.location
}

func (consumer *Consumer) CoverageZoneID() int {
	return consumer.coverageZoneID
}
