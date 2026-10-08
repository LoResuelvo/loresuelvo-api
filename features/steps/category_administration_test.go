package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type categoryAdministrationState struct {
	original   map[int]category.Category
	selected   int
	auditMark  int64
	operations map[string]*operationDetailAuditSnapshot
	order      *workorder.WorkOrder
}

type administeredCategoryResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Version int    `json:"version"`
}

func registerCategoryAdministrationSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que existe el rubro (habilitado|deshabilitado) "([^"]*)"(?: sin órdenes en curso| en la versión 1)?$`, s.categoryStateFixture)
	sc.Step(`^que existe el rubro "([^"]*)" en la versión 1$`, s.thereIsCategoryNamed)
	sc.Step(`^que el mismo administrador mantiene abierta otra pestaña con la versión 1 del rubro$`, func() error {
		for _, id := range s.categoryIDsByName {
			if err := s.rememberCategoryOriginal(id); err != nil {
				return err
			}
			if s.categoryAdministration.original[id].Version != 1 {
				return fmt.Errorf("expected tab version 1")
			}
		}
		return nil
	})
	sc.Step(`^que desde la primera pestaña ya cambió el nombre del rubro a "([^"]*)"$`, s.categoryFirstTabRename)
	sc.Step(`^(?:intento )?cambi(?:o|ar) el nombre del rubro "([^"]*)" por "([^"]*)" con la versión actual(?: y el motivo "([^"]*)")?$`, s.categoryRename)
	sc.Step(`^intento cambiar el nombre del rubro "([^"]*)" sin indicar la versión esperada$`, func(name string) error { return s.categoryPatch(name, map[string]any{"name": "Updated category"}) })
	sc.Step(`^intento cambiar desde la otra pestaña el nombre del rubro "([^"]*)" por "([^"]*)" usando la versión 1$`, func(name, next string) error {
		return s.categoryPatch(name, map[string]any{"name": next, "expected_version": 1})
	})
	sc.Step(`^intento deshabilitar el rubro "([^"]*)" con la versión actual y sin motivo$`, func(name string) error { return s.categorySetEnabled(name, false, "", false, "") })
	sc.Step(`^deshabilito el rubro "([^"]*)" y cambio su nombre a "([^"]*)" con la versión actual y el motivo "([^"]*)"$`, func(name, next, reason string) error { return s.categorySetEnabled(name, false, reason, false, next) })
	sc.Step(`^habilito el rubro "([^"]*)" con la versión actual y el motivo "([^"]*)"$`, func(name, reason string) error { return s.categorySetEnabled(name, true, reason, false, "") })
	sc.Step(`^intento deshabilitar el rubro "([^"]*)" con la versión actual y el motivo "([^"]*)" sin confirmar las órdenes en curso$`, func(name, reason string) error { return s.categorySetEnabled(name, false, reason, false, "") })
	sc.Step(`^deshabilito el rubro "([^"]*)" con la versión actual, el motivo "([^"]*)" y confirmo las órdenes en curso$`, func(name, reason string) error { return s.categorySetEnabled(name, false, reason, true, "") })
	sc.Step(`^actualizo el rubro "([^"]*)" con su mismo nombre y estado habilitado usando la versión 1$`, func(name string) error {
		return s.categoryPatch(name, map[string]any{"name": name, "enabled": true, "expected_version": 1})
	})
	sc.Step(`^el sistema (?:actualiza el rubro existente|actualiza el nombre visible a "([^"]*)")$`, s.categoryUpdated)
	sc.Step(`^la respuesta contiene el identificador original, el nombre "([^"]*)", su estado habilitado y la nueva versión$`, func(name string) error { return s.categoryResponseMatches(name, true, 2) })
	sc.Step(`^el sistema devuelve el mismo rubro deshabilitado con el nombre "([^"]*)" y su nueva versión$`, func(name string) error { return s.categoryResponseMatches(name, false, 2) })
	sc.Step(`^el sistema devuelve el mismo rubro (habilitado|deshabilitado) con su nueva versión$`, s.categoryResponseState)
	sc.Step(`^el rubro conserva su identificador y permanece habilitado$`, func() error { return s.categoryResponseState("habilitado") })
	sc.Step(`^el sistema rechaza (?:la edición|el cambio) con estado (\d+)(?: porque .*)?$`, s.lastResponseShouldHaveStatusCode)
	sc.Step(`^el sistema rechaza el cambio porque el nombre del rubro ya existe$`, s.systemRejectsDuplicateCategory)
	sc.Step(`^el rubro conserva su nombre, estado y versión$`, s.categoryUnchanged)
	sc.Step(`^el rubro permanece habilitado(?: con la misma versión)?$`, s.categoryUnchanged)
	sc.Step(`^ambos rubros conservan sus nombres y estados$`, s.allCategoriesUnchanged)
	sc.Step(`^el rubro conserva el nombre "([^"]*)" y sólo registra el cambio efectivo$`, s.categoryRetainsEffectiveChange)
	sc.Step(`^el sistema responde con estado 200 y devuelve el rubro con la versión 1$`, func() error { return s.categoryResponseVersion(1) })
	sc.Step(`^no queda registrado un evento (?:exitoso )?de edición (?:de|para) esta solicitud$`, s.categoryNoEditAudit)
	sc.Step(`^queda registrado un único evento de (edición|reactivación) de este rubro por "([^"]*)" con el motivo "([^"]*)"$`, s.categoryEditAudit)
	sc.Step(`^queda registrado un único evento para esta solicitud, correspondiente a la desactivación de este rubro por "([^"]*)" con el motivo "([^"]*)" y los estados anterior habilitado y posterior deshabilitado$`, func(email, reason string) error { return s.categoryEditAudit("desactivación", email, reason) })
}

