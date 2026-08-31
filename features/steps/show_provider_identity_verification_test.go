package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/cucumber/godog"
)

func registerShowProviderIdentityVerificationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto mi perfil de usuario$`, suite.requestCurrentUserProfile)
	sc.Step(`^mi estado de verificación de identidad es "([^"]*)"$`, suite.currentUserIdentityVerificationStatusIs)
	sc.Step(`^mi fecha de verificación no está informada$`, suite.currentUserIdentityVerificationDateIsMissing)
	sc.Step(`^busco prestadores del rubro "([^"]*)"$`, suite.filterProvidersByCategory)
	sc.Step(`^"([^"]*)" figura con identidad verificada$`, suite.providerAppearsWithVerifiedIdentity)
	sc.Step(`^"([^"]*)" continúa apareciendo$`, suite.providerContinuesAppearing)
	sc.Step(`^figura sin identidad verificada$`, suite.providerAppearsWithoutVerifiedIdentity)
	sc.Step(`^consulto el perfil público de "([^"]*)"$`, suite.requestPublicProviderProfile)
	sc.Step(`^el perfil indica que la identidad está verificada$`, suite.publicProviderProfileIndicatesVerifiedIdentity)
	sc.Step(`^que la verificación de "([^"]*)" fue rechazada por el riesgo "([^"]*)"$`, suite.identityVerificationWasDeclinedWithRisk)
	sc.Step(`^el perfil no expone el estado detallado de la verificación$`, suite.publicProviderProfileDoesNotExposeVerificationDetails)
	sc.Step(`^el perfil no expone códigos de riesgo ni datos de identidad$`, suite.publicProviderProfileDoesNotExposeRiskCodesOrIdentityData)
}

func (suite *testSuite) requestPublicProviderProfile(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	suite.lastProviderProfileID = providerID
	return suite.getProviderProfile(providerID)
}

func (suite *testSuite) publicProviderProfileIndicatesVerifiedIdentity() error {
	profile, err := suite.providerProfileResponseShouldBeOK()
	if err != nil {
		return err
	}
	if !profile.IdentityVerified {
		return fmt.Errorf("expected public provider profile to indicate verified identity, got body %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) requestCurrentUserProfile() error {
	response, err := suite.getAuthenticatedUserInfo()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	return err
}

func (suite *testSuite) currentUserIdentityVerificationStatusIs(expectedStatus string) error {
	profile, err := suite.currentUserIdentityVerificationProfile()
	if err != nil {
		return err
	}
	if strings.TrimSpace(profile.IdentityVerificationStatus) != expectedStatus {
		return fmt.Errorf("expected identity verification status %q, got %q", expectedStatus, profile.IdentityVerificationStatus)
	}
	return nil
}

func (suite *testSuite) currentUserIdentityVerificationDateIsMissing() error {
	profile, err := suite.currentUserIdentityVerificationProfile()
	if err != nil {
		return err
	}
	if profile.IdentityVerifiedOn != nil {
		return fmt.Errorf("expected identity verification date to be null, got %s", profile.IdentityVerifiedOn)
	}
	return nil
}

func (suite *testSuite) currentUserIdentityVerificationProfile() (identityVerificationStatusResponse, error) {
	if suite.lastStatus != http.StatusOK {
		return identityVerificationStatusResponse{}, fmt.Errorf("expected current user profile status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var profile identityVerificationStatusResponse
	if err := json.Unmarshal(suite.lastBody, &profile); err != nil {
		return profile, fmt.Errorf("decoding current user identity verification profile: %w", err)
	}
	return profile, nil
}

func (suite *testSuite) providerAppearsWithVerifiedIdentity(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	for _, foundProvider := range providers {
		if foundProvider.ID != providerID {
			continue
		}
		if !foundProvider.IdentityVerified {
			return fmt.Errorf("expected provider %q to have verified identity, got body %s", email, suite.lastBody)
		}
		return nil
	}
	return fmt.Errorf("expected provider %q in search results, got body %s", email, suite.lastBody)
}

func (suite *testSuite) providerContinuesAppearing(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	for _, foundProvider := range providers {
		if foundProvider.ID != providerID {
			continue
		}
		suite.lastIdentityVerificationProviderID = providerID
		return nil
	}
	return fmt.Errorf("expected provider %q in search results, got body %s", email, suite.lastBody)
}

func (suite *testSuite) providerAppearsWithoutVerifiedIdentity() error {
	if suite.lastIdentityVerificationProviderID == 0 {
		return fmt.Errorf("provider identity verification search result was not prepared")
	}
	providers, err := suite.providerSummaryResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	for _, foundProvider := range providers {
		if foundProvider.ID != suite.lastIdentityVerificationProviderID {
			continue
		}
		if foundProvider.IdentityVerified {
			return fmt.Errorf("expected provider %d to appear without verified identity, got body %s", suite.lastIdentityVerificationProviderID, suite.lastBody)
		}
		return nil
	}
	return fmt.Errorf("expected provider %d in search results, got body %s", suite.lastIdentityVerificationProviderID, suite.lastBody)
}

func (suite *testSuite) identityVerificationWasDeclinedWithRisk(email, riskCode string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	verification, err := identityVerificationFixtureForProvider(suite, providerID, string(identityverification.StatusDeclined))
	if err != nil {
		return err
	}
	verification.RiskCodes = []string{riskCode}
	if err := suite.identityVerificationRepository.Save(context.Background(), verification); err != nil {
		return fmt.Errorf("saving declined identity verification fixture: %w", err)
	}
	suite.expectedIdentityVerificationSessionID = verification.ExternalSessionID
	return nil
}

func (suite *testSuite) publicProviderProfileDoesNotExposeVerificationDetails() error {
	response, err := suite.providerProfileJSONShouldBeOK()
	if err != nil {
		return err
	}
	return providerProfileDoesNotExposeFields(response, suite.lastBody, []string{
		"identity_verification_status",
		"verification_status",
		"status",
		"identity_verified_on",
		"verified_on",
		"session_id",
		"external_session_id",
		"workflow_id",
		"workflow_version",
	})
}

func (suite *testSuite) publicProviderProfileDoesNotExposeRiskCodesOrIdentityData() error {
	response, err := suite.providerProfileJSONShouldBeOK()
	if err != nil {
		return err
	}
	return providerProfileDoesNotExposeFields(response, suite.lastBody, []string{
		"risk_codes",
		"risk_code",
		"identity_data",
		"extracted_identity",
		"document_number",
		"document_type",
		"document_expiry",
		"date_of_birth",
		"nationality",
		"address",
		"first_name",
		"last_name",
		"email",
		"auth_id",
		"session_id",
		"external_session_id",
		"session_token",
		"verification_url",
		"workflow_id",
		"workflow_version",
	})
}
