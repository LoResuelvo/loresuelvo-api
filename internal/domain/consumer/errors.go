package consumer

import "errors"

var (
	ErrAddressRequired                   = errors.New("Address is required")
	ErrStreetRequired                    = errors.New("Address street is required")
	ErrStreetNumberRequired              = errors.New("Address street number is required")
	ErrAddressFieldTooLong               = errors.New("Address field is too long")
	ErrAddressNotValidated               = errors.New("Address could not be validated")
	ErrAddressServiceUnavailable         = errors.New("Address validation is temporarily unavailable")
	ErrCoverageZoneNotAvailable          = errors.New("Services are not available in this location")
	ErrAddressResolverNotConfigured      = errors.New("Address resolver is not configured")
	ErrCoverageZoneResolverNotConfigured = errors.New("Coverage zone resolver is not configured")
	ErrConsumerAddressNotPersisted       = errors.New("Consumer address is required")
	ErrLatitudeInvalid                   = errors.New("Latitude is invalid")
	ErrLongitudeInvalid                  = errors.New("Longitude is invalid")
)
