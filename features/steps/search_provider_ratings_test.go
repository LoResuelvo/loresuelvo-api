package steps_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"
)

type providerRatingSearchResponse struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Surname       string   `json:"surname"`
	RatingAverage *float64 `json:"rating_average"`
	RatingCount   *int     `json:"rating_count"`
}

func registerProviderRatingSearchSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^el resultado del prestador "([^"]*)" informa un promedio de rating de ([0-9.]+) y una cantidad de ratings de (\d+)$`, suite.providerSearchResultShowsRating)
}

func (suite *testSuite) providerSearchResultShowsRating(providerFullName, expectedAverageText, expectedCountText string) error {
	expectedAverage, err := strconv.ParseFloat(expectedAverageText, 64)
	if err != nil {
		return fmt.Errorf("parsing expected provider rating average %q: %w", expectedAverageText, err)
	}
	expectedCount, err := strconv.Atoi(expectedCountText)
	if err != nil {
		return fmt.Errorf("parsing expected provider rating count %q: %w", expectedCountText, err)
	}

	providers, err := suite.providerRatingSearchResponses()
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if !providerMatchesFullName(providerSummaryResponse{
			Name:    provider.Name,
			Surname: provider.Surname,
		}, providerFullName) {
			continue
		}
		if provider.RatingAverage == nil || provider.RatingCount == nil {
			return fmt.Errorf("provider %q result does not include rating_average and rating_count", providerFullName)
		}
		if math.Abs(*provider.RatingAverage-expectedAverage) > 0.000001 {
			return fmt.Errorf("expected provider %q rating average %v, got %v", providerFullName, expectedAverage, *provider.RatingAverage)
		}
		if *provider.RatingCount != expectedCount {
			return fmt.Errorf("expected provider %q rating count %d, got %d", providerFullName, expectedCount, *provider.RatingCount)
		}
		return nil
	}

	return fmt.Errorf("provider %q was not found in rating search response: %s", providerFullName, string(suite.lastBody))
}

func (suite *testSuite) providerRatingSearchResponses() ([]providerRatingSearchResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, err
	}

	var providers []providerRatingSearchResponse
	if err := json.Unmarshal(suite.lastBody, &providers); err != nil {
		return nil, fmt.Errorf("provider rating search response is not valid JSON: %w", err)
	}
	return providers, nil
}