func (s *testSuite) rememberCategoryOriginal(id int) error {
	if s.categoryAdministration.original == nil {
		s.categoryAdministration.original = map[int]category.Category{}
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("category %d not found", id)
	}
	if _, exists := s.categoryAdministration.original[id]; !exists {
		s.categoryAdministration.original[id] = *found
	}
	s.categoryAdministration.selected = id
	return nil
}
func (s *testSuite) categoryStateFixture(state, name string) error {
	if err := s.thereIsCategoryNamed(name); err != nil {
		return err
	}
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	found.Enabled = state == "habilitado"
	if _, err = s.categoryRepository.Save(*found); err != nil {
		return err
	}
	return s.rememberCategoryOriginal(id)
}
func (s *testSuite) categoryPatch(name string, payload map[string]any) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	if err = s.rememberCategoryOriginal(id); err != nil {
		return err
	}
	if err = s.categoryAuditBaseline(); err != nil {
		return err
	}
	s.categoryAuditCapture.resetAttempt()
	if err = s.sendAuthenticatedJSON(http.MethodPatch, "/categories/"+strconv.Itoa(id), "", payload); err != nil {
		return err
	}
	s.lastCategoryAuditEventIDs = s.categoryAuditCapture.snapshot()
	return nil
}
func (s *testSuite) categoryRename(name, next, reason string) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	return s.categoryPatch(name, map[string]any{"name": next, "expected_version": found.Version, "reason": reason})
}
func (s *testSuite) categorySetEnabled(name string, enabled bool, reason string, confirmed bool, next string) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	found, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	payload := map[string]any{"enabled": enabled, "reason": reason, "expected_version": found.Version, "confirm_ongoing_orders": confirmed}
	if next != "" {
		payload["name"] = next
	}
	return s.categoryPatch(name, payload)
}
func (s *testSuite) categoryFirstTabRename(next string) error {
	for name := range s.categoryIDsByName {
		if err := s.categoryRename(name, next, ""); err != nil {
			return err
		}
		if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
			return err
		}
		if len(s.lastCategoryAuditEventIDs) != 1 {
			return fmt.Errorf("expected exactly one effective tab edit audit")
		}
		return nil
	}
	return fmt.Errorf("no category fixture")
}
func (s *testSuite) categoryUpdated(name string) error {
	if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
		return err
	}
	if name != "" {
		return s.categoryResponseMatches(name, true, 2)
	}
	return nil
}
func (s *testSuite) categoryResponseMatches(name string, enabled bool, version int) error {
	if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
		return err
	}
	var response administeredCategoryResponse
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return err
	}
	if response.ID != s.categoryAdministration.selected || response.Name != name || response.Enabled != enabled || response.Version != version {
		return fmt.Errorf("category response mismatch: %s", s.lastBody)
	}
	persisted, err := s.categoryRepository.FindByID(s.scenarioContext, response.ID)
	if err != nil {
		return err
	}
	if persisted == nil || persisted.Name != response.Name || persisted.Enabled != response.Enabled || persisted.Version != response.Version {
		return fmt.Errorf("category response differs from persisted state")
	}
	return nil
}
func (s *testSuite) categoryResponseState(state string) error {
	original := s.categoryAdministration.original[s.categoryAdministration.selected]
	var response administeredCategoryResponse
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return err
	}
	return s.categoryResponseMatches(response.Name, state == "habilitado", original.Version+1)
}
func (s *testSuite) categoryResponseVersion(version int) error {
	original := s.categoryAdministration.original[s.categoryAdministration.selected]
	return s.categoryResponseMatches(original.Name, original.Enabled, version)
}
func (s *testSuite) categoryUnchanged() error {
	id := s.categoryAdministration.selected
	if id == 0 {
		for _, candidate := range s.categoryIDsByName {
			id = candidate
			break
		}
	}
	original, ok := s.categoryAdministration.original[id]
	if !ok {
		return fmt.Errorf("missing original category snapshot")
	}
	current, err := s.categoryRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	if current == nil || *current != original {
		return fmt.Errorf("category unexpectedly changed: %+v", current)
	}
	return nil
}
func (s *testSuite) allCategoriesUnchanged() error {
	for id, original := range s.categoryAdministration.original {
		current, err := s.categoryRepository.FindByID(s.scenarioContext, id)
		if err != nil {
			return err
		}
		if current == nil || *current != original {
			return fmt.Errorf("category %d changed", id)
		}
	}
	return nil
}
func (s *testSuite) categoryRetainsEffectiveChange(name string) error {
	current, err := s.categoryRepository.FindByID(s.scenarioContext, s.categoryAdministration.selected)
	if err != nil {
		return err
	}
	if current == nil || current.Name != name || current.Version != 2 {
		return fmt.Errorf("effective category change lost")
	}
	return s.categoryNoEditAudit()
}

