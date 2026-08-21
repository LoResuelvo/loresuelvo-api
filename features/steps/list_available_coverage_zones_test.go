package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/cucumber/godog"
)

const coverageZoneListingAuth0ID = "auth0|coverage-zone-listing-test"

type coverageZoneListItemResponse struct {
	ID       int                          `json:"id"`
	Name     string                       `json:"name"`
	Boundary coverageZoneBoundaryResponse `json:"boundary"`
}

type coverageZoneBoundaryResponse struct {
	Type    string `json:"type"`
	PlaceID string `json:"place_id"`
}

func registerListAvailableCoverageZonesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el market de cobertura "([^"]*)" está habilitado$`, suite.coverageMarketIsEnabled)
	sc.Step(`^que están habilitadas las zonas de cobertura "([^"]*)" y "([^"]*)" en el market "([^"]*)"$`, suite.enabledCoverageZonesInMarket)
	sc.Step(`^consulto el listado de zonas de cobertura disponibles$`, suite.requestAvailableCoverageZoneList)
	sc.Step(`^el sistema responde exitosamente con las siguientes zonas de cobertura:$`, suite.systemRespondsWithCoverageZoneList)
	sc.Step(`^cada zona de cobertura incluye un identificador interno estable$`, suite.eachAvailableCoverageZoneHasStableID)
	sc.Step(`^cada zona de cobertura incluye una referencia de Google utilizable para representar su límite$`, suite.eachAvailableCoverageZoneHasGoogleBoundaryReference)
	sc.Step(`^el listado incluye la zona de cobertura "([^"]*)"$`, suite.availableCoverageZoneListIncludes)
	sc.Step(`^el listado no incluye la zona de cobertura "([^"]*)"$`, suite.availableCoverageZoneListExcludes)
	sc.Step(`^que no existen zonas de cobertura habilitadas en el market "([^"]*)"$`, suite.noAvailableCoverageZonesInMarket)
	sc.Step(`^el sistema responde exitosamente con un listado de zonas de cobertura vacío$`, suite.systemRespondsWithEmptyCoverageZoneList)
}

func (suite *testSuite) coverageMarketIsEnabled(code string) error {
	market, err := suite.coverageZoneRepository.FindMarketByCode(context.Background(), code)
	if err == nil {
		if !market.Enabled {
			return fmt.Errorf("coverage market %q is disabled", code)
		}
		return nil
	}
	if !errors.Is(err, coveragezone.ErrDoesNotExist) {
		return fmt.Errorf("could not find coverage market %q: %w", code, err)
	}

	name := strings.TrimSpace(code)
	if strings.EqualFold(name, coveragezone.DefaultMarketCode) {
		name = "Ciudad Autónoma de Buenos Aires"
	}
	newMarket, err := coveragezone.NewMarket(code, name)
	if err != nil {
		return err
	}
	if _, err := suite.coverageZoneRepository.SaveMarket(context.Background(), *newMarket); err != nil {
		return fmt.Errorf("could not create coverage market %q: %w", code, err)
	}

	return nil
}

func (suite *testSuite) enabledCoverageZonesInMarket(firstName, secondName, marketCode string) error {
	if err := suite.coverageZoneRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not reset coverage zones: %w", err)
	}
	if err := suite.coverageMarketIsEnabled(marketCode); err != nil {
		return err
	}

	market, err := suite.coverageZoneRepository.FindMarketByCode(context.Background(), marketCode)
	if err != nil {
		return fmt.Errorf("could not load coverage market %q: %w", marketCode, err)
	}

	for _, name := range []string{firstName, secondName} {
		zone, err := coveragezone.New(market.ID, listingCoverageZoneCode(marketCode, name), name, coveragezone.KindCommune)
		if err != nil {
			return fmt.Errorf("could not build coverage zone %q: %w", name, err)
		}
		savedZone, err := suite.coverageZoneRepository.Save(context.Background(), *zone)
		if err != nil {
			return fmt.Errorf("could not create coverage zone %q: %w", name, err)
		}
		reference, err := coveragezone.NewExternalReference(
			savedZone.ID,
			"GOOGLE",
			listingCoverageZoneGooglePlaceID(savedZone.Code),
			"bdd",
		)
		if err != nil {
			return fmt.Errorf("could not build Google boundary reference for coverage zone %q: %w", name, err)
		}
		if err := suite.coverageZoneRepository.SaveExternalReference(context.Background(), *reference); err != nil {
			return fmt.Errorf("could not save Google boundary reference for coverage zone %q: %w", name, err)
		}
	}

	return nil
}

func (suite *testSuite) noAvailableCoverageZonesInMarket(marketCode string) error {
	if err := suite.coverageMarketIsEnabled(marketCode); err != nil {
		return err
	}
	if err := suite.coverageZoneRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not reset coverage zones: %w", err)
	}

	return nil
}

