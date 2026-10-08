package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

func registerCategoryAvailabilitySteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que el rubro "([^"]*)" se deshabilitó después (?:del registro de "([^"]*)"|de aceptar esa solicitud|de guardar esa recomendación)$`, func(name, _ string) error { return s.categoryAvailabilityFixture(name, false) })
	sc.Step(`^que el rubro "([^"]*)" se deshabilitó y luego se reactivó con el mismo identificador$`, func(name string) error {
		if err := s.categoryAvailabilityFixture(name, false); err != nil {
			return err
		}
		return s.categoryAvailabilityFixture(name, true)
	})
	sc.Step(`^filtro técnicos usando el identificador guardado del rubro "([^"]*)"$`, s.filterProvidersByCategory)
	sc.Step(`^intento enviar una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y la descripción:$`, s.sendJobRequestToProviderWithTitleAndDescription)
	sc.Step(`^intento contactar al prestador recomendado "([^"]*)" desde esa conversación con el chatbot$`, s.tryContactProviderFromChatbotConversation)
	sc.Step(`^que la recomendación de "([^"]*)" quedó guardada mientras el rubro "([^"]*)" estaba habilitado$`, s.categorySavedRecommendation)
	sc.Step(`^el sistema rechaza (?:la solicitud porque el rubro (?:del prestador|evaluado)|el registro porque el rubro) no está habilitado para nuevas operaciones$`, func() error {
		if err := s.lastResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
			return err
		}
		return s.registrationResponseShouldSay(category.ErrDisabled.Error())
	})
	sc.Step(`^el sistema no registra una cuenta de prestador para "([^"]*)"$`, func(email string) error {
		if s.userRepository.FindByEmail(email) {
			return fmt.Errorf("unexpected provider account")
		}
		return nil
	})
	sc.Step(`^el sistema registra la propuesta de servicio para la solicitud existente$`, s.systemRegistersServiceProposal)
	sc.Step(`^la solicitud de trabajo permanece aceptada$`, s.categoryAcceptedRequestUnchanged)
	sc.Step(`^consulto el listado de rubros incluyendo los deshabilitados$`, func() error {
		if err := s.categoryAuditBaseline(); err != nil {
			return err
		}
		return s.sendAdminGet("/categories", map[string][]string{"include_disabled": {"true"}}, "")
	})
	sc.Step(`^el listado incluye el rubro "([^"]*)" y no incluye el rubro "([^"]*)"$`, s.categoryPublicList)
	sc.Step(`^cada rubro listado incluye solamente su identificador y nombre visible$`, s.everyCategoryOnlyIncludesPublicFields)
	sc.Step(`^el listado incluye ambos rubros con su estado y versión$`, s.categoryAdministrativeList)
}
func (s *testSuite) categoryAvailabilityFixture(name string, enabled bool) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("category missing")
	}
	found.Enabled = enabled
	found.Version++
	_, err = s.categoryRepository.Save(*found)
	return err
}
func (s *testSuite) categorySavedRecommendation(fullName, name string) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	if found == nil || !found.Enabled {
		return fmt.Errorf("recommendation category not enabled")
	}
	providerID, err := s.providerIDByFullName(fullName)
	if err != nil {
		return err
	}
	persisted, err := s.conversationRepository.FindByID(s.scenarioContext, s.aiSourceChatbotConversationID)
	if err != nil {
		return err
	}
	chatbot, ok := persisted.(*conversation.ChatBotConversation)
	if !ok || chatbot.CurrentRecommendation == nil {
		return fmt.Errorf("saved provider recommendation missing")
	}
	for _, item := range chatbot.CurrentRecommendation.Recommendations {
		if item.ProviderID == providerID {
			return nil
		}
	}
	return fmt.Errorf("provider %s not in saved recommendation", fullName)
}
func (s *testSuite) categoryPublicList(included, excluded string) error {
	if err := s.categoryListIncludes(included); err != nil {
		return err
	}
	items, err := s.categoryListResponse()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Name == excluded {
			return fmt.Errorf("disabled category in public list")
		}
	}
	return nil
}
func (s *testSuite) categoryAdministrativeList() error {
	if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
		return err
	}
	var items []administeredCategoryResponse
	if err := json.Unmarshal(s.lastBody, &items); err != nil {
		return err
	}
	if len(items) != len(s.categoryIDsByName) {
		return fmt.Errorf("expected full category catalog")
	}
	for _, item := range items {
		found, err := s.categoryRepository.FindByID(s.scenarioContext, item.ID)
		if err != nil {
			return err
		}
		if found == nil || found.Name != item.Name || found.Enabled != item.Enabled || found.Version != item.Version {
			return fmt.Errorf("administrative category mismatch")
		}
	}
	return nil
}
func (s *testSuite) categoryAcceptedRequestUnchanged() error {
	request, err := s.jobRequestRepository.FindByConversationID(s.lastConversationID)
	if err != nil {
		return err
	}
	if request.Status != "accepted" {
		return fmt.Errorf("accepted request changed")
	}
	return nil
}
