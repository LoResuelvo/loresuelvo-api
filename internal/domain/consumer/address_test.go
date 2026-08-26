package consumer_test

import (
	"math"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/require"
)

func TestNewAddressTrimsProvidedValues(t *testing.T) {
	address, err := consumer.NewAddress(" Av. Rivadavia ", " 5100 ", " 4 ", " B ")

	require.NoError(t, err)
	require.Equal(t, consumer.Address{
		Street:       "Av. Rivadavia",
		StreetNumber: "5100",
		Floor:        "4",
		Unit:         "B",
	}, *address)
}

func TestNewAddressRejectsMissingAddress(t *testing.T) {
	address, err := consumer.NewAddress("", "", "", "")

	require.ErrorIs(t, err, consumer.ErrAddressRequired)
	require.Nil(t, address)
}

func TestNewAddressRejectsMissingStreetOrNumber(t *testing.T) {
	address, err := consumer.NewAddress("", "5100", "", "")
	require.ErrorIs(t, err, consumer.ErrStreetRequired)
	require.Nil(t, address)

	address, err = consumer.NewAddress("Av. Rivadavia", "", "", "")
	require.ErrorIs(t, err, consumer.ErrStreetNumberRequired)
	require.Nil(t, address)
}

func TestNewGeoPointRejectsOutOfRangeAndNonFiniteCoordinates(t *testing.T) {
	for _, test := range []struct {
		name      string
		latitude  float64
		longitude float64
		err       error
	}{
		{name: "latitude below range", latitude: -91, longitude: 0, err: consumer.ErrLatitudeInvalid},
		{name: "latitude above range", latitude: 91, longitude: 0, err: consumer.ErrLatitudeInvalid},
		{name: "longitude below range", latitude: 0, longitude: -181, err: consumer.ErrLongitudeInvalid},
		{name: "longitude above range", latitude: 0, longitude: 181, err: consumer.ErrLongitudeInvalid},
		{name: "non finite latitude", latitude: math.Inf(1), longitude: 0, err: consumer.ErrLatitudeInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			point, err := consumer.NewGeoPoint(test.latitude, test.longitude)

			require.ErrorIs(t, err, test.err)
			require.Equal(t, consumer.GeoPoint{}, point)
		})
	}
}

func TestNewAddressRejectsValuesLongerThanPersistenceLimits(t *testing.T) {
	address, err := consumer.NewAddress(strings.Repeat("a", 201), "1", "", "")

	require.ErrorIs(t, err, consumer.ErrAddressFieldTooLong)
	require.Nil(t, address)
}
