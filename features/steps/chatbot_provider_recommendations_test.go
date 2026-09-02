package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type chatbotProviderRecommendationsResponse struct {
	ID                   int                        `json:"id"`
	Assessment           *chatbotAssessmentResponse `json:"assessment"`
	RecommendedProviders []providerSummaryResponse  `json:"recommended_providers"`
}

func registerChatbotProviderRecommendationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el domicilio del consumidor "([^"]*)" pertenece a la zona de cobertura "([^"]*)"$`, suite.consumerAddressBelongsToCoverageZone)
	sc.Step(`^que la IA de recomendación de prestadores está disponible$`, suite.providerRecommendationAIIsAvailable)
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y zona de cobertura "([^"]*)"$`, suite.thereIsRegisteredProviderWithCategoryAndCoverageZone)
	sc.Step(`^existen los siguientes prestadores elegibles de "([^"]*)" en la zona "([^"]*)":$`, suite.thereAreEligibleProviders)
	sc.Step(`^que esos prestadores tienen diferentes ratings, reseñas de consumidores y experiencia en trabajos pagados$`, suite.thoseProvidersHaveDifferentEvidence)
	sc.Step(`^que la IA recomendará, en este orden, a "([^"]*)", "([^"]*)" y "([^"]*)" con razones fundamentadas en la evidencia$`, suite.aiWillRecommendProviders)
	sc.Step(`^existe un prestador registrado con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y las zonas de cobertura "([^"]*)" y "([^"]*)"$`, suite.thereIsRegisteredProviderWithCategoryAndCoverageZones)
	sc.Step(`^existe un prestador elegible de "([^"]*)" llamado "([^"]*)"$`, suite.thereIsEligibleProvider)
	sc.Step(`^existe un prestador elegible de "([^"]*)" llamado "([^"]*)" con reseñas e informes de finalización$`, suite.thereIsEligibleProviderWithReviewsAndCompletionReports)
	sc.Step(`^que "([^"]*)" tiene trabajos pagados con ratings y reseñas escritas por consumidores$`, suite.providerHasPaidJobsWithConsumerReviews)
	sc.Step(`^que "([^"]*)" tiene informes de finalización escritos por él para trabajos pagados$`, suite.providerHasProviderAuthoredReports)
	sc.Step(`^que sus informes de finalización incluyen imágenes privadas$`, suite.providerReportsIncludePrivateImages)
	sc.Step(`^existe un prestador elegible de "([^"]*)" llamado "([^"]*)" sin trabajos pagados, ratings, reseñas ni informes de finalización$`, suite.thereIsNewEligibleProvider)
	sc.Step(`^que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "([^"]*)" con la respuesta:$`, suite.chatbotWillConcludeDiagnosisAndRecommendCategory)
	sc.Step(`^que el chatbot concluirá que se requiere un profesional del rubro "([^"]*)"$`, suite.chatbotWillConcludeProfessionalRequired)
	sc.Step(`^que una evaluación vigente requiere un profesional de "([^"]*)"$`, suite.currentAssessmentRequiresProfessional)
	sc.Step(`^que una evaluación anterior requiere un profesional de "([^"]*)"$`, suite.previousAssessmentRequiresProfessional)
	sc.Step(`^que sus recomendaciones persistidas son "([^"]*)", "([^"]*)" y "([^"]*)" en ese orden con sus razones$`, suite.persistedProviderRecommendations)
	sc.Step(`^que sus recomendaciones persistidas son "([^"]*)" y "([^"]*)" en ese orden con sus razones$`, suite.persistedProviderRecommendationsTwo)
	sc.Step(`^que el ranking vigente tiene a "([^"]*)" como recomendación$`, suite.currentRankingHasProvider)
	sc.Step(`^que el chatbot responderá sin modificar la evaluación vigente$`, suite.chatbotWillRespondWithoutChangingCurrentAssessment)
	sc.Step(`^que el chatbot generará una nueva evaluación que requiere un profesional de "([^"]*)"$`, suite.chatbotWillGenerateNewProfessionalAssessment)
	sc.Step(`^que la IA recomendará a "([^"]*)" para la nueva evaluación$`, suite.aiWillRecommendProviderForNewAssessment)
	sc.Step(`^que ningún prestador de "([^"]*)" cubre la zona "([^"]*)"$`, suite.noProviderCoversZone)
	sc.Step(`^consulto el detalle de la conversación$`, suite.requestThatConversationDetail)
	sc.Step(`^continúo la conversación con información que no modifica el diagnóstico$`, suite.continueWithUnchangedAssessment)
	sc.Step(`^continúo la conversación con nueva información sobre el problema$`, suite.continueWithNewProblemInformation)
	sc.Step(`^el sistema muestra a "([^"]*)", "([^"]*)" y "([^"]*)" en ese orden$`, suite.systemShowsProvidersInOrder)
	sc.Step(`^el detalle muestra "([^"]*)", "([^"]*)" y "([^"]*)" en el orden persistido con sus razones$`, suite.detailShowsPersistedProvidersWithReasons)
	sc.Step(`^la respuesta conserva a "([^"]*)" y "([^"]*)" en el orden persistido con sus razones$`, suite.responseShowsPersistedProvidersWithReasons)
	sc.Step(`^el sistema reemplaza el ranking vigente por uno con "([^"]*)" asociado a la nueva evaluación$`, suite.systemReplacesCurrentRanking)
	sc.Step(`^una consulta posterior devuelve a "([^"]*)" sin reutilizar el ranking anterior$`, suite.subsequentQueryReturnsProviderWithoutReusingPreviousRanking)
	sc.Step(`^la cantidad de prestadores mostrados no supera el máximo de 3 prestadores recomendados$`, suite.systemShowsAtMostThreeProviders)
	sc.Step(`^cada prestador recomendado incluye las razones seleccionadas por la IA$`, suite.eachRecommendedProviderHasAIReason)
	sc.Step(`^persiste el ranking vigente con los candidatos considerados, la selección ordenada y sus razones$`, suite.systemPersistsCurrentProviderRanking)
	sc.Step(`^la IA de recomendación recibe como único candidato al prestador "([^"]*)"$`, suite.aiReceivesOnlyProvider)
	sc.Step(`^la IA de recomendación no recibe a "([^"]*)" ni a "([^"]*)"$`, suite.aiDoesNotReceiveProviders)
	sc.Step(`^la evidencia de "([^"]*)" enviada a la IA incluye el promedio, cantidad y distribución de ratings$`, suite.evidenceIncludesRatingStats)
	sc.Step(`^incluye la cantidad y recencia de sus trabajos pagados$`, suite.evidenceIncludesPaidWorkRecency)
	sc.Step(`^incluye sus reseñas identificadas como opiniones escritas por consumidores$`, suite.evidenceIncludesConsumerReviews)
	sc.Step(`^incluye sus informes de finalización identificados como evidencia autoescrita por el prestador$`, suite.evidenceIncludesProviderReports)
	sc.Step(`^la IA de recomendación recibe a "([^"]*)" como candidato$`, suite.aiReceivesProvider)
	sc.Step(`^la IA de recomendación identifica al candidato mediante una referencia opaca$`, suite.aiIdentifiesCandidateByOpaqueReference)
	sc.Step(`^la IA de recomendación no recibe datos personales, fotos ni información sensible$`, suite.aiDoesNotReceivePersonalDataPhotosOrSensitiveInformation)
	sc.Step(`^la IA de recomendación no vuelve a ser invocada$`, suite.providerRankingWasNotInvokedAgain)
	sc.Step(`^la IA de recomendación no es invocada$`, suite.providerRankingWasNotInvoked)
	sc.Step(`^su evidencia histórica se representa como vacía$`, suite.providerEvidenceIsEmpty)
	sc.Step(`^el sistema muestra una lista vacía de prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsEmptyRecommendedProviderListInChatbotResponse)
	sc.Step(`^el sistema persiste una lista vacía de prestadores recomendados para la evaluación$`, suite.systemPersistsEmptyProviderRecommendation)
	sc.Step(`^una consulta posterior devuelve la misma lista vacía$`, suite.subsequentQueryReturnsEmptyProviderRecommendation)
	sc.Step(`^el sistema muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemShowsRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra al prestador recomendado "([^"]*)" en la respuesta del chatbot$`, suite.systemDoesNotShowRecommendedProviderInChatbotResponse)
	sc.Step(`^el sistema no muestra prestadores recomendados en la respuesta del chatbot$`, suite.systemShowsNoRecommendedProvidersInChatbotResponse)
}

