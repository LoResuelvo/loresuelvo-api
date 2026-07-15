package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const (
	providerProfilePhotoPurpose = "provider_profile_photo"
	validProfilePhotoSizeBytes  = 1024 * 1024
	oversizedProfilePhotoBytes  = 6 * 1024 * 1024
)

type presignFileRequest struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int    `json:"size_bytes"`
	Purpose      string `json:"purpose"`
}

type presignFileResponse struct {
	FileID    string            `json:"file_id"`
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
}

type confirmFileRequest struct {
	Key       string `json:"key"`
	MimeType  string `json:"mime_type"`
	SizeBytes int    `json:"size_bytes"`
}

func registerProviderWithProfilePhotoSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que cargué una foto de perfil válida$`, suite.uploadValidProviderProfilePhoto)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y rubro "([^"]*)" sin cargar foto de perfil$`, suite.requestProviderAccountRegistrationWithoutProfilePhoto)
	sc.Step(`^intento cargar una foto de perfil con formato no válido para el registro$`, suite.tryUploadProviderProfilePhotoWithInvalidFormat)
	sc.Step(`^intento cargar una foto de perfil que pesa 6 MB para el registro$`, suite.tryUploadOversizedProviderProfilePhoto)
	sc.Step(`^el sistema me indica que la foto de perfil es obligatoria$`, suite.systemReportsProfilePhotoIsRequired)
	sc.Step(`^el sistema me indica que la foto de perfil no pudo ser cargada$`, suite.systemReportsProfilePhotoCouldNotBeUploaded)
}

func (suite *testSuite) uploadValidProviderProfilePhoto() error {
	fileID, err := suite.uploadValidProviderProfilePhotoFor("auth0|provider-test")
	if err != nil {
		return err
	}
	suite.providerProfilePhotoFileID = fileID
	return nil
}

func (suite *testSuite) uploadValidProviderProfilePhotoFor(auth0ID string) (string, error) {
	return suite.uploadValidProfilePhotoFor(auth0ID)
}

func (suite *testSuite) uploadValidProfilePhotoFor(auth0ID string) (string, error) {
	response, err := suite.requestProfilePhotoPresign(auth0ID, presignFileRequest{
		OriginalName: "foto-perfil.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    validProfilePhotoSizeBytes,
		Purpose:      providerProfilePhotoPurpose,
	})
	if err != nil {
		return "", err
	}

	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return "", err
	}
	if response.FileID == "" {
		return "", fmt.Errorf("expected profile photo upload response to include file_id, got body %s", string(suite.lastBody))
	}
	if response.Key == "" {
		return "", fmt.Errorf("expected profile photo upload response to include key, got body %s", string(suite.lastBody))
	}
	if response.UploadURL == "" {
		return "", fmt.Errorf("expected profile photo upload response to include upload_url, got body %s", string(suite.lastBody))
	}

	if err := suite.putProfilePhotoObject(*response, "image/jpeg", validProfilePhotoSizeBytes); err != nil {
		return "", err
	}
	if err := suite.confirmProfilePhotoUpload(auth0ID, *response); err != nil {
		return "", err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return "", err
	}

	return response.FileID, nil
}

func (suite *testSuite) requestProviderAccountRegistrationWithoutProfilePhoto(email, name, surname, categoryName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	return suite.requestProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
	})
}

func (suite *testSuite) tryUploadProviderProfilePhotoWithInvalidFormat() error {
	_, err := suite.requestProfilePhotoPresign("auth0|provider-test", presignFileRequest{
		OriginalName: "foto-perfil.gif",
		MimeType:     "image/gif",
		SizeBytes:    validProfilePhotoSizeBytes,
		Purpose:      providerProfilePhotoPurpose,
	})
	return err
}

func (suite *testSuite) tryUploadOversizedProviderProfilePhoto() error {
	_, err := suite.requestProfilePhotoPresign("auth0|provider-test", presignFileRequest{
		OriginalName: "foto-perfil.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    oversizedProfilePhotoBytes,
		Purpose:      providerProfilePhotoPurpose,
	})
	return err
}

func (suite *testSuite) systemReportsProfilePhotoIsRequired() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.registrationResponseShouldSay("Profile photo is required")
}

func (suite *testSuite) systemReportsProfilePhotoCouldNotBeUploaded() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.registrationResponseShouldSay("Profile photo could not be uploaded")
}

func (suite *testSuite) requestProfilePhotoPresign(auth0ID string, payload presignFileRequest) (*presignFileResponse, error) {
	resp, err := suite.postJSONWithAuth(auth0ID, "/files/presign", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	var response presignFileResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse profile photo upload response: %w", err)
		}
	}

	return &response, nil
}

func (suite *testSuite) putProfilePhotoObject(upload presignFileResponse, mimeType string, sizeBytes int) error {
	body := bytes.Repeat([]byte{0xff}, sizeBytes)
	httpReq, err := http.NewRequest(http.MethodPut, upload.UploadURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to prepare profile photo object upload: %w", err)
	}
	for key, value := range upload.Headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("Content-Type", mimeType)
	httpReq.ContentLength = int64(sizeBytes)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("profile photo object upload failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read profile photo object upload response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("expected profile photo object upload status 2xx, got %d with body %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func (suite *testSuite) confirmProfilePhotoUpload(auth0ID string, upload presignFileResponse) error {
	resp, err := suite.postJSONWithAuth(auth0ID, "/files/"+upload.FileID+"/confirm", confirmFileRequest{
		Key:       upload.Key,
		MimeType:  "image/jpeg",
		SizeBytes: validProfilePhotoSizeBytes,
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

func (suite *testSuite) postJSONWithAuth(auth0ID, path string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+path, bytes.NewReader(body))
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