func listingCoverageZoneCode(marketCode, name string) string {
	codeName := strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(name)), "-"))
	parts := strings.Split(codeName, "-")
	if len(parts) > 1 {
		if number, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			parts[len(parts)-1] = fmt.Sprintf("%02d", number)
			codeName = strings.Join(parts, "-")
		}
	}

	return strings.ToUpper(strings.TrimSpace(marketCode)) + "-" + codeName
}

func listingCoverageZoneGooglePlaceID(zoneCode string) string {
	return "google-place-" + strings.ToLower(strings.TrimSpace(zoneCode))
}

func (suite *testSuite) requestAvailableCoverageZoneList() error {
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+"/coverage-zones", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(coverageZoneListingAuth0ID, nil))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = response.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) systemRespondsWithCoverageZoneList(expected *godog.Table) error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(expected.Rows) < 1 {
		return fmt.Errorf("expected a coverage-zone table header")
	}

	expectedNames := make([]string, 0, len(expected.Rows)-1)
	for _, row := range expected.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one coverage-zone name column, got %d", len(row.Cells))
		}
		expectedNames = append(expectedNames, row.Cells[0].Value)
	}
	if len(zones) != len(expectedNames) {
		return fmt.Errorf("expected %d coverage zones, got %d with body %s", len(expectedNames), len(zones), string(suite.lastBody))
	}

	for index, expectedName := range expectedNames {
		if zones[index].Name != expectedName {
			return fmt.Errorf("expected coverage zone %q at position %d, got %q with body %s", expectedName, index, zones[index].Name, string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) eachAvailableCoverageZoneHasStableID() error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	seenIDs := make(map[int]struct{}, len(zones))
	for _, listedZone := range zones {
		if listedZone.ID <= 0 {
			return fmt.Errorf("coverage zone %q has invalid internal id %d", listedZone.Name, listedZone.ID)
		}
		if _, exists := seenIDs[listedZone.ID]; exists {
			return fmt.Errorf("coverage zone id %d appears more than once", listedZone.ID)
		}
		seenIDs[listedZone.ID] = struct{}{}

		storedZone, err := suite.coverageZoneRepository.FindByID(context.Background(), listedZone.ID)
		if err != nil {
			return fmt.Errorf("listed coverage zone id %d cannot be used for registration: %w", listedZone.ID, err)
		}
		if storedZone.Name != listedZone.Name {
			return fmt.Errorf("listed coverage zone id %d does not identify %q", listedZone.ID, listedZone.Name)
		}
	}

	return nil
}

func (suite *testSuite) eachAvailableCoverageZoneHasGoogleBoundaryReference() error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	seenPlaceIDs := make(map[string]struct{}, len(zones))
	for _, listedZone := range zones {
		if listedZone.Boundary.Type != "google_place" {
			return fmt.Errorf(
				"coverage zone %q has boundary type %q, expected google_place",
				listedZone.Name,
				listedZone.Boundary.Type,
			)
		}
		placeID := strings.TrimSpace(listedZone.Boundary.PlaceID)
		if placeID == "" {
			return fmt.Errorf("coverage zone %q has an empty Google place id", listedZone.Name)
		}
		if _, exists := seenPlaceIDs[placeID]; exists {
			return fmt.Errorf("Google place id %q appears more than once", placeID)
		}
		seenPlaceIDs[placeID] = struct{}{}

		storedZone, err := suite.coverageZoneRepository.FindByExternalReference(context.Background(), "GOOGLE", placeID)
		if err != nil {
			return fmt.Errorf("Google place id %q cannot resolve coverage zone %q: %w", placeID, listedZone.Name, err)
		}
		if storedZone.ID != listedZone.ID {
			return fmt.Errorf(
				"Google place id %q resolves coverage zone id %d instead of %d",
				placeID,
				storedZone.ID,
				listedZone.ID,
			)
		}
	}

	return nil
}

func (suite *testSuite) availableCoverageZoneListIncludes(name string) error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, zone := range zones {
		if zone.Name == name {
			return nil
		}
	}

	return fmt.Errorf("expected coverage-zone list to include %q, got body %s", name, string(suite.lastBody))
}

func (suite *testSuite) availableCoverageZoneListExcludes(name string) error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, zone := range zones {
		if zone.Name == name {
			return fmt.Errorf("expected coverage-zone list not to include %q, got body %s", name, string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) systemRespondsWithEmptyCoverageZoneList() error {
	zones, err := suite.coverageZoneListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(zones) != 0 {
		return fmt.Errorf("expected empty coverage-zone list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) coverageZoneListResponseShouldHaveStatusCode(statusCode int) ([]coverageZoneListItemResponse, error) {
	if suite.lastStatus != statusCode {
		return nil, fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}

	var zones []coverageZoneListItemResponse
	if err := json.Unmarshal(suite.lastBody, &zones); err != nil {
		return nil, fmt.Errorf("response is not valid JSON coverage-zone list: %w", err)
	}

	return zones, nil
}
