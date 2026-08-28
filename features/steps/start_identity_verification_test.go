package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
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
	sc.Step(`^intento iniciar la verificación de identidad del prestador "([^"]*)"$`, suite.tryStartIdentityVerificationForProvider)
	sc.Step(`^el sistema deniega el inicio de la verificación$`, suite.systemDeniesIdentityVerificationStart)
	sc.Step(`^no se crea ninguna sesión para "([^"]*)"$`, suite.noIdentityVerificationSessionForProvider)
	sc.Step(`^que la verificación de "([^"]*)" está en estado "([^"]*)"$`, suite.identityVerificationHasStatus)
	sc.Step(`^inicio nuevamente mi verificación de identidad$`, suite.startIdentityVerification)
	sc.Step(`^intento iniciar otra verificación de identidad$`, suite.tryStartAnotherIdentityVerification)
	sc.Step(`^el sistema entrega las credenciales de la misma sesión$`, suite.systemReturnsSameVerificationSession)
	sc.Step(`^se conserva una única sesión activa para "([^"]*)"$`, suite.onlyOneActiveVerificationSession)
	sc.Step(`^el sistema informa que mi identidad ya está verificada$`, suite.systemReportsIdentityAlreadyApproved)
	sc.Step(`^no se crea una nueva sesión$`, suite.noNewIdentityVerificationSession)
	sc.Step(`^que el verificador de identidad no está disponible$`, suite.identityVerifierIsUnavailable)
	sc.Step(`^el sistema informa que la verificación no está disponible temporalmente$`, suite.systemReportsIdentityVerifierUnavailable)
	sc.Step(`^no se guarda una sesión incompleta$`, suite.noIncompleteIdentityVerificationSession)
}

func (suite *testSuite) tryStartIdentityVerificationForProvider(_ string) error {
	return suite.startIdentityVerification()
}

func (suite *testSuite) tryStartAnotherIdentityVerification() error {
	return suite.startIdentityVerification()
}

func (suite *testSuite) systemDeniesIdentityVerificationStart() error {
	if suite.lastStatus != http.StatusForbidden {
		return fmt.Errorf("expected status 403, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) noIdentityVerificationSessionForProvider(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	verification, err := suite.identityVerificationRepository.FindLatestByProviderID(suite.scenarioContext, providerID)
	if err != nil {
		return err
	}
	if verification != nil {
		return fmt.Errorf("expected no identity verification session for %q", email)
	}
	return nil
}

func (suite *testSuite) identityVerificationHasStatus(email, status string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	verification, err := identityVerificationFixtureForProvider(suite, providerID, status)
	if err != nil {
		return err
	}
	suite.expectedIdentityVerificationSessionID = verification.ExternalSessionID
	return nil
}

func identityVerificationFixtureForProvider(suite *testSuite, providerID int, status string) (*identityverification.IdentityVerification, error) {
	verification, err := identityverification.NewVerification(providerID, uuid.New(), uuid.New(), "fake", 1, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	verification.Status = identityverification.VerificationStatus(status)
	if err := suite.identityVerificationRepository.Save(context.Background(), verification); err != nil {
		return nil, err
	}
	return verification, nil
}

func (suite *testSuite) systemReturnsSameVerificationSession() error {
	response, err := suite.identityVerificationResponse()
	if err != nil {
		return err
	}
	if response.SessionID != suite.expectedIdentityVerificationSessionID {
		return fmt.Errorf("expected session %s, got %s", suite.expectedIdentityVerificationSessionID, response.SessionID)
	}
	return nil
}

func (suite *testSuite) onlyOneActiveVerificationSession(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	verifications, err := suite.identityVerificationRepository.FindByProviderID(context.Background(), providerID)
	if err != nil {
		return err
	}
	if len(verifications) != 1 {
		return fmt.Errorf("expected one identity verification session, got %d", len(verifications))
	}
	return nil
}

func (suite *testSuite) systemReportsIdentityAlreadyApproved() error {
	if suite.lastStatus != http.StatusConflict {
		return fmt.Errorf("expected status 409, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) noNewIdentityVerificationSession() error {
	providerID, err := suite.providerIDByEmail("juan.plomero@example.com")
	if err != nil {
		return err
	}
	verifications, err := suite.identityVerificationRepository.FindByProviderID(context.Background(), providerID)
	if err != nil {
		return err
	}
	if len(verifications) != 1 {
		return fmt.Errorf("expected one identity verification session, got %d", len(verifications))
	}
	return nil
}

func (suite *testSuite) identityVerifierIsUnavailable() error {
	suite.identityVerifier.SetAvailable(false)
	return nil
}

func (suite *testSuite) systemReportsIdentityVerifierUnavailable() error {
	if suite.lastStatus != http.StatusServiceUnavailable {
		return fmt.Errorf("expected status 503, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) noIncompleteIdentityVerificationSession() error {
	providerID, err := suite.providerIDByEmail("juan.plomero@example.com")
	if err != nil {
		return err
	}
	verifications, err := suite.identityVerificationRepository.FindByProviderID(context.Background(), providerID)
	if err != nil {
		return err
	}
	if len(verifications) != 0 {
		return fmt.Errorf("expected no incomplete identity verification session")
	}
	return nil
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
