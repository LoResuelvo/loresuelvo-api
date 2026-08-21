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
	Email                  string `json:"email"`
	Name                   string `json:"name"`
	Surname                string `json:"surname"`
	CategoryID             int    `json:"category_id"`
	CoverageZoneIDs        []int  `json:"coverage_zone_ids,omitempty"`
	CriminalRecordFile     string `json:"criminal_record_file"`
	CUITCertificateFile    string `json:"cuit_certificate_file"`
	BiometricValidationID  string `json:"biometric_validation_id"`
	ProfessionalCredential string `json:"professional_credential_file"`
	ProfilePhotoFileID     string `json:"profile_photo_file_id,omitempty"`
}

func registerProviderAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe un usuario con correo "([^"]*)"$`, suite.thereIsNoUserWithEmail)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y rubro "([^"]*)"$`, suite.requestProviderAccountRegistration)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y sin rubro$`, suite.requestProviderAccountRegistrationWithoutCategory)
	sc.Step(`^el sistema me indica que el rubro es obligatorio$`, suite.systemReportsCategoryIsRequired)
}

func (suite *testSuite) thereIsNoUserWithEmail(_ string) error {
	return suite.userRepository.DeleteAll()
}

func (suite *testSuite) requestProviderAccountRegistration(email, name, surname, categoryName string) error {
	if suite.providerProfilePhotoFileID == "" {
		if err := suite.uploadValidProviderProfilePhoto(); err != nil {
			return err
		}
	}

	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	coverageZoneID, err := suite.ensureDefaultProviderCoverageZone()
	if err != nil {
		return err
	}

	if err := suite.requestProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CoverageZoneIDs:        []int{coverageZoneID},
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
		ProfilePhotoFileID:     suite.providerProfilePhotoFileID,
	}); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) requestProviderAccountRegistrationWithoutCategory(email, name, surname string) error {
	return suite.requestProviderRegistration(map[string]any{
		"email":                        email,
		"name":                         name,
		"surname":                      surname,
		"criminal_record_file":         "criminal-record.pdf",
		"cuit_certificate_file":        "cuit-certificate.pdf",
		"biometric_validation_id":      "biometric-validation-approved",
		"professional_credential_file": "professional-license-or-certificate.pdf",
	})
}

func (suite *testSuite) requestProviderRegistration(payload any) error {
	resp, err := suite.postProviderRegistrationPayload("auth0|provider-test", payload)
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

func (suite *testSuite) systemReportsCategoryIsRequired() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.registrationResponseShouldSay("Category id is required")
}

func (suite *testSuite) postProviderRegistrationWithAuth0ID(auth0ID string, req providerRegistrationRequest) (*http.Response, error) {
	if req.ProfilePhotoFileID == "" {
		fileID, err := suite.uploadValidProviderProfilePhotoFor(auth0ID)
		if err != nil {
			return nil, err
		}
		req.ProfilePhotoFileID = fileID
	}
	return suite.postProviderRegistrationPayload(auth0ID, req)
}

func (suite *testSuite) postProviderRegistrationPayload(auth0ID string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
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
