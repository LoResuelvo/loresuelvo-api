package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const identityVerificationWebhookPath = "/webhooks/didit"

type identityVerificationWebhookPayload struct {
	CreatedAt       int64     `json:"created_at"`
	EventID         uuid.UUID `json:"event_id"`
	RiskCodes       []string  `json:"risk_codes,omitempty"`
	SessionID       uuid.UUID `json:"session_id"`
	Status          string    `json:"status"`
	Timestamp       int64     `json:"timestamp"`
	VendorData      string    `json:"vendor_data"`
	WebhookType     string    `json:"webhook_type"`
	WorkflowID      uuid.UUID `json:"workflow_id"`
	WorkflowVersion int       `json:"workflow_version"`
}

type identityVerificationStatusResponse struct {
	IdentityVerificationStatus string     `json:"identity_verification_status"`
	IdentityVerifiedOn         *time.Time `json:"identity_verified_on"`
}

func registerProcessIdentityVerificationResultSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el prestador "([^"]*)" inició una verificación de identidad$`, suite.providerStartedIdentityVerification)
	sc.Step(`^el verificador informa de forma auténtica que la sesión está "([^"]*)"$`, suite.verifierReportsIdentityVerificationStatus)
	sc.Step(`^el verificador informa de forma auténtica que la sesión está "([^"]*)" por el riesgo "([^"]*)"$`, suite.verifierReportsIdentityVerificationStatusWithRisk)
	sc.Step(`^"([^"]*)" consulta en su perfil el estado "([^"]*)"$`, suite.providerChecksIdentityVerificationStatus)
	sc.Step(`^que la identidad de "([^"]*)" está aprobada$`, suite.identityVerificationIsApproved)
	sc.Step(`^la fecha de verificación de "([^"]*)" es "([^"]*)"$`, suite.providerChecksIdentityVerificationDate)
	sc.Step(`^el sistema conserva solamente el código de riesgo "([^"]*)"$`, suite.systemKeepsOnlyRiskCode)
	sc.Step(`^"([^"]*)" ya no figura como prestador verificado$`, suite.providerIsNotVerified)
}

func (suite *testSuite) providerStartedIdentityVerification(email string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(email)
	if err := suite.startIdentityVerification(); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("starting identity verification returned status %d: %s", suite.lastStatus, suite.lastBody)
	}
	response, err := suite.identityVerificationResponse()
	if err != nil {
		return err
	}
	suite.expectedIdentityVerificationSessionID = response.SessionID
	return nil
}

func (suite *testSuite) verifierReportsIdentityVerificationStatus(status string) error {
	return suite.verifierReportsIdentityVerificationStatusWithRisk(status, "")
}

func (suite *testSuite) verifierReportsIdentityVerificationStatusWithRisk(status, riskCode string) error {
	verification, err := suite.identityVerificationRepository.FindBySessionID(context.Background(), suite.expectedIdentityVerificationSessionID)
	if err != nil {
		return fmt.Errorf("finding started identity verification: %w", err)
	}
	if verification == nil {
		return fmt.Errorf("started identity verification session was not found")
	}

	now := suite.clock.Now()
	payload := identityVerificationWebhookPayload{
		EventID: uuid.New(), SessionID: verification.ExternalSessionID, Status: diditStatusFor(status),
		WebhookType: "status.updated", CreatedAt: now.Unix(), Timestamp: now.Unix(),
		WorkflowID: verification.WorkflowID, WorkflowVersion: verification.WorkflowVersion,
		VendorData: strconv.Itoa(verification.ProviderID),
	}
	if strings.TrimSpace(riskCode) != "" {
		payload.RiskCodes = []string{riskCode}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding identity verification webhook: %w", err)
	}
	request, err := http.NewRequest(http.MethodPost, suite.server.URL+identityVerificationWebhookPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	signer := newIdentityVerificationWebhookSignerStub()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Signature-V2", signer.Sign(body))
	request.Header.Set("X-Timestamp", strconv.FormatInt(now.Unix(), 10))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("sending identity verification webhook: %w", err)
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	return err
}

func (suite *testSuite) providerChecksIdentityVerificationStatus(email, expectedStatus string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(email)
	response, err := suite.getAuthenticatedUserInfo()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading current provider profile: %w", err)
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("reading current provider profile returned status %d: %s", suite.lastStatus, suite.lastBody)
	}
	var profile identityVerificationStatusResponse
	if err := json.Unmarshal(suite.lastBody, &profile); err != nil {
		return fmt.Errorf("decoding current provider profile: %w", err)
	}
	if strings.TrimSpace(profile.IdentityVerificationStatus) != expectedStatus {
		return fmt.Errorf("expected identity verification status %q, got %q", expectedStatus, profile.IdentityVerificationStatus)
	}
	return nil
}

