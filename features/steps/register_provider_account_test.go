package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type providerRegistrationRequest struct {
	Email                  string   `json:"email"`
	Name                   string   `json:"name"`
	Surname                string   `json:"surname"`
	CategoryID             int      `json:"category_id"`
	CoverageZone           []string `json:"coverage_zone"`
	CriminalRecordFile     string   `json:"criminal_record_file"`
	CUITCertificateFile    string   `json:"cuit_certificate_file"`
	BiometricValidationID  string   `json:"biometric_validation_id"`
	ProfessionalCredential string   `json:"professional_credential_file"`
}

func registerProviderAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe un usuario con correo "([^"]*)"$`, suite.thereIsNoUserWithEmail)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y rubro "([^"]*)"$`, suite.requestProviderAccountRegistration)
}

func (suite *testSuite) thereIsNoUserWithEmail(_ string) error {
	return suite.userRepository.DeleteAll()
}

func (suite *testSuite) requestProviderAccountRegistration(email, name, surname, categoryName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	resp, err := suite.postProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
	})
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

func (suite *testSuite) postProviderRegistration(req providerRegistrationRequest) (*http.Response, error) {
	return suite.postProviderRegistrationWithAuth0ID("auth0|provider-test", req)
}

func (suite *testSuite) postProviderRegistrationWithAuth0ID(auth0ID string, req providerRegistrationRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/providers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}

	return resp, nil
}
