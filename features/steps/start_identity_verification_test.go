package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const identityVerificationSessionPath = "/providers/me/identity-verification-sessions"

type identityVerificationSessionResponse struct {
	SessionID       uuid.UUID `json:"session_id"`
	SessionToken    string    `json:"session_token"`
	VerificationURL string    `json:"verification_url"`
	Status          string    `json:"status"`
}

func registerStartIdentityVerificationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^inicio mi verificación de identidad$`, suite.startIdentityVerification)
	sc.Step(`^el sistema entrega las credenciales temporales de la verificación$`, suite.systemReturnsTemporaryVerificationCredentials)
	sc.Step(`^la verificación de "([^"]*)" queda en estado "([^"]*)"$`, suite.providerVerificationHasStatus)
	sc.Step(`^la sesión queda asociada solamente al prestador "([^"]*)"$`, suite.sessionBelongsOnlyToProvider)
}

func (suite *testSuite) startIdentityVerification() error {
	request, err := http.NewRequest(http.MethodPost, suite.server.URL+identityVerificationSessionPath, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("starting identity verification: %w", err)
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	return err
}

func (suite *testSuite) identityVerificationResponse() (identityVerificationSessionResponse, error) {
	var response identityVerificationSessionResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return response, fmt.Errorf("decoding identity verification response: %w", err)
	}
	return response, nil
}

func (suite *testSuite) systemReturnsTemporaryVerificationCredentials() error {
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	response, err := suite.identityVerificationResponse()
	if err != nil {
		return err
	}
	if response.SessionID == uuid.Nil || strings.TrimSpace(response.SessionToken) == "" || strings.TrimSpace(response.VerificationURL) == "" {
		return fmt.Errorf("expected temporary identity verification credentials")
	}
	return nil
}

func (suite *testSuite) providerVerificationHasStatus(_ string, expectedStatus string) error {
	response, err := suite.identityVerificationResponse()
	if err != nil {
		return err
	}
	verification, err := suite.identityVerificationRepository.FindBySessionID(suite.scenarioContext, response.SessionID)
	if err != nil {
		return err
	}
	if verification == nil || string(verification.Status) != expectedStatus {
		return fmt.Errorf("expected persisted verification status %q", expectedStatus)
	}
	return nil
}

func (suite *testSuite) sessionBelongsOnlyToProvider(email string) error {
	response, err := suite.identityVerificationResponse()
	if err != nil {
		return err
	}
	verification, err := suite.identityVerificationRepository.FindBySessionID(suite.scenarioContext, response.SessionID)
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	if verification == nil || verification.ProviderID != providerID {
		return fmt.Errorf("identity verification session is not associated with provider %q", email)
	}
	return nil
}
