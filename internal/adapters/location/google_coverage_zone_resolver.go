package location

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
)

const (
	defaultRegionLookupURL       = "https://regionlookup.googleapis.com/v1alpha:searchRegion"
	googleRegionProvider         = "GOOGLE"
	defaultRegionLookupPlaceType = "administrative_area_level_2"
)

type CoverageZoneByExternalReferenceFinder interface {
	FindByExternalReferenceInMarket(ctx context.Context, provider, externalID, marketCode string) (*coveragezone.CoverageZone, error)
}

type CoverageZoneByMarketAndNameFinder interface {
	FindByMarketCodeAndName(ctx context.Context, marketCode, name string) (*coveragezone.CoverageZone, error)
}

type GoogleCoverageZoneResolverConfig struct {
	APIKey      string
	BaseURL     string
	CountryCode string
	Client      *http.Client
}

type GoogleCoverageZoneResolver struct {
	apiKey      string
	baseURL     string
	countryCode string
	marketCode  string
	placeType   string
	client      *http.Client
	zoneFinder  CoverageZoneByExternalReferenceFinder
}

func NewGoogleCoverageZoneResolver(
	config GoogleCoverageZoneResolverConfig,
	zoneFinder CoverageZoneByExternalReferenceFinder,
) *GoogleCoverageZoneResolver {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultRegionLookupURL
	}
	countryCode := strings.ToLower(strings.TrimSpace(config.CountryCode))
	if countryCode == "" {
		countryCode = "ar"
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: defaultLocationTimeout}
	}

	return &GoogleCoverageZoneResolver{
		apiKey:      strings.TrimSpace(config.APIKey),
		baseURL:     strings.TrimRight(baseURL, "/"),
		countryCode: countryCode,
		marketCode:  coveragezone.DefaultMarketCode,
		placeType:   defaultRegionLookupPlaceType,
		client:      client,
		zoneFinder:  zoneFinder,
	}
}

type googleRegionLookupRequest struct {
	SearchValues []googleRegionSearchValue `json:"search_values"`
}

type googleRegionSearchValue struct {
	LatLng       googleLatLng `json:"latlng"`
	PlaceType    string       `json:"place_type"`
	RegionCode   string       `json:"region_code"`
	LanguageCode string       `json:"language_code"`
}

type googleLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type googleRegionLookupResponse struct {
	Matches []struct {
		MatchedPlaceID string   `json:"matchedPlaceId"`
		Candidates     []string `json:"candidates"`
	} `json:"matches"`
}

func (resolver *GoogleCoverageZoneResolver) Resolve(ctx context.Context, point consumer.GeoPoint) (*coveragezone.CoverageZone, error) {
	if resolver == nil || resolver.client == nil || resolver.apiKey == "" || resolver.zoneFinder == nil {
		return nil, consumer.ErrAddressServiceUnavailable
	}

	payload := googleRegionLookupRequest{
		SearchValues: []googleRegionSearchValue{{
			LatLng: googleLatLng{
				Latitude:  point.Latitude,
				Longitude: point.Longitude,
			},
			PlaceType:    resolver.placeType,
			RegionCode:   resolver.countryCode,
			LanguageCode: "es",
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding region lookup request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, resolver.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building region lookup request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Goog-Api-Key", resolver.apiKey)

	response, err := resolver.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, consumer.ErrAddressServiceUnavailable
		}
		return nil, consumer.ErrAddressServiceUnavailable
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, consumer.ErrAddressServiceUnavailable
	}

	var responsePayload googleRegionLookupResponse
	if err := json.NewDecoder(response.Body).Decode(&responsePayload); err != nil {
		return nil, consumer.ErrAddressServiceUnavailable
	}
	if len(responsePayload.Matches) == 0 || strings.TrimSpace(responsePayload.Matches[0].MatchedPlaceID) == "" {
		return nil, coveragezone.ErrDoesNotExist
	}

	externalID := strings.TrimSpace(responsePayload.Matches[0].MatchedPlaceID)
	externalID = strings.TrimPrefix(externalID, "places/")
	zone, err := resolver.zoneFinder.FindByExternalReferenceInMarket(ctx, googleRegionProvider, externalID, resolver.marketCode)
	if err != nil {
		if errors.Is(err, coveragezone.ErrDoesNotExist) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage zone for Google region %q: %w", externalID, err)
	}
	if zone == nil {
		return nil, coveragezone.ErrDoesNotExist
	}

	return zone, nil
}
