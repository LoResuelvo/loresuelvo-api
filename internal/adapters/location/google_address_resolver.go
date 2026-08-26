package location

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

const defaultGeocodingURL = "https://maps.googleapis.com/maps/api/geocode/json"

type GoogleAddressResolverConfig struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

type GoogleAddressResolver struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewGoogleAddressResolver(config GoogleAddressResolverConfig) *GoogleAddressResolver {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultGeocodingURL
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: defaultLocationTimeout}
	}

	return &GoogleAddressResolver{
		apiKey:  strings.TrimSpace(config.APIKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

type googleGeocodingResponse struct {
	Status  string `json:"status"`
	Results []struct {
		PartialMatch bool `json:"partial_match"`
		Geometry     struct {
			Location struct {
				Latitude  float64 `json:"lat"`
				Longitude float64 `json:"lng"`
			} `json:"location"`
			LocationType string `json:"location_type"`
		} `json:"geometry"`
	} `json:"results"`
}

func (resolver *GoogleAddressResolver) Resolve(ctx context.Context, address consumer.Address) (consumer.GeoPoint, error) {
	if resolver == nil || resolver.client == nil || resolver.apiKey == "" {
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}

	requestURL, err := url.Parse(resolver.baseURL)
	if err != nil {
		return consumer.GeoPoint{}, fmt.Errorf("building geocoding request URL: %w", err)
	}
	query := requestURL.Query()
	query.Set("address", address.Query()+", Argentina")
	query.Set("key", resolver.apiKey)
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return consumer.GeoPoint{}, fmt.Errorf("building geocoding request: %w", err)
	}

	response, err := resolver.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
		}
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}

	var payload googleGeocodingResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}
	if payload.Status != "OK" || len(payload.Results) == 0 {
		if payload.Status == "ZERO_RESULTS" {
			return consumer.GeoPoint{}, consumer.ErrAddressNotValidated
		}
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}
	if len(payload.Results) != 1 || payload.Results[0].PartialMatch || !isAddressPrecisionAccepted(payload.Results[0].Geometry.LocationType) {
		return consumer.GeoPoint{}, consumer.ErrAddressNotValidated
	}

	point, err := consumer.NewGeoPoint(
		payload.Results[0].Geometry.Location.Latitude,
		payload.Results[0].Geometry.Location.Longitude,
	)
	if err != nil {
		return consumer.GeoPoint{}, consumer.ErrAddressNotValidated
	}

	return point, nil
}

func isAddressPrecisionAccepted(locationType string) bool {
	switch strings.ToUpper(strings.TrimSpace(locationType)) {
	case "ROOFTOP", "RANGE_INTERPOLATED":
		return true
	default:
		return false
	}
}