func (suite *testSuite) providerRecommendationAIIsAvailable() error {
	suite.chatbot.SetProviderRankingError(nil)
	return nil
}

func (suite *testSuite) chatbotWillConcludeProfessionalRequired(categoryName string) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}
	suite.lastChatbotRecommendedCategoryName = categoryName
	suite.chatbot.SetConcludedDiagnosisResponse(
		"Diagnóstico profesional",
		"Se requiere la evaluación de un prestador para resolver el problema informado.",
		categoryName,
	)
	return nil
}

func (suite *testSuite) thereAreEligibleProviders(categoryName, coverageZoneName string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("expected at least one eligible provider in table")
	}
	zone, err := suite.coverageZoneRepository.FindByMarketCodeAndName(context.Background(), defaultProviderCoverageMarketCode, coverageZoneName)
	if err != nil {
		return fmt.Errorf("could not find coverage zone %q: %w", coverageZoneName, err)
	}
	if zone == nil {
		return fmt.Errorf("coverage zone repository returned no zone for %q", coverageZoneName)
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) < 3 {
			return fmt.Errorf("expected provider table columns correo, nombre and apellido")
		}
		if err := suite.registerProviderFixture(row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, categoryName, []int{zone.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) thoseProvidersHaveDifferentEvidence() error {
	providerNames := make([]string, 0, len(suite.providerEmailsByFullName))
	for name := range suite.providerEmailsByFullName {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)
	ratings := []int{5, 4, 3, 2}
	for index, name := range providerNames {
		email := suite.providerEmailsByFullName[name]
		imageName := "ranking-" + strings.NewReplacer("@", "-", ".", "-").Replace(email) + ".jpg"
		if err := suite.createWorkOrderForListingStatus("paid", email, "ana@example.com", imageName); err != nil {
			return fmt.Errorf("creating paid evidence for %q: %w", name, err)
		}
		suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
		if err := suite.requestReview(ratings[index%len(ratings)], "Opinión del consumidor sobre el trabajo realizado."); err != nil {
			return fmt.Errorf("creating consumer review for %q: %w", name, err)
		}
		if suite.lastStatus != http.StatusCreated {
			return fmt.Errorf("creating consumer review for %q returned status %d with body %s", name, suite.lastStatus, string(suite.lastBody))
		}
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	return nil
}

func (suite *testSuite) aiWillRecommendProviders(first, second, third string) error {
	providerIDs := make([]int, 0, 3)
	for _, name := range []string{first, second, third} {
		providerID, err := suite.providerIDForFullName(name)
		if err != nil {
			return err
		}
		providerIDs = append(providerIDs, providerID)
	}
	suite.chatbot.SetProviderRankingByProviderIDs(providerIDs...)
	return nil
}

func (suite *testSuite) currentAssessmentRequiresProfessional(categoryName string) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}
	suite.lastChatbotRecommendedCategoryName = categoryName
	return nil
}

