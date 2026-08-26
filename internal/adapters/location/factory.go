package location

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func NewGoogleAddressResolverFromEnv() *GoogleAddressResolver {
	return NewGoogleAddressResolver(GoogleAddressResolverConfig{
		APIKey:  strings.TrimSpace(os.Getenv("GOOGLE_MAPS_API_KEY")),
		BaseURL: strings.TrimSpace(os.Getenv("GOOGLE_GEOCODING_API_URL")),
		Client:  newHTTPClientFromEnv(),
	})
}

func NewGoogleCoverageZoneResolverFromEnv(zoneFinder CoverageZoneByExternalReferenceFinder) *GoogleCoverageZoneResolver {
	return NewGoogleCoverageZoneResolver(GoogleCoverageZoneResolverConfig{
		APIKey:      strings.TrimSpace(os.Getenv("GOOGLE_MAPS_API_KEY")),
		BaseURL:     strings.TrimSpace(os.Getenv("GOOGLE_REGION_LOOKUP_API_URL")),
		CountryCode: strings.TrimSpace(os.Getenv("GOOGLE_REGION_LOOKUP_REGION_CODE")),
		Client:      newHTTPClientFromEnv(),
	}, zoneFinder)
}

func newHTTPClientFromEnv() *http.Client {
	timeout := defaultLocationTimeout
	configuredTimeout := strings.TrimSpace(os.Getenv("GOOGLE_LOCATION_TIMEOUT"))
	if configuredTimeout != "" {
		if duration, err := time.ParseDuration(configuredTimeout); err == nil && duration > 0 {
			timeout = duration
		} else if seconds, err := strconv.Atoi(configuredTimeout); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	return &http.Client{Timeout: timeout}
}
