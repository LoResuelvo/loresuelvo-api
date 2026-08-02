package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

const unregisteredLoginAuth0ID = "auth0|unregistered-login-test"

type currentUserProfilePhotoResponse struct {
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

type currentUserCategoryResponse struct {
	Name string `json:"name"`
}

type currentUserResponse struct {
	Name         string                           `json:"name"`
	Surname      string                           `json:"surname"`
	Email        string                           `json:"email"`
	Role         string                           `json:"role"`
	ProfilePhoto *currentUserProfilePhotoResponse `json:"profile_photo,omitempty"`
	Category     *currentUserCategoryResponse     `json:"category,omitempty"`
}

func registerLoginSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un consumidor registrado con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" sin foto de perfil$`, suite.thereIsRegisteredConsumerWithoutProfilePhoto)
	sc.Step(`^que existe un consumidor registrado con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" con la foto de perfil cargada$`, suite.thereIsRegisteredConsumerWithProfilePhoto)
	sc.Step(`^que cargué una foto de perfil válida para mi registro como prestador$`, suite.uploadValidLoginProviderProfilePhoto)
	sc.Step(`^que existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y la foto de perfil cargada$`, suite.thereIsRegisteredProviderWithProfilePhoto)
	sc.Step(`^que estoy autenticado con una identidad que no pertenece a un usuario registrado$`, suite.iAmAuthenticatedAsUnregisteredUser)
	sc.Step(`^que no tengo una sesión válida$`, suite.iDoNotHaveAValidSession)
	sc.Step(`^consulto mi información de usuario autenticado$`, suite.requestAuthenticatedUserInfo)
	sc.Step(`^el sistema devuelve mi perfil de (consumidor|prestador)$`, suite.systemReturnsMyUserProfile)
	sc.Step(`^el perfil contiene el nombre "([^"]*)", apellido "([^"]*)" y correo "([^"]*)"$`, suite.profileContainsPersonalInformation)
	sc.Step(`^el perfil informa el rol "([^"]*)"$`, suite.profileReportsRole)
	sc.Step(`^el perfil no incluye una foto de perfil$`, suite.profileDoesNotIncludeProfilePhoto)
	sc.Step(`^el perfil incluye la foto de perfil$`, suite.profileIncludesProfilePhoto)
	sc.Step(`^el perfil incluye el rubro "([^"]*)"$`, suite.profileIncludesCategory)
	sc.Step(`^el sistema deniega el acceso$`, suite.systemDeniesAccess)
	sc.Step(`^el sistema informa que el usuario no fue encontrado$`, suite.systemReportsUserNotFound)
}

func (suite *testSuite) thereIsRegisteredConsumerWithoutProfilePhoto(email, name, surname string) error {
	return suite.registerConsumerProfileFixture(email, consumerRegistrationRequest{Email: email, Name: name, Surname: surname})
}

func (suite *testSuite) thereIsRegisteredConsumerWithProfilePhoto(email, name, surname string) error {
	auth0ID := auth0IDForConsumerEmail(email)
	fileID, err := suite.uploadValidProfilePhotoFor(auth0ID)
	if err != nil {
		return err
	}

	return suite.registerConsumerProfileFixture(email, consumerRegistrationRequest{
		Email: email, Name: name, Surname: surname, ProfilePhotoFileID: fileID,
	})
}

func (suite *testSuite) registerConsumerProfileFixture(email string, request consumerRegistrationRequest) error {
	resp, err := suite.postConsumerRegistrationWithAuth0ID(auth0IDForConsumerEmail(email), request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return requireFixtureCreated(resp, "consumer")
}

func (suite *testSuite) uploadValidLoginProviderProfilePhoto() error {
	fileID, err := suite.uploadValidProviderProfilePhotoFor(auth0IDForProviderEmail("juan@example.com"))
	if err != nil {
		return err
	}
	suite.providerProfilePhotoFileID = fileID
	return nil
}

func (suite *testSuite) thereIsRegisteredProviderWithProfilePhoto(email, name, surname, categoryName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	auth0ID := auth0IDForProviderEmail(email)
	fileID := suite.providerProfilePhotoFileID
	if fileID == "" {
		fileID, err = suite.uploadValidProviderProfilePhotoFor(auth0ID)
		if err != nil {
			return err
		}
	}

	resp, err := suite.postProviderRegistrationWithAuth0ID(auth0ID, providerRegistrationRequest{
		Email: email, Name: name, Surname: surname, CategoryID: categoryID, ProfilePhotoFileID: fileID,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return requireFixtureCreated(resp, "provider")
}

func requireFixtureCreated(resp *http.Response, fixtureName string) error {
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare registered %s: status %d, body %s", fixtureName, resp.StatusCode, string(body))
}

func (suite *testSuite) iAmAuthenticatedAsUnregisteredUser() error {
	suite.currentAuth0ID = unregisteredLoginAuth0ID
	return nil
}

func (suite *testSuite) iDoNotHaveAValidSession() error {
	suite.currentAuth0ID = ""
	return nil
}

func (suite *testSuite) requestAuthenticatedUserInfo() error {
	resp, err := suite.getAuthenticatedUserInfo()
	if err != nil {
		return err
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

func (suite *testSuite) systemReturnsMyUserProfile(userType string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	expectedRole := map[string]string{"consumidor": "consumer", "prestador": "provider"}[userType]
	return suite.profileReportsRole(expectedRole)
}

func (suite *testSuite) profileContainsPersonalInformation(name, surname, email string) error {
	response, err := suite.currentUserResponse()
	if err != nil {
		return err
	}
	if response.Name != name || response.Surname != surname || response.Email != email {
		return fmt.Errorf("expected profile %q %q <%s>, got body %s", name, surname, email, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) profileReportsRole(role string) error {
	response, err := suite.currentUserResponse()
	if err != nil {
		return err
	}
	if response.Role != role {
		return fmt.Errorf("expected role %q, got body %s", role, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) profileDoesNotIncludeProfilePhoto() error {
	response, err := suite.currentUserResponse()
	if err != nil {
		return err
	}
	if response.ProfilePhoto != nil {
		return fmt.Errorf("expected profile without profile_photo, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) profileIncludesProfilePhoto() error {
	response, err := suite.currentUserResponse()
	if err != nil {
		return err
	}
	if response.ProfilePhoto == nil || strings.TrimSpace(response.ProfilePhoto.OriginalName) == "" || strings.TrimSpace(response.ProfilePhoto.URL) == "" {
		return fmt.Errorf("expected profile_photo with original_name and url, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) profileIncludesCategory(categoryName string) error {
	response, err := suite.currentUserResponse()
	if err != nil {
		return err
	}
	if response.Category == nil || response.Category.Name != categoryName {
		return fmt.Errorf("expected category %q, got body %s", categoryName, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemDeniesAccess() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusUnauthorized); err != nil {
		return err
	}
	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemReportsUserNotFound() error {
	return suite.lastResponseShouldHaveStatusCode(http.StatusNotFound)
}

func (suite *testSuite) currentUserResponse() (*currentUserResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, err
	}
	var response currentUserResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return nil, fmt.Errorf("current user response is not valid JSON: %w", err)
	}
	return &response, nil
}

func (suite *testSuite) getAuthenticatedUserInfo() (*http.Response, error) {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/me", nil)
	if err != nil {
		return nil, err
	}
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}
	return resp, nil
}