func (suite *testSuite) previousAssessmentRequiresProfessional(categoryName string) error {
	return suite.currentAssessmentRequiresProfessional(categoryName)
}

func (suite *testSuite) persistedProviderRecommendations(first, second, third string) error {
	return suite.prepareChatbotWithProviderRanking([]string{first, second, third})
}

func (suite *testSuite) persistedProviderRecommendationsTwo(first, second string) error {
	return suite.prepareChatbotWithProviderRanking([]string{first, second})
}

func (suite *testSuite) currentRankingHasProvider(providerName string) error {
	if err := suite.prepareChatbotWithProviderRanking([]string{providerName}); err != nil {
		return err
	}

	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding initial chatbot conversation for provider ranking replacement: %w", err)
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentAssessment == nil || chatbotConversation.CurrentRecommendation == nil {
		return fmt.Errorf("expected initial chatbot conversation to have a current provider ranking")
	}
	suite.previousAssessmentID = chatbotConversation.CurrentAssessment.ID
	return nil
}

func (suite *testSuite) chatbotWillRespondWithoutChangingCurrentAssessment() error {
	suite.chatbot.SetUnchangedResponse(
		"Información recibida",
		"La información no modifica el diagnóstico preliminar vigente.",
	)
	return nil
}

func (suite *testSuite) chatbotWillGenerateNewProfessionalAssessment(categoryName string) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}
	suite.lastChatbotRecommendedCategoryName = categoryName
	suite.chatbot.SetConcludedDiagnosisResponse(
		"Nuevo diagnóstico profesional",
		"La nueva información modifica el diagnóstico preliminar y requiere revisar la recomendación.",
		categoryName,
	)
	return nil
}

func (suite *testSuite) aiWillRecommendProviderForNewAssessment(providerName string) error {
	if err := suite.ensureProviderRegisteredForRecommendation(providerName); err != nil {
		return err
	}
	providerID, err := suite.providerIDForFullName(providerName)
	if err != nil {
		return err
	}
	suite.chatbot.SetProviderRankingByProviderIDs(providerID)
	return nil
}

