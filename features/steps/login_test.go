package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const (
	loginConsumerAuth0ID = "auth0|login-consumer-test"
	loginProviderAuth0ID = "auth0|login-provider-test"
)

func registerLoginSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que estoy registrado como consumidor$`, suite.iAmRegisteredAsConsumer)
	sc.Step(`^que estoy registrado como prestador$`, suite.iAmRegisteredAsProvider)
	sc.Step(`^que no tengo una sesión válida$`, suite.iDoNotHaveAValidSession)
	sc.Step(`^consulto mi información de usuario autenticado$`, suite.requestAuthenticatedUserInfo)
	sc.Step(`^el sistema informa que tengo rol "([^"]*)"$`, suite.systemReportsRole)
	sc.Step(`^el sistema deniega el acceso$`, suite.systemDeniesAccess)
}

func (suite *testSuite) iAmRegisteredAsConsumer() error {
	resp, err := suite.postConsumerRegistrationWithAuth0ID(loginConsumerAuth0ID, consumerRegistrationRequest{
		Email:   "login-consumer@example.com",
		Name:    "Consumer",
		Surname: "Login",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("could not prepare registered consumer: status %d, body %s", resp.StatusCode, string(body))
	}

	suite.currentAuth0ID = loginConsumerAuth0ID
	return nil
}

func (suite *testSuite) iAmRegisteredAsProvider() error {
	if err := suite.thereIsCategoryNamed("Plomería"); err != nil {
		return err
	}

	categoryID, err := suite.categoryIDFor("Plomería")
	if err != nil {
		return err
	}

	resp, err := suite.postProviderRegistrationWithAuth0ID(loginProviderAuth0ID, providerRegistrationRequest{
		Email:                  "login-provider@example.com",
		Name:                   "Provider",
		Surname:                "Login",
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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("could not prepare registered provider: status %d, body %s", resp.StatusCode, string(body))
	}

	suite.currentAuth0ID = loginProviderAuth0ID
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

func (suite *testSuite) systemReportsRole(role string) error {
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("expected status code %d, got %d with body %s", http.StatusOK, suite.lastStatus, string(suite.lastBody))
	}

	var response map[string]any
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	if responseContainsRole(response, role) {
		return nil
	}

	return fmt.Errorf("expected role %q in response, got body %s", role, string(suite.lastBody))
}

func (suite *testSuite) systemDeniesAccess() error {
	if suite.lastStatus != http.StatusUnauthorized && suite.lastStatus != http.StatusForbidden && suite.lastStatus != http.StatusNotFound {
		return fmt.Errorf("expected access to be denied, got status %d with body %s", suite.lastStatus, string(suite.lastBody))
	}

	return nil
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

func responseContainsRole(response map[string]any, role string) bool {
	if returnedRole, ok := response["Role"].(string); ok && isExpectedRole(returnedRole, role) {
		return true
	}

	return false
}

func isExpectedRole(returned string, expected string) bool {
	equivalentRoles := map[string]string{
		"consumer":   "consumer",
		"consumidor": "consumer",
		"provider":   "provider",
		"prestador":  "provider",
	}

	returnedRole, returnedExists := equivalentRoles[returned]
	expectedRole, expectedExists := equivalentRoles[expected]

	return returnedExists && expectedExists && returnedRole == expectedRole
}
