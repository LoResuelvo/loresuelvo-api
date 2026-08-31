package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const (
	providerProfileFixtureEmail  = "provider-profile@example.com"
	nonExistingProviderProfileID = 999999999
)

type providerProfilePhotoResponse struct {
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

type providerProfileCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type providerProfileResponse struct {
	ID               int                             `json:"id"`
	Name             string                          `json:"name"`
	Surname          string                          `json:"surname"`
	ProfilePhoto     *providerProfilePhotoResponse   `json:"profile_photo"`
	Category         providerProfileCategoryResponse `json:"category"`
	IdentityVerified bool                            `json:"identity_verified"`
}

func registerGetProviderProfileSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un prestador llamado "([^"]*)" en el rubro "([^"]*)" con foto de perfil$`, suite.thereIsProviderWithProfilePhoto)
	sc.Step(`^consulto el perfil del prestador "([^"]*)"$`, suite.requestProviderProfile)
	sc.Step(`^consulto el perfil de un prestador inexistente$`, suite.requestNonExistingProviderProfile)
	sc.Step(`^el sistema devuelve el perfil del prestador$`, suite.systemReturnsProviderProfile)
	sc.Step(`^el perfil muestra el nombre "([^"]*)" y apellido "([^"]*)"$`, suite.providerProfileShowsNameAndSurname)
	sc.Step(`^el perfil muestra la foto del prestador$`, suite.providerProfileShowsPhoto)
	sc.Step(`^el perfil muestra el rubro "([^"]*)"$`, suite.providerProfileShowsCategory)
	sc.Step(`^el perfil no expone el correo ni la identidad de autenticación del prestador$`, suite.providerProfileDoesNotExposePrivateIdentity)
	sc.Step(`^el sistema informa que el prestador no fue encontrado$`, suite.systemReportsProviderProfileNotFound)
}

func (suite *testSuite) thereIsProviderWithProfilePhoto(fullName, categoryName string) error {
	name, surname, err := splitFullName(fullName)
	if err != nil {
		return err
	}
	if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(
		providerProfileFixtureEmail,
		name,
		surname,
		categoryName,
	); err != nil {
		return err
	}

	providerID, err := suite.providerIDByEmail(providerProfileFixtureEmail)
	if err != nil {
		return fmt.Errorf("finding provider profile fixture: %w", err)
	}
	suite.lastProviderProfileID = providerID
	return nil
}

func (suite *testSuite) requestProviderProfile(_ string) error {
	if suite.lastProviderProfileID == 0 {
		return fmt.Errorf("provider profile fixture was not prepared")
	}
	return suite.getProviderProfile(suite.lastProviderProfileID)
}

func (suite *testSuite) requestNonExistingProviderProfile() error {
	return suite.getProviderProfile(nonExistingProviderProfileID)
}

func (suite *testSuite) getProviderProfile(providerID int) error {
	httpReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/providers/%d", suite.server.URL, providerID), nil)
	if err != nil {
		return err
	}
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read provider profile response: %w", err)
	}
	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) systemReturnsProviderProfile() error {
	_, err := suite.providerProfileResponseShouldBeOK()
	return err
}

func (suite *testSuite) providerProfileShowsNameAndSurname(name, surname string) error {
	response, err := suite.providerProfileResponseShouldBeOK()
	if err != nil {
		return err
	}
	if response.Name != name || response.Surname != surname {
		return fmt.Errorf("expected provider name %q and surname %q, got body %s", name, surname, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerProfileShowsPhoto() error {
	response, err := suite.providerProfileResponseShouldBeOK()
	if err != nil {
		return err
	}
	if response.ProfilePhoto == nil || response.ProfilePhoto.OriginalName == "" || response.ProfilePhoto.URL == "" {
		return fmt.Errorf("expected provider profile photo with original_name and url, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerProfileShowsCategory(categoryName string) error {
	response, err := suite.providerProfileResponseShouldBeOK()
	if err != nil {
		return err
	}
	if response.Category.ID == 0 || response.Category.Name != categoryName {
		return fmt.Errorf("expected provider category %q, got body %s", categoryName, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerProfileDoesNotExposePrivateIdentity() error {
	response, err := suite.providerProfileJSONShouldBeOK()
	if err != nil {
		return err
	}
	return providerProfileDoesNotExposeFields(response, suite.lastBody, []string{"email", "auth_id", "AuthID"})
}

func (suite *testSuite) systemReportsProviderProfileNotFound() error {
	return suite.lastResponseShouldHaveStatusCode(http.StatusNotFound)
}

func (suite *testSuite) providerProfileResponseShouldBeOK() (*providerProfileResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, err
	}

	var response providerProfileResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return nil, fmt.Errorf("provider profile response is not valid JSON: %w", err)
	}
	if response.ID == 0 {
		return nil, fmt.Errorf("expected provider profile id, got body %s", string(suite.lastBody))
	}
	return &response, nil
}

func (suite *testSuite) providerProfileJSONShouldBeOK() (map[string]json.RawMessage, error) {
	if _, err := suite.providerProfileResponseShouldBeOK(); err != nil {
		return nil, err
	}

	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return nil, fmt.Errorf("provider profile response is not valid JSON: %w", err)
	}
	return response, nil
}

func providerProfileDoesNotExposeFields(response map[string]json.RawMessage, body []byte, forbiddenFields []string) error {
	for _, forbiddenField := range forbiddenFields {
		if _, exists := response[forbiddenField]; exists {
			return fmt.Errorf("expected provider profile not to expose %q, got body %s", forbiddenField, string(body))
		}
	}
	return nil
}

func splitFullName(fullName string) (string, string, error) {
	for index, character := range fullName {
		if character == ' ' {
			name, surname := fullName[:index], fullName[index+1:]
			if name != "" && surname != "" {
				return name, surname, nil
			}
		}
	}
	return "", "", fmt.Errorf("full name %q must contain name and surname", fullName)
}