func (suite *testSuite) identityVerificationIsApproved(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	if suite.expectedIdentityVerificationSessionID == uuid.Nil {
		verification, err := identityVerificationFixtureForProvider(suite, providerID, string(identityverification.StatusApproved))
		if err != nil {
			return err
		}
		suite.expectedIdentityVerificationSessionID = verification.ExternalSessionID
		return nil
	}
	verification, err := suite.identityVerificationRepository.FindBySessionID(context.Background(), suite.expectedIdentityVerificationSessionID)
	if err != nil {
		return err
	}
	if verification == nil || verification.ProviderID != providerID {
		return fmt.Errorf("identity verification session is not associated with provider %q", email)
	}
	verification.Status = identityverification.StatusApproved
	return suite.identityVerificationRepository.Save(context.Background(), verification)
}

func (suite *testSuite) providerChecksIdentityVerificationDate(email, expectedDateTime string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(email)
	response, err := suite.getAuthenticatedUserInfo()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading current provider profile: %w", err)
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("reading current provider profile returned status %d: %s", suite.lastStatus, suite.lastBody)
	}
	var profile identityVerificationStatusResponse
	if err := json.Unmarshal(suite.lastBody, &profile); err != nil {
		return fmt.Errorf("decoding current provider profile: %w", err)
	}
	expected, err := time.Parse(time.RFC3339Nano, expectedDateTime)
	if err != nil {
		return fmt.Errorf("parsing expected verification date: %w", err)
	}
	if profile.IdentityVerifiedOn == nil || !profile.IdentityVerifiedOn.Equal(expected) {
		return fmt.Errorf("expected verification date %s, got %v", expected, profile.IdentityVerifiedOn)
	}
	return nil
}

func (suite *testSuite) systemKeepsOnlyRiskCode(expectedRiskCode string) error {
	verification, err := suite.identityVerificationRepository.FindBySessionID(context.Background(), suite.expectedIdentityVerificationSessionID)
	if err != nil {
		return err
	}
	if verification == nil || verification.Status != identityverification.StatusDeclined {
		return fmt.Errorf("expected declined identity verification session")
	}
	if len(verification.RiskCodes) != 1 || verification.RiskCodes[0] != expectedRiskCode {
		return fmt.Errorf("expected only risk code %q, got %v", expectedRiskCode, verification.RiskCodes)
	}
	return nil
}

func (suite *testSuite) providerIsNotVerified(email string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(email)
	response, err := suite.getAuthenticatedUserInfo()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading current provider profile: %w", err)
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("reading current provider profile returned status %d: %s", suite.lastStatus, suite.lastBody)
	}
	var profile identityVerificationStatusResponse
	if err := json.Unmarshal(suite.lastBody, &profile); err != nil {
		return fmt.Errorf("decoding current provider profile: %w", err)
	}
	if profile.IdentityVerificationStatus == string(identityverification.StatusApproved) {
		return fmt.Errorf("provider %q is still verified", email)
	}
	return nil
}

func diditStatusFor(status string) string {
	return map[string]string{
		"not_started":   "Not Started",
		"in_progress":   "In Progress",
		"awaiting_user": "Awaiting User",
		"in_review":     "In Review",
		"approved":      "Approved",
		"declined":      "Declined",
		"resubmitted":   "Resubmitted",
		"abandoned":     "Abandoned",
		"expired":       "Expired",
		"kyc_expired":   "Kyc Expired",
	}[status]
}
