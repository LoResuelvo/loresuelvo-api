package location_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/location"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/stretchr/testify/require"
)

type coverageZoneFinderStub struct {
	provider   string
	externalID string
	marketCode string
	zone       *coveragezone.CoverageZone
	err        error
}

func (stub *coverageZoneFinderStub) FindByExternalReferenceInMarket(_ context.Context, provider, externalID, marketCode string) (*coveragezone.CoverageZone, error) {
	stub.provider = provider
	stub.externalID = externalID
	stub.marketCode = marketCode
	return stub.zone, stub.err
}

func TestGoogleAddressResolverReturnsOnlyRooftopOrInterpolatedCoordinates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "test-key", request.URL.Query().Get("key"))
		require.Equal(t, "Av. Rivadavia 5100, Argentina", request.URL.Query().Get("address"))
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": "OK",
			"results": []any{map[string]any{
				"geometry": map[string]any{
					"location":      map[string]float64{"lat": -34.62, "lng": -58.43},
					"location_type": "ROOFTOP",
				},
			}},
		})
	}))
	defer server.Close()

	resolver := location.NewGoogleAddressResolver(location.GoogleAddressResolverConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  server.Client(),
	})

	point, err := resolver.Resolve(context.Background(), consumer.Address{Street: "Av. Rivadavia", StreetNumber: "5100"})

	require.NoError(t, err)
	require.Equal(t, consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43}, point)
}

func TestGoogleAddressResolverRejectsApproximateOrPartialResults(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "approximate",
			payload: map[string]any{
				"status": "OK",
				"results": []any{map[string]any{
					"geometry": map[string]any{
						"location":      map[string]float64{"lat": -34.62, "lng": -58.43},
						"location_type": "APPROXIMATE",
					},
				}},
			},
		},
		{
			name: "partial",
			payload: map[string]any{
				"status": "OK",
				"results": []any{map[string]any{
					"partial_match": true,
					"geometry": map[string]any{
						"location":      map[string]float64{"lat": -34.62, "lng": -58.43},
						"location_type": "ROOFTOP",
					},
				}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(writer).Encode(test.payload))
			}))
			defer server.Close()
			resolver := location.NewGoogleAddressResolver(location.GoogleAddressResolverConfig{
				APIKey: "test-key", BaseURL: server.URL, Client: server.Client(),
			})

			point, err := resolver.Resolve(context.Background(), consumer.Address{Street: "Av. Rivadavia", StreetNumber: "5100"})

			require.ErrorIs(t, err, consumer.ErrAddressNotValidated)
			require.Equal(t, consumer.GeoPoint{}, point)
		})
	}
}

func TestGoogleAddressResolverMapsZeroResultsAndTransportFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"status": "ZERO_RESULTS"})
	}))
	resolver := location.NewGoogleAddressResolver(location.GoogleAddressResolverConfig{
		APIKey: "test-key", BaseURL: server.URL, Client: server.Client(),
	})

	point, err := resolver.Resolve(context.Background(), consumer.Address{Street: "Unknown", StreetNumber: "1"})

	require.ErrorIs(t, err, consumer.ErrAddressNotValidated)
	require.Equal(t, consumer.GeoPoint{}, point)
	server.Close()

	point, err = resolver.Resolve(context.Background(), consumer.Address{Street: "Unknown", StreetNumber: "1"})
	require.ErrorIs(t, err, consumer.ErrAddressServiceUnavailable)
	require.Equal(t, consumer.GeoPoint{}, point)
}

func TestGoogleCoverageZoneResolverUsesTransientPlaceIDOnlyForInternalLookup(t *testing.T) {
	zoneFinder := &coverageZoneFinderStub{zone: &coveragezone.CoverageZone{ID: 6, Name: "Comuna 6", Enabled: true}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "test-key", request.Header.Get("X-Goog-Api-Key"))
		var payload map[string][]map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Len(t, payload["search_values"], 1)
		require.Equal(t, "administrative_area_level_2", payload["search_values"][0]["place_type"])
		require.Equal(t, "ar", payload["search_values"][0]["region_code"])
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"matches": []any{map[string]any{"matchedPlaceId": "places/google-commune-6"}},
		})
	}))
	defer server.Close()

	resolver := location.NewGoogleCoverageZoneResolver(location.GoogleCoverageZoneResolverConfig{
		APIKey: "test-key", BaseURL: server.URL, Client: server.Client(),
	}, zoneFinder)

	zone, err := resolver.Resolve(context.Background(), consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43})

	require.NoError(t, err)
	require.Equal(t, zoneFinder.zone, zone)
	require.Equal(t, "GOOGLE", zoneFinder.provider)
	require.Equal(t, "google-commune-6", zoneFinder.externalID)
	require.Equal(t, "CABA", zoneFinder.marketCode)
}

func TestGoogleCoverageZoneResolverRejectsUnmatchedRegion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"matches": []any{
				map[string]any{"candidates": []string{"candidate"}},
			},
		})
	}))
	defer server.Close()

	resolver := location.NewGoogleCoverageZoneResolver(location.GoogleCoverageZoneResolverConfig{
		APIKey: "test-key", BaseURL: server.URL, Client: server.Client(),
	}, &coverageZoneFinderStub{})

	zone, err := resolver.Resolve(context.Background(), consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43})

	require.ErrorIs(t, err, coveragezone.ErrDoesNotExist)
	require.Nil(t, zone)
}
