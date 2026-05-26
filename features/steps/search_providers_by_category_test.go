package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"
)

const providerSearchAuth0ID = "auth0|provider-search-test"

type providerSearchResponse struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func registerSearchProvidersByCategorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y rubro "([^"]*)"$`, suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory)
	sc.Step(`^busco técnicos del rubro "([^"]*)"$`, suite.searchProvidersByCategory)
	sc.Step(`^intento buscar técnicos sin indicar rubro$`, suite.trySearchProvidersWithoutCategory)
	sc.Step(`^el sistema muestra al técnico "([^"]*)"$`, suite.systemShowsProvider)
	sc.Step(`^el sistema muestra solamente al técnico "([^"]*)" en el resultado$`, suite.systemShowsOnlyProvider)
	sc.Step(`^el sistema muestra un listado de técnicos vacío$`, suite.systemShowsEmptyProviderList)
	sc.Step(`^el sistema me indica que el rubro no existe$`, suite.systemReportsCategoryDoesNotExist)
}

func (suite *testSuite) thereIsRegisteredProviderWithEmailNameSurnameAndCategory(email, name, surname, categoryName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	resp, err := suite.postProviderRegistrationWithAuth0ID(auth0IDForProviderEmail(email), providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CoverageZone:           []string{"Zona Norte"},
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare registered provider: status %d, body %s", resp.StatusCode, string(body))
}

func (suite *testSuite) searchProvidersByCategory(categoryName string) error {
	return suite.requestProviderSearch(url.Values{"category": []string{categoryName}})
}

func (suite *testSuite) trySearchProvidersWithoutCategory() error {
	return suite.requestProviderSearch(url.Values{})
}

func (suite *testSuite) requestProviderSearch(query url.Values) error {
	requestURL := suite.server.URL + "/providers"
	if encodedQuery := query.Encode(); encodedQuery != "" {
		requestURL += "?" + encodedQuery
	}

	httpReq, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(providerSearchAuth0ID, nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) systemShowsProvider(fullName string) error {
	providers, err := suite.providerSearchResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if providerListIncludesFullName(providers, fullName) {
		return nil
	}

	return fmt.Errorf("expected provider search results to include %q, got body %s", fullName, string(suite.lastBody))
}

func (suite *testSuite) systemShowsOnlyProvider(fullName string) error {
	providers, err := suite.providerSearchResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(providers) != 1 {
		return fmt.Errorf("expected exactly one provider, got %d with body %s", len(providers), string(suite.lastBody))
	}

	if !providerMatchesFullName(providers[0], fullName) {
		return fmt.Errorf("expected only provider %q, got body %s", fullName, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsEmptyProviderList() error {
	providers, err := suite.providerSearchResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(providers) != 0 {
		return fmt.Errorf("expected empty provider list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemReportsCategoryDoesNotExist() error {
	if suite.lastStatus != http.StatusBadRequest && suite.lastStatus != http.StatusNotFound {
		return fmt.Errorf("expected status code %d or %d, got %d with body %s", http.StatusBadRequest, http.StatusNotFound, suite.lastStatus, string(suite.lastBody))
	}

	return suite.registrationResponseShouldSay("Category does not exist")
}

func (suite *testSuite) providerSearchResponseShouldHaveStatusCode(statusCode int) ([]providerSearchResponse, error) {
	if suite.lastStatus != statusCode {
		return nil, fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}

	var providers []providerSearchResponse
	if err := json.Unmarshal(suite.lastBody, &providers); err != nil {
		return nil, fmt.Errorf("response is not valid JSON provider list with name and surname: %w", err)
	}

	for _, provider := range providers {
		if strings.TrimSpace(provider.Name) == "" || strings.TrimSpace(provider.Surname) == "" {
			return nil, fmt.Errorf("expected each provider to include name and surname, got body %s", string(suite.lastBody))
		}
	}

	return providers, nil
}

func providerListIncludesFullName(providers []providerSearchResponse, fullName string) bool {
	for _, provider := range providers {
		if providerMatchesFullName(provider, fullName) {
			return true
		}
	}

	return false
}

func providerMatchesFullName(provider providerSearchResponse, expectedFullName string) bool {
	return normalizeComparableName(providerFullName(provider)) == normalizeComparableName(expectedFullName)
}

func providerFullName(provider providerSearchResponse) string {
	return strings.TrimSpace(strings.Join([]string{provider.Name, provider.Surname}, " "))
}

func normalizeComparableName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

func auth0IDForProviderEmail(email string) string {
	replacer := strings.NewReplacer("@", "-", ".", "-", "+", "-", "_", "-")
	return "auth0|provider-search-" + replacer.Replace(strings.ToLower(email))
}