func (suite *testSuite) noProviderCoversZone(categoryName, coverageZoneName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}
	zoneID, err := suite.ensureProviderCoverageZoneByName(coverageZoneName)
	if err != nil {
		return err
	}
	providers, err := suite.userRepository.FindProvidersByCategoryAndCoverageZoneID(context.Background(), categoryID, zoneID)
	if err != nil {
		return fmt.Errorf("finding providers for empty recommendation fixture: %w", err)
	}
	if len(providers) != 0 {
		return fmt.Errorf("expected no provider for category %q in coverage zone %q, got %d", categoryName, coverageZoneName, len(providers))
	}
	suite.lastChatbotRecommendedCategoryName = categoryName
	suite.providerRankingRequestCountBeforeAction = suite.chatbot.ProviderRankingRequestCount()
	return nil
}

func (suite *testSuite) prepareChatbotWithProviderRanking(providerNames []string) error {
	if suite.lastChatbotRecommendedCategoryName == "" {
		return fmt.Errorf("expected a chatbot professional assessment category before preparing persisted recommendations")
	}
	providerIDs := make([]int, 0, len(providerNames))
	for _, providerName := range providerNames {
		if err := suite.ensureProviderRegisteredForRecommendation(providerName); err != nil {
			return err
		}
		providerID, err := suite.providerIDForFullName(providerName)
		if err != nil {
			return err
		}
		providerIDs = append(providerIDs, providerID)
	}

	suite.chatbot.SetProviderRankingByProviderIDs(providerIDs...)
	suite.chatbot.SetConcludedDiagnosisResponse(
		"Diagnóstico persistido",
		"La evaluación vigente requiere la intervención de un prestador.",
		suite.lastChatbotRecommendedCategoryName,
	)
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: "Necesito resolver el problema informado."}); err != nil {
		return err
	}
	if err := suite.rememberCreatedChatbotConversation(); err != nil {
		return err
	}
	suite.providerRankingRequestCountBeforeAction = suite.chatbot.ProviderRankingRequestCount()
	return nil
}

func (suite *testSuite) ensureProviderRegisteredForRecommendation(providerName string) error {
	if _, err := suite.providerEmailForFullName(providerName); err == nil {
		return nil
	}
	return suite.thereIsEligibleProvider(suite.lastChatbotRecommendedCategoryName, providerName)
}

func (suite *testSuite) thereIsEligibleProvider(categoryName, providerName string) error {
	name, surname := splitProviderName(providerName)
	email := suite.generatedProviderEmail(providerName)
	zoneID, err := suite.ensureProviderCoverageZoneByName("Comuna 6")
	if err != nil {
		return err
	}
	return suite.registerProviderFixture(email, name, surname, categoryName, []int{zoneID})
}

func (suite *testSuite) thereIsEligibleProviderWithReviewsAndCompletionReports(categoryName, providerName string) error {
	if err := suite.thereIsEligibleProvider(categoryName, providerName); err != nil {
		return err
	}
	return suite.providerHasPaidJobsWithConsumerReviews(providerName)
}

func (suite *testSuite) thereIsNewEligibleProvider(categoryName, providerName string) error {
	return suite.thereIsEligibleProvider(categoryName, providerName)
}

func (suite *testSuite) providerHasPaidJobsWithConsumerReviews(providerName string) error {
	email, err := suite.providerEmailForFullName(providerName)
	if err != nil {
		return err
	}
	if err := suite.createWorkOrderForListingStatus("paid", email, "ana@example.com", "ranking-history.jpg"); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	if err := suite.requestReview(5, "La reparación resolvió la pérdida."); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating consumer review returned status %d with body %s", suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerHasProviderAuthoredReports(providerName string) error {
	providerID, err := suite.providerIDForFullName(providerName)
	if err != nil {
		return err
	}
	history, err := suite.workOrderRepository.FindPaidWorkHistoryByProviderID(context.Background(), providerID)
	if err != nil {
		return fmt.Errorf("finding paid work history for %q: %w", providerName, err)
	}
	for _, work := range history {
		if work.CompletionReport != nil {
			return nil
		}
	}
	return fmt.Errorf("expected a provider-authored completion report for %q", providerName)
}

func (suite *testSuite) providerReportsIncludePrivateImages() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	report := order.CompletionReport()
	if report == nil || len(report.ImageFileIDs()) == 0 {
		return fmt.Errorf("expected the provider completion report to include private images")
	}

	for _, fileID := range report.ImageFileIDs() {
		file, err := suite.fileRepository.FindByID(context.Background(), fileID)
		if err != nil {
			return fmt.Errorf("finding completion image %q: %w", fileID, err)
		}
		if file == nil || !file.IsConfirmed() || file.IsPublic() || !file.HasPurpose(filedomain.PurposeWorkOrderCompletionImage) {
			return fmt.Errorf("expected completion image %q to be confirmed, private and scoped to completion reports", fileID)
		}
	}

	return nil
}