func (s *testSuite) categoryAuditBaseline() error {
	mark, err := s.dependencies.Persistence.AuditEventRepository.CaptureWatermark(s.scenarioContext)
	if err != nil {
		return err
	}
	s.categoryAdministration.auditMark = mark
	return nil
}
func (s *testSuite) categoryNoEditAudit() error {
	mark, err := s.dependencies.Persistence.AuditEventRepository.CaptureWatermark(s.scenarioContext)
	if err != nil {
		return err
	}
	if mark != s.categoryAdministration.auditMark {
		return fmt.Errorf("unexpected persisted audit event")
	}
	if len(s.lastCategoryAuditEventIDs) != 0 || len(s.categoryAuditCapture.snapshot()) != 0 {
		return fmt.Errorf("unexpected category edit audit event")
	}
	return nil
}
func (s *testSuite) categoryEditAudit(kind, email, reason string) error {
	ids := s.lastCategoryAuditEventIDs
	if len(ids) != 1 {
		return fmt.Errorf("expected one edit audit event, got %d", len(ids))
	}
	event, err := s.auditEvents.FindByID(s.scenarioContext, ids[0])
	if err != nil {
		return err
	}
	operator, err := s.userRepository.FindOperatorIDByAuthID(s.scenarioContext, auth0IDForAdminEmail(email))
	if err != nil {
		return err
	}
	if event.OperatorID() != operator || event.ResourceType() != "category" || event.ResourceID() != strconv.Itoa(s.categoryAdministration.selected) || event.Action() != audit.ActionExecute || event.Result() != audit.ResultSucceeded || event.Reason() == nil || event.Reason().Text() != reason || event.CorrelationID() != s.adminRequest.headers.Get("X-Request-ID") {
		return fmt.Errorf("category edit audit mismatch")
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(s.scenarioContext, s.database, event.CorrelationID())
	if err != nil {
		return err
	}
	if total != 1 {
		return fmt.Errorf("expected exactly one committed category request audit, got %d", total)
	}
	_, offset := event.OccurredOn().Zone()
	if event.OccurredOn().IsZero() || offset != 0 {
		return fmt.Errorf("category audit instant missing or not UTC")
	}
	change := event.StateChange()
	if kind == "edición" {
		if change != nil {
			return fmt.Errorf("rename unexpectedly audited state transition")
		}
		return nil
	}
	from, to := "enabled", "disabled"
	if kind == "reactivación" {
		from, to = to, from
	}
	if change == nil || change.Field() != "enabled" || change.From() != from || change.To() != to {
		return fmt.Errorf("category state audit mismatch: %+v", change)
	}
	return nil
}
