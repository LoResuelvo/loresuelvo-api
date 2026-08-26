package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type chatbotProviderRecommendationsResponse struct {
	Assessment           *chatbotAssessmentResponse `json:"assessment"`
	RecommendedProviders []providerSummaryResponse  `json:"recommended_providers"`
}

func registerChatbotProviderRecommendationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el domicilio del consumidor "([^"]*)" pertenece a la zona de cobertura "([^"]*)"$`, suite.consumerAddressBelongsToCoverageZone)
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y zona de cobertura "([^"]*)"$`, suite.thereIsRegisteredProviderWithCategoryAndCoverageZone)
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y las zonas de cobertura "([^"]*)" y "([^"]*)"$`, suite.thereIsRegisteredProviderWithCategoryAndCoverageZones)
	sc.Step(`^que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "([^"]*)" con la respuesta:$`, suite.chatbotWillConcludeDiagnosisAndRecommendCategory)
	sc.Step(`^el sistema muestra una lista vacía de prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsEmptyRecommendedProviderListInChatbotResponse)
	sc.Step(`^el sistema muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemShowsRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemDoesNotShowRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsNoRecommendedProvidersInChatbotResponse)
}

func (suite *testSuite) consumerAddressBelongsToCoverageZone(consumerEmail, coverageZoneName string) error {
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return fmt.Errorf("could not find consumer %q: %w", consumerEmail, err)
	}

	zone, err := suite.coverageZoneRepository.FindByMarketCodeAndName(context.Background(), defaultProviderCoverageMarketCode, coverageZoneName)
	if err != nil {
		return fmt.Errorf("could not find coverage zone %q: %w", coverageZoneName, err)
	}
	if zone == nil {
		return fmt.Errorf("coverage zone repository returned no zone for %q", coverageZoneName)
	}
	if err := suite.userRepository.UpdateConsumerCoverageZone(context.Background(), consumerID, zone.ID); err != nil {
		return fmt.Errorf("could not associate consumer %q with coverage zone %q: %w", consumerEmail, coverageZoneName, err)
	}

	return nil
}

func (suite *testSuite) thereIsRegisteredProviderWithCategoryAndCoverageZone(email, name, surname, categoryName, coverageZoneName string) error {
	zone, err := suite.coverageZoneRepository.FindByMarketCodeAndName(context.Background(), defaultProviderCoverageMarketCode, coverageZoneName)
	if err != nil {
		return fmt.Errorf("could not find coverage zone %q: %w", coverageZoneName, err)
	}
	if zone == nil {
		return fmt.Errorf("coverage zone repository returned no zone for %q", coverageZoneName)
	}

	return suite.registerProviderFixture(email, name, surname, categoryName, []int{zone.ID})
}

func (suite *testSuite) thereIsRegisteredProviderWithCategoryAndCoverageZones(email, name, surname, categoryName, firstZoneName, secondZoneName string) error {
	zoneIDs := make([]int, 0, 2)
	for _, zoneName := range []string{firstZoneName, secondZoneName} {
		zone, err := suite.coverageZoneRepository.FindByMarketCodeAndName(context.Background(), defaultProviderCoverageMarketCode, zoneName)
		if err != nil {
			return fmt.Errorf("could not find coverage zone %q: %w", zoneName, err)
		}
		if zone == nil {
			return fmt.Errorf("coverage zone repository returned no zone for %q", zoneName)
		}
		zoneIDs = append(zoneIDs, zone.ID)
	}

	return suite.registerProviderFixture(email, name, surname, categoryName, zoneIDs)
}

func (suite *testSuite) registerProviderFixture(email, name, surname, categoryName string, coverageZoneIDs []int) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	resp, err := suite.postProviderRegistrationWithAuth0ID(auth0IDForProviderEmail(email), providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CoverageZoneIDs:        coverageZoneIDs,
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		suite.rememberParticipantFullName(name, surname, participantRoleProvider)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare registered provider: status %d, body %s", resp.StatusCode, string(body))
}

func (suite *testSuite) chatbotWillConcludeDiagnosisAndRecommendCategory(categoryName string, message *godog.DocString) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}

	suite.lastChatbotRecommendedCategoryName = categoryName
	suite.chatbot.SetConcludedDiagnosisResponse("Diagnóstico concluido", normalizeDocString(message), categoryName)
	return nil
}

func (suite *testSuite) systemShowsEmptyRecommendedProviderListInChatbotResponse() error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	if len(providers) != 0 {
		return fmt.Errorf("expected empty recommended provider list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsRecommendedProviderInChatbotResponse(fullName string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	if providerListIncludesFullName(providers, fullName) {
		return nil
	}

	return fmt.Errorf("expected chatbot response recommended providers to include %q, got body %s", fullName, string(suite.lastBody))
}

func (suite *testSuite) systemDoesNotShowRecommendedProviderInChatbotResponse(fullName string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	if providerListIncludesFullName(providers, fullName) {
		return fmt.Errorf("expected chatbot response recommended providers not to include %q, got body %s", fullName, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsNoRecommendedProvidersInChatbotResponse() error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		if fieldIsMissingInChatbotResponse("recommended_providers", suite.lastBody) {
			return nil
		}
		return err
	}
	if len(providers) != 0 {
		return fmt.Errorf("expected chatbot response not to include recommended providers, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) recommendedProvidersFromLastChatbotResponse() ([]providerSummaryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return nil, err
	}

	var response chatbotProviderRecommendationsResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return nil, fmt.Errorf("response is not valid JSON chatbot response with recommended_providers: %w", err)
	}

	if fieldIsMissingInChatbotResponse("recommended_providers", suite.lastBody) {
		return nil, fmt.Errorf("expected chatbot response to include recommended_providers, got body %s", string(suite.lastBody))
	}
	if suite.lastChatbotRecommendedCategoryName != "" {
		if response.Assessment == nil || response.Assessment.Outcome != "professional_required" {
			return nil, fmt.Errorf("expected chatbot response to require a professional, got body %s", string(suite.lastBody))
		}
		if response.Assessment.ProblemCategory.ID == 0 || !sameNormalizedName(response.Assessment.ProblemCategory.Name, suite.lastChatbotRecommendedCategoryName) {
			return nil, fmt.Errorf("expected chatbot response category %q with a valid id, got %+v with body %s", suite.lastChatbotRecommendedCategoryName, response.Assessment.ProblemCategory, string(suite.lastBody))
		}
	}

	for _, provider := range response.RecommendedProviders {
		if provider.ID == 0 {
			return nil, fmt.Errorf("expected each recommended provider to include id, got body %s", string(suite.lastBody))
		}
		if provider.Name == "" || provider.Surname == "" || provider.CategoryName == "" {
			return nil, fmt.Errorf("expected each recommended provider to include name, surname and category_name, got body %s", string(suite.lastBody))
		}
		if suite.lastChatbotRecommendedCategoryName != "" && !sameNormalizedName(provider.CategoryName, suite.lastChatbotRecommendedCategoryName) {
			return nil, fmt.Errorf("expected recommended provider category %q to match chatbot recommended category %q, got body %s", provider.CategoryName, suite.lastChatbotRecommendedCategoryName, string(suite.lastBody))
		}
	}

	return response.RecommendedProviders, nil
}

func fieldIsMissingInChatbotResponse(fieldName string, body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	_, ok := raw[fieldName]
	return !ok
}