func (suite *testSuite) providerIDForFullName(providerName string) (int, error) {
	email, err := suite.providerEmailForFullName(providerName)
	if err != nil {
		return 0, err
	}
	providerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return 0, fmt.Errorf("could not find provider %q: %w", providerName, err)
	}
	return providerID, nil
}

func (suite *testSuite) providerEmailForFullName(providerName string) (string, error) {
	if email := suite.providerEmailsByFullName[strings.TrimSpace(providerName)]; email != "" {
		return email, nil
	}
	return "", fmt.Errorf("provider %q is not registered", providerName)
}

func (suite *testSuite) generatedProviderEmail(providerName string) string {
	emailName := strings.ToLower(strings.TrimSpace(providerName))
	emailName = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", " ", ".").Replace(emailName)
	return emailName + "@example.com"
}

func splitProviderName(providerName string) (string, string) {
	parts := strings.Fields(providerName)
	if len(parts) < 2 {
		return providerName, "Proveedor"
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func (suite *testSuite) ensureProviderCoverageZoneByName(name string) (int, error) {
	zone, err := suite.coverageZoneRepository.FindByMarketCodeAndName(context.Background(), defaultProviderCoverageMarketCode, name)
	if err != nil {
		return 0, fmt.Errorf("could not find coverage zone %q: %w", name, err)
	}
	if zone == nil {
		return 0, fmt.Errorf("coverage zone repository returned no zone for %q", name)
	}
	return zone.ID, nil
}

func (suite *testSuite) systemShowsProvidersInOrder(first, second, third string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	expected := []string{first, second, third}
	if len(providers) < len(expected) {
		return fmt.Errorf("expected providers %v in response order, got %v", expected, providers)
	}
	for index, expectedName := range expected {
		if providerFullName(providers[index]) != expectedName {
			return fmt.Errorf("expected provider %q at position %d, got %q with body %s", expectedName, index+1, providerFullName(providers[index]), string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) detailShowsPersistedProvidersWithReasons(first, second, third string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotConversationDetail()
	if err != nil {
		return err
	}
	return validateProviderOrderAndReasons(providers, []string{first, second, third}, suite.lastBody)
}

func (suite *testSuite) responseShowsPersistedProvidersWithReasons(first, second string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	return validateProviderOrderAndReasons(providers, []string{first, second}, suite.lastBody)
}

func validateProviderOrderAndReasons(providers []providerSummaryResponse, expected []string, body []byte) error {
	if len(providers) != len(expected) {
		return fmt.Errorf("expected providers %v in persisted order, got %v with body %s", expected, providers, string(body))
	}
	for index, expectedName := range expected {
		if providerFullName(providers[index]) != expectedName {
			return fmt.Errorf("expected provider %q at position %d, got %q with body %s", expectedName, index+1, providerFullName(providers[index]), string(body))
		}
		if strings.TrimSpace(providers[index].RecommendationReason) == "" {
			return fmt.Errorf("expected a persisted recommendation reason for provider %q, got body %s", expectedName, string(body))
		}
	}
	return nil
}

func (suite *testSuite) systemReplacesCurrentRanking(providerName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	providerID, err := suite.providerIDForFullName(providerName)
	if err != nil {
		return err
	}
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding chatbot conversation after ranking replacement: %w", err)
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentAssessment == nil || chatbotConversation.CurrentRecommendation == nil {
		return fmt.Errorf("expected chatbot conversation to persist the replacement ranking")
	}
	if chatbotConversation.CurrentAssessment.ID <= suite.previousAssessmentID {
		return fmt.Errorf("expected a new assessment after %d, got %d", suite.previousAssessmentID, chatbotConversation.CurrentAssessment.ID)
	}
	currentRecommendation := chatbotConversation.CurrentRecommendation
	if currentRecommendation.AssessmentID != chatbotConversation.CurrentAssessment.ID {
		return fmt.Errorf("expected replacement ranking assessment %d, got %d", chatbotConversation.CurrentAssessment.ID, currentRecommendation.AssessmentID)
	}
	if len(currentRecommendation.CandidateProviderIDs) == 0 || len(currentRecommendation.Recommendations) != 1 {
		return fmt.Errorf("expected replacement ranking to retain eligible candidates and contain one recommendation for provider %d, got %+v", providerID, currentRecommendation)
	}
	item := currentRecommendation.Recommendations[0]
	if item.ProviderID != providerID || item.Position != 1 || strings.TrimSpace(item.Reason) == "" {
		return fmt.Errorf("expected replacement recommendation for provider %d, got %+v", providerID, item)
	}
	if suite.chatbot.ProviderRankingRequestCount() != suite.providerRankingRequestCountBeforeAction+1 {
		return fmt.Errorf("expected exactly one new provider ranking invocation, got %d after baseline %d", suite.chatbot.ProviderRankingRequestCount(), suite.providerRankingRequestCountBeforeAction)
	}
	return nil
}

func (suite *testSuite) subsequentQueryReturnsProviderWithoutReusingPreviousRanking(providerName string) error {
	providerRankingRequestCount := suite.chatbot.ProviderRankingRequestCount()
	if err := suite.requestThatConversationDetail(); err != nil {
		return err
	}
	providers, err := suite.recommendedProvidersFromLastChatbotConversationDetail()
	if err != nil {
		return err
	}
	if len(providers) != 1 || providerFullName(providers[0]) != providerName || strings.TrimSpace(providers[0].RecommendationReason) == "" {
		return fmt.Errorf("expected subsequent detail to return only %q with its reason, got %v and body %s", providerName, providers, string(suite.lastBody))
	}
	if suite.chatbot.ProviderRankingRequestCount() != providerRankingRequestCount {
		return fmt.Errorf("expected subsequent detail not to invoke provider ranking again")
	}
	return nil
}

func (suite *testSuite) systemShowsAtMostThreeProviders() error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	if len(providers) > 3 {
		return fmt.Errorf("expected no more than 3 providers, got %d", len(providers))
	}
	return nil
}

func (suite *testSuite) eachRecommendedProviderHasAIReason() error {
	providers, err := suite.recommendedProvidersFromLastChatbotResponse()
	if err != nil {
		return err
	}
	for _, foundProvider := range providers {
		if strings.TrimSpace(foundProvider.RecommendationReason) == "" {
			return fmt.Errorf("expected AI recommendation reason for provider %q, got body %s", providerFullName(foundProvider), string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) systemPersistsCurrentProviderRanking() error {
	var response chatbotProviderRecommendationsResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding chatbot recommendation response: %w", err)
	}
	if response.ID <= 0 {
		return fmt.Errorf("expected created chatbot conversation id, got %d", response.ID)
	}
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), response.ID)
	if err != nil {
		return fmt.Errorf("finding persisted chatbot conversation: %w", err)
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentRecommendation == nil {
		return fmt.Errorf("expected current provider recommendation to be persisted")
	}
	currentRecommendation := chatbotConversation.CurrentRecommendation
	if len(currentRecommendation.CandidateProviderIDs) != 4 || len(currentRecommendation.Recommendations) != len(response.RecommendedProviders) {
		return fmt.Errorf("unexpected persisted ranking shape: candidates=%v recommendations=%v", currentRecommendation.CandidateProviderIDs, currentRecommendation.Recommendations)
	}
	for index, item := range currentRecommendation.Recommendations {
		if index >= len(response.RecommendedProviders) || item.ProviderID != response.RecommendedProviders[index].ID || item.Position != index+1 || strings.TrimSpace(item.Reason) == "" {
			return fmt.Errorf("persisted ranking does not match response at position %d: %+v", index+1, item)
		}
	}
	return nil
}

func (suite *testSuite) aiReceivesOnlyProvider(providerName string) error {
	request := suite.chatbot.LastProviderRankingRequest()
	providerID, err := suite.providerIDForFullName(providerName)
	if err != nil {
		return err
	}
	if len(request.Candidates) != 1 || request.Candidates[0].ProviderID != providerID {
		return fmt.Errorf("expected only provider %q (%d) as AI candidate, got %+v", providerName, providerID, request.Candidates)
	}
	return nil
}

func (suite *testSuite) aiDoesNotReceiveProviders(first, second string) error {
	request := suite.chatbot.LastProviderRankingRequest()
	for _, providerName := range []string{first, second} {
		providerID, err := suite.providerIDForFullName(providerName)
		if err != nil {
			return err
		}
		for _, candidate := range request.Candidates {
			if candidate.ProviderID == providerID {
				return fmt.Errorf("provider %q (%d) must not be sent to AI", providerName, providerID)
			}
		}
	}
	return nil
}

func (suite *testSuite) evidenceForProvider(providerName string) (providerEvidence, error) {
	providerID, err := suite.providerIDForFullName(providerName)
	if err != nil {
		return providerEvidence{}, err
	}
	request := suite.chatbot.LastProviderRankingRequest()
	for _, candidate := range request.Candidates {
		if candidate.ProviderID == providerID {
			return providerEvidence{average: candidate.Evidence.RatingAverage, count: candidate.Evidence.RatingCount, distribution: candidate.Evidence.RatingDistribution, paidCount: candidate.Evidence.PaidWorkCount, mostRecent: candidate.Evidence.MostRecentPaidWork, history: candidate.Evidence.WorkHistory}, nil
		}
	}
	return providerEvidence{}, fmt.Errorf("provider %q (%d) was not sent to AI", providerName, providerID)
}

type providerEvidence struct {
	average      float64
	count        int
	distribution provider.RatingDistribution
	paidCount    int
	mostRecent   time.Time
	history      []providerreadmodel.WorkOrder
}

func (suite *testSuite) evidenceIncludesRatingStats(providerName string) error {
	evidence, err := suite.evidenceForProvider(providerName)
	if err != nil {
		return err
	}
	if evidence.count <= 0 || evidence.average <= 0 {
		return fmt.Errorf("expected non-empty rating average and count, got %+v", evidence)
	}
	distributionTotal := 0
	for _, count := range evidence.distribution {
		distributionTotal += count
	}
	if distributionTotal != evidence.count {
		return fmt.Errorf("expected rating distribution total %d to equal count %d", distributionTotal, evidence.count)
	}
	return nil
}

func (suite *testSuite) evidenceIncludesPaidWorkRecency() error {
	providerName, err := suite.onlyProviderName()
	if err != nil {
		return err
	}
	evidence, err := suite.evidenceForProvider(providerName)
	if err != nil {
		return err
	}
	if evidence.paidCount <= 0 || evidence.mostRecent.IsZero() || len(evidence.history) != evidence.paidCount {
		return fmt.Errorf("expected paid work count and recency, got %+v", evidence)
	}
	return nil
}

func (suite *testSuite) evidenceIncludesConsumerReviews() error {
	providerName, err := suite.onlyProviderName()
	if err != nil {
		return err
	}
	evidence, err := suite.evidenceForProvider(providerName)
	if err != nil {
		return err
	}
	for _, work := range evidence.history {
		if work.Review != nil && work.Review.Rating > 0 {
			return nil
		}
	}
	return fmt.Errorf("expected consumer review evidence for %q, got %+v", providerName, evidence.history)
}

func (suite *testSuite) evidenceIncludesProviderReports() error {
	providerName, err := suite.onlyProviderName()
	if err != nil {
		return err
	}
	evidence, err := suite.evidenceForProvider(providerName)
	if err != nil {
		return err
	}
	for _, work := range evidence.history {
		if work.CompletionReport != nil && strings.TrimSpace(work.CompletionReport.Description) != "" {
			return nil
		}
	}
	return fmt.Errorf("expected provider-authored completion report evidence for %q, got %+v", providerName, evidence.history)
}

func (suite *testSuite) aiReceivesProvider(providerName string) error {
	_, err := suite.evidenceForProvider(providerName)
	return err
}

func (suite *testSuite) aiIdentifiesCandidateByOpaqueReference() error {
	request := suite.chatbot.LastProviderRankingRequest()
	if len(request.Candidates) == 0 {
		return fmt.Errorf("expected at least one provider ranking candidate")
	}
	seenReferences := make(map[string]struct{}, len(request.Candidates))
	for _, candidate := range request.Candidates {
		reference := strings.TrimSpace(candidate.Reference)
		if !strings.HasPrefix(reference, "candidate-") {
			return fmt.Errorf("expected opaque candidate reference, got %q", reference)
		}
		if _, err := uuid.Parse(strings.TrimPrefix(reference, "candidate-")); err != nil {
			return fmt.Errorf("expected candidate reference %q to contain an opaque UUID: %w", reference, err)
		}
		if _, duplicate := seenReferences[reference]; duplicate {
			return fmt.Errorf("candidate reference %q was repeated", reference)
		}
		seenReferences[reference] = struct{}{}
	}
	return nil
}

func (suite *testSuite) aiDoesNotReceivePersonalDataPhotosOrSensitiveInformation() error {
	payload, err := suite.chatbot.LastProviderRankingAIInputJSON()
	if err != nil {
		return fmt.Errorf("encoding provider ranking AI input: %w", err)
	}
	serializedPayload := strings.ToLower(string(payload))
	for _, providerName := range suite.providerNamesForRankingPrivacyCheck() {
		if strings.Contains(serializedPayload, strings.ToLower(providerName)) {
			return fmt.Errorf("provider identity %q was included in the AI input: %s", providerName, string(payload))
		}
	}
	for _, providerEmail := range suite.providerEmailsByFullName {
		if strings.Contains(serializedPayload, strings.ToLower(providerEmail)) {
			return fmt.Errorf("provider email %q was included in the AI input: %s", providerEmail, string(payload))
		}
	}
	for _, forbiddenField := range []string{
		"provider_id",
		"original_name",
		"file_id",
		"images",
		"photo",
		"profile_photo",
		"upload",
		"url",
	} {
		if strings.Contains(serializedPayload, forbiddenField) {
			return fmt.Errorf("private or identifying field %q was included in the AI input: %s", forbiddenField, string(payload))
		}
	}
	if strings.Contains(serializedPayload, "ranking-history.jpg") {
		return fmt.Errorf("private completion image name was included in the AI input: %s", string(payload))
	}
	return nil
}

func (suite *testSuite) providerNamesForRankingPrivacyCheck() []string {
	names := make([]string, 0, len(suite.providerEmailsByFullName)*2)
	for fullName := range suite.providerEmailsByFullName {
		name, surname := splitProviderName(fullName)
		names = append(names, name, surname, fullName)
	}
	return names
}

func (suite *testSuite) providerRankingWasNotInvokedAgain() error {
	return suite.providerRankingInvocationCountEqualsBaseline()
}

func (suite *testSuite) providerRankingWasNotInvoked() error {
	return suite.providerRankingInvocationCountEqualsBaseline()
}

func (suite *testSuite) providerRankingInvocationCountEqualsBaseline() error {
	actual := suite.chatbot.ProviderRankingRequestCount()
	if actual != suite.providerRankingRequestCountBeforeAction {
		return fmt.Errorf("expected provider ranking invocation count to remain %d, got %d", suite.providerRankingRequestCountBeforeAction, actual)
	}
	return nil
}

func (suite *testSuite) onlyProviderName() (string, error) {
	if len(suite.providerEmailsByFullName) != 1 {
		return "", fmt.Errorf("expected exactly one registered provider, got %d", len(suite.providerEmailsByFullName))
	}
	for name := range suite.providerEmailsByFullName {
		return name, nil
	}
	return "", fmt.Errorf("expected one registered provider")
}

func (suite *testSuite) providerEvidenceIsEmpty() error {
	request := suite.chatbot.LastProviderRankingRequest()
	if len(request.Candidates) != 1 || !request.Candidates[0].Evidence.IsEmpty() {
		return fmt.Errorf("expected empty evidence for sole candidate, got %+v", request.Candidates)
	}
	return nil
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
		suite.providerEmailsByFullName[strings.TrimSpace(name+" "+surname)] = email
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

func (suite *testSuite) continueWithUnchangedAssessment() error {
	return suite.sendNewMessageToExistingChatbotConversation(&godog.DocString{
		Content: "La pérdida sigue igual, no hay información que cambie el diagnóstico.",
	})
}

func (suite *testSuite) continueWithNewProblemInformation() error {
	return suite.sendNewMessageToExistingChatbotConversation(&godog.DocString{
		Content: "Ahora también pierde agua cuando la conexión está cerrada.",
	})
}

func (suite *testSuite) systemPersistsEmptyProviderRecommendation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	var response chatbotProviderRecommendationsResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding empty chatbot recommendation response: %w", err)
	}
	if response.ID <= 0 || len(response.RecommendedProviders) != 0 {
		return fmt.Errorf("expected an empty recommendation response, got %+v with body %s", response, string(suite.lastBody))
	}
	suite.lastConversationID = response.ID

	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), response.ID)
	if err != nil {
		return fmt.Errorf("finding chatbot conversation with empty recommendation: %w", err)
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentAssessment == nil || chatbotConversation.CurrentRecommendation == nil {
		return fmt.Errorf("expected the empty provider recommendation to be persisted with the current assessment")
	}
	currentRecommendation := chatbotConversation.CurrentRecommendation
	if currentRecommendation.AssessmentID != chatbotConversation.CurrentAssessment.ID || len(currentRecommendation.CandidateProviderIDs) != 0 || len(currentRecommendation.Recommendations) != 0 {
		return fmt.Errorf("expected empty recommendation associated with assessment %d, got %+v", chatbotConversation.CurrentAssessment.ID, currentRecommendation)
	}
	suite.previousAssessmentID = chatbotConversation.CurrentAssessment.ID
	return nil
}

func (suite *testSuite) subsequentQueryReturnsEmptyProviderRecommendation() error {
	if err := suite.requestThatConversationDetail(); err != nil {
		return err
	}
	providers, err := suite.recommendedProvidersFromLastChatbotConversationDetail()
	if err != nil {
		return err
	}
	if len(providers) != 0 {
		return fmt.Errorf("expected subsequent chatbot detail to return the same empty recommendation, got %+v", providers)
	}
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding chatbot conversation after empty recommendation query: %w", err)
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentRecommendation == nil || chatbotConversation.CurrentRecommendation.AssessmentID != suite.previousAssessmentID {
		return fmt.Errorf("expected the empty recommendation to remain associated with assessment %d", suite.previousAssessmentID)
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
