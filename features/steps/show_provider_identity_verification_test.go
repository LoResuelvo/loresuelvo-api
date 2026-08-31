package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

func registerShowProviderIdentityVerificationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto mi perfil de usuario$`, suite.requestCurrentUserProfile)
	sc.Step(`^mi estado de verificación de identidad es "([^"]*)"$`, suite.currentUserIdentityVerificationStatusIs)
	sc.Step(`^mi fecha de verificación no está informada$`, suite.currentUserIdentityVerificationDateIsMissing)
	sc.Step(`^busco prestadores del rubro "([^"]*)"$`, suite.filterProvidersByCategory)
	sc.Step(`^"([^"]*)" figura con identidad verificada$`, suite.providerAppearsWithVerifiedIdentity)
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
