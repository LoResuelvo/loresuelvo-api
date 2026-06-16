package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

type chatbotProviderRecommendationsResponse struct {
	RecommendedProviders []providerSummaryResponse `json:"recommended_providers"`
}

func registerChatbotProviderRecommendationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "([^"]*)" con la respuesta:$`, suite.chatbotWillConcludeDiagnosisAndRecommendCategory)
	sc.Step(`^el sistema muestra una lista vacía de prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsEmptyRecommendedProviderListInChatbotResponse)
	sc.Step(`^el sistema muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemShowsRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemDoesNotShowRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsNoRecommendedProvidersInChatbotResponse)
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
