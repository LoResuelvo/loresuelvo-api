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

const providerFilterAuth0ID = "auth0|provider-search-test"

type providerSummaryResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Surname      string `json:"surname"`
	CategoryName string `json:"category_name"`
}

func registerFilterProvidersByCategorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y rubro "([^"]*)"$`, suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory)
	sc.Step(`^filtro técnicos por el rubro "([^"]*)"$`, suite.filterProvidersByCategory)
	sc.Step(`^intento filtrar técnicos sin indicar rubro$`, suite.tryFilterProvidersWithoutCategory)
	sc.Step(`^el sistema muestra al técnico "([^"]*)"$`, suite.systemShowsProvider)
	sc.Step(`^el sistema muestra solamente al técnico "([^"]*)" en el resultado$`, suite.systemShowsOnlyProvider)
	sc.Step(`^el sistema muestra un listado de técnicos vacío$`, suite.systemShowsEmptyProviderList)
	sc.Step(`^filtro técnicos por un rubro inexistente$`, suite.filterProvidersByNonExistingCategoryID)
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

func (suite *testSuite) filterProvidersByCategory(categoryName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	suite.lastProviderFilterCategoryName = categoryName
	return suite.requestProviderFilter(url.Values{"category_id": []string{fmt.Sprintf("%d", categoryID)}})
}

func (suite *testSuite) tryFilterProvidersWithoutCategory() error {
	suite.lastProviderFilterCategoryName = ""
	return suite.requestProviderFilter(url.Values{})
}

func (suite *testSuite) filterProvidersByNonExistingCategoryID() error {
	suite.lastProviderFilterCategoryName = ""
	return suite.requestProviderFilter(url.Values{"category_id": []string{"999999999"}})
}

func (suite *testSuite) requestProviderFilter(query url.Values) error {
	requestURL := suite.server.URL + "/providers"
	if encodedQuery := query.Encode(); encodedQuery != "" {
		requestURL += "?" + encodedQuery
	}

	httpReq, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(providerFilterAuth0ID, nil))

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
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if providerListIncludesFullName(providers, fullName) {
		return nil
	}

	return fmt.Errorf("expected provider search results to include %q, got body %s", fullName, string(suite.lastBody))
}

func (suite *testSuite) systemShowsOnlyProvider(fullName string) error {
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
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
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
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

func (suite *testSuite) providerSummaryResponseShouldHaveStatusCode(statusCode int) ([]providerSummaryResponse, error) {
	if suite.lastStatus != statusCode {
		return nil, fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}

	var providers []providerSummaryResponse
	if err := json.Unmarshal(suite.lastBody, &providers); err != nil {
		return nil, fmt.Errorf("response is not valid JSON provider summary list with id, name, surname and category_name: %w", err)
	}

	for _, provider := range providers {
		if provider.ID == 0 {
			return nil, fmt.Errorf("expected each provider to include id, got body %s", string(suite.lastBody))
		}

		if strings.TrimSpace(provider.Name) == "" || strings.TrimSpace(provider.Surname) == "" || strings.TrimSpace(provider.CategoryName) == "" {
			return nil, fmt.Errorf("expected each provider to include name, surname and category_name, got body %s", string(suite.lastBody))
		}

		if strings.TrimSpace(suite.lastProviderFilterCategoryName) != "" && !sameNormalizedName(provider.CategoryName, suite.lastProviderFilterCategoryName) {
			return nil, fmt.Errorf("expected provider category %q to match searched category %q, got body %s", provider.CategoryName, suite.lastProviderFilterCategoryName, string(suite.lastBody))
		}
	}

	return providers, nil
}

func providerListIncludesFullName(providers []providerSummaryResponse, fullName string) bool {
	for _, provider := range providers {
		if providerMatchesFullName(provider, fullName) {
			return true
		}
	}

	return false
}

func providerMatchesFullName(provider providerSummaryResponse, expectedFullName string) bool {
	return normalizeComparableName(providerFullName(provider)) == normalizeComparableName(expectedFullName)
}

func providerFullName(provider providerSummaryResponse) string {
	return strings.TrimSpace(strings.Join([]string{provider.Name, provider.Surname}, " "))
}

func sameNormalizedName(left, right string) bool {
	return normalizeComparableName(left) == normalizeComparableName(right)
}

func normalizeComparableName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

func auth0IDForProviderEmail(email string) string {
	replacer := strings.NewReplacer("@", "-", ".", "-", "+", "-", "_", "-")
	return "auth0|provider-search-" + replacer.Replace(strings.ToLower(email))
}
