package consumer

import (
	"strings"
	"unicode/utf8"
)

// Address contains only the address data supplied by the consumer. Derived
// geographic data is resolved by the application service and is not accepted
// from the client.
const (
	maxStreetLength       = 200
	maxStreetNumberLength = 50
	maxFloorLength        = 20
	maxUnitLength         = 50
)

type Address struct {
	Street       string
	StreetNumber string
	Floor        string
	Unit         string
}

func NewAddress(street, streetNumber, floor, unit string) (*Address, error) {
	address := Address{
		Street:       strings.TrimSpace(street),
		StreetNumber: strings.TrimSpace(streetNumber),
		Floor:        strings.TrimSpace(floor),
		Unit:         strings.TrimSpace(unit),
	}

	if err := address.Validate(); err != nil {
		return nil, err
	}

	return &address, nil
}

func (address Address) Validate() error {
	if strings.TrimSpace(address.Street) == "" && strings.TrimSpace(address.StreetNumber) == "" {
		return ErrAddressRequired
	}
	if strings.TrimSpace(address.Street) == "" {
		return ErrStreetRequired
	}
	if strings.TrimSpace(address.StreetNumber) == "" {
		return ErrStreetNumberRequired
	}
	if utf8.RuneCountInString(address.Street) > maxStreetLength ||
		utf8.RuneCountInString(address.StreetNumber) > maxStreetNumberLength ||
		utf8.RuneCountInString(address.Floor) > maxFloorLength ||
		utf8.RuneCountInString(address.Unit) > maxUnitLength {
		return ErrAddressFieldTooLong
	}

	return nil
}

func (address Address) Query() string {
	return strings.TrimSpace(address.Street + " " + address.StreetNumber)
}
