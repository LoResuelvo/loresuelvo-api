package steps_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type categoryCreationRequest struct {
	Name string `json:"name"`
}

type categoryCreationResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
}

func registerCreateCategorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe el rubro "([^"]*)"$`, suite.thereIsCategoryNamed)
	sc.Step(`^creo el rubro "([^"]*)"$`, suite.requestCategoryCreationWithName)
	sc.Step(`^intento crear un rubro sin nombre$`, suite.tryCreateCategoryWithoutName)
	sc.Step(`^intento crear el rubro "([^"]*)"$`, suite.requestCategoryCreationWithName)
	sc.Step(`^intento crear un rubro con el nombre vacío$`, suite.tryCreateCategoryWithEmptyName)
	sc.Step(`^intento crear un rubro con solamente espacios$`, suite.tryCreateCategoryWithWhitespaceName)
	sc.Step(`^intento crear un rubro con un nombre de 101 caracteres$`, suite.tryCreateCategoryWithTooLongName)
	sc.Step(`^intento crear un rubro con un nombre numérico$`, suite.tryCreateCategoryWithNumericName)
	sc.Step(`^que estoy autenticado como administrador "([^"]*)" solamente con el permiso "([^"]*)"$`, suite.iAmAuthenticatedAsAdminWithPermission)
	sc.Step(`^el sistema crea el rubro$`, suite.systemCreatesCategory)
	sc.Step(`^la respuesta contiene el identificador, el nombre "([^"]*)" y el nombre normalizado "([^"]*)"$`, suite.categoryCreationResponseContains)
	sc.Step(`^la ubicación del recurso creado corresponde al identificador del rubro$`, suite.createdCategoryLocationMatchesID)
	sc.Step(`^queda registrado un único evento de creación exitosa para este rubro por "([^"]*)"$`, suite.categoryCreationAuditEventIsRecorded)
	sc.Step(`^ese evento contiene la fecha y hora "([^"]*)" en UTC y la correlación de esta solicitud$`, suite.categoryCreationAuditEventHasExpectedTimeAndCorrelation)
	sc.Step(`^el sistema rechaza la creación porque el nombre del rubro es obligatorio$`, suite.systemRejectsRequiredCategoryName)
	sc.Step(`^el sistema rechaza la creación porque el nombre del rubro es demasiado largo$`, suite.systemRejectsTooLongCategoryName)
	sc.Step(`^el sistema rechaza la creación porque el nombre del rubro debe ser texto$`, suite.systemRejectsNonTextCategoryName)
	sc.Step(`^el sistema rechaza la creación porque el rubro ya existe$`, suite.systemRejectsDuplicateCategory)
}

func (suite *testSuite) thereIsCategoryNamed(name string) error {
	categoryToSave, err := category.New(name)
	if err != nil {
		return fmt.Errorf("building category fixture: %w", err)
	}

	savedCategory, err := suite.categoryRepository.Save(*categoryToSave)
	if err != nil {
		if !errors.Is(err, category.ErrAlreadyExists) {
			return fmt.Errorf("saving category fixture: %w", err)
		}
		savedCategory = suite.categoryRepository.FindByNormalizedName(categoryToSave.NormalizedName)
		if savedCategory == nil {
			return fmt.Errorf("finding existing category fixture %q", name)
		}
	}

	suite.categoryIDsByName[name] = savedCategory.ID
	return nil
}

func (suite *testSuite) requestCategoryCreationWithName(name string) error {
	return suite.requestCategoryCreation(categoryCreationRequest{Name: name})
}

func (suite *testSuite) tryCreateCategoryWithoutName() error {
	return suite.requestCategoryCreation(map[string]any{})
}

func (suite *testSuite) tryCreateCategoryWithEmptyName() error {
	return suite.requestCategoryCreation(categoryCreationRequest{Name: ""})
}

func (suite *testSuite) tryCreateCategoryWithWhitespaceName() error {
	return suite.requestCategoryCreation(categoryCreationRequest{Name: "   "})
}

func (suite *testSuite) tryCreateCategoryWithTooLongName() error {
	return suite.requestCategoryCreation(categoryCreationRequest{Name: strings.Repeat("a", 101)})
}

func (suite *testSuite) tryCreateCategoryWithNumericName() error {
	return suite.requestCategoryCreation(map[string]any{"name": 42})
}

func (suite *testSuite) requestCategoryCreation(payload any) error {
	suite.categoryAuditCapture.reset()
	suite.lastCategoryAuditEventIDs = nil
	response, err := suite.postCategoryCreation(payload)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading category creation response: %w", err)
	}

	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	suite.lastLocation = response.Header.Get("Location")
	suite.lastRequestID = response.Header.Get("X-Request-ID")
	suite.lastCategoryAuditEventIDs = suite.categoryAuditCapture.snapshot()
	return nil
}

func (suite *testSuite) categoryCreationAuditEventIsRecorded(email string) error {
	if len(suite.lastCategoryAuditEventIDs) != 1 {
		return fmt.Errorf("expected exactly one committed category audit event for this request, got %d", len(suite.lastCategoryAuditEventIDs))
	}

	categoryResponse, err := suite.categoryCreationResponse()
	if err != nil {
		return err
	}
	event, err := suite.categoryAuditEvent(suite.lastCategoryAuditEventIDs[0])
	if err != nil {
		return err
	}
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(email))
	if err != nil {
		return fmt.Errorf("finding audit event operator for %q: %w", email, err)
	}
	if event.OperatorID() != operatorID || event.Action() != audit.ActionCreate ||
		event.ResourceType() != "category" || event.ResourceID() != fmt.Sprintf("%d", categoryResponse.ID) ||
		event.Result() != audit.ResultSucceeded {
		return fmt.Errorf(
			"category audit event mismatch: operator_id=%d action=%q resource_type=%q resource_id=%q result=%q",
			event.OperatorID(), event.Action(), event.ResourceType(), event.ResourceID(), event.Result(),
		)
	}
	if event.Reason() != nil || event.StateChange() != nil {
		return fmt.Errorf("category creation audit event unexpectedly contains optional metadata")
	}
	return nil
}

func (suite *testSuite) categoryCreationAuditEventHasExpectedTimeAndCorrelation(expectedTime string) error {
	if len(suite.lastCategoryAuditEventIDs) != 1 {
		return fmt.Errorf("expected exactly one committed category audit event for this request, got %d", len(suite.lastCategoryAuditEventIDs))
	}
	event, err := suite.categoryAuditEvent(suite.lastCategoryAuditEventIDs[0])
	if err != nil {
		return err
	}
	parsedExpectedTime, err := time.Parse(time.RFC3339Nano, expectedTime)
	if err != nil {
		return fmt.Errorf("parsing expected category audit timestamp %q: %w", expectedTime, err)
	}
	_, offset := event.OccurredOn().Zone()
	if offset != 0 || !event.OccurredOn().Equal(parsedExpectedTime.UTC()) {
		return fmt.Errorf("expected category audit timestamp %s in UTC, got %s", parsedExpectedTime.UTC().Format(time.RFC3339Nano), event.OccurredOn().Format(time.RFC3339Nano))
	}
	if suite.lastRequestID == "" || event.CorrelationID() != suite.lastRequestID {
		return fmt.Errorf("expected audit correlation %q from response X-Request-ID, got %q", suite.lastRequestID, event.CorrelationID())
	}
	return nil
}

func (suite *testSuite) categoryAuditEvent(id uuid.UUID) (*audit.Event, error) {
	return suite.auditEvents.FindByID(suite.scenarioContext, id)
}

func (suite *testSuite) systemCreatesCategory() error {
	return suite.categoryCreationResponseShouldHaveStatusCode(http.StatusCreated)
}

func (suite *testSuite) categoryCreationResponseContains(name, normalizedName string) error {
	response, err := suite.categoryCreationResponse()
	if err != nil {
		return err
	}
	if response.ID <= 0 || response.Name != name || response.NormalizedName != normalizedName {
		return fmt.Errorf("expected created category (%q, %q), got body %s", name, normalizedName, string(suite.lastBody))
	}

	suite.categoryIDsByName[name] = response.ID
	return nil
}

func (suite *testSuite) createdCategoryLocationMatchesID() error {
	response, err := suite.categoryCreationResponse()
	if err != nil {
		return err
	}
	expectedLocation := fmt.Sprintf("/categories/%d", response.ID)
	if suite.lastLocation != expectedLocation {
		return fmt.Errorf("expected category Location %q, got %q", expectedLocation, suite.lastLocation)
	}
	return nil
}

func (suite *testSuite) systemRejectsRequiredCategoryName() error {
	return suite.categoryCreationShouldFail(http.StatusBadRequest, category.ErrNameRequired.Error())
}

func (suite *testSuite) systemRejectsTooLongCategoryName() error {
	return suite.categoryCreationShouldFail(http.StatusBadRequest, category.ErrNameTooLong.Error())
}

func (suite *testSuite) systemRejectsNonTextCategoryName() error {
	return suite.categoryCreationShouldFail(http.StatusBadRequest, "Category name must be text")
}

func (suite *testSuite) systemRejectsDuplicateCategory() error {
	return suite.categoryCreationShouldFail(http.StatusConflict, category.ErrAlreadyExists.Error())
}

func (suite *testSuite) categoryCreationShouldFail(statusCode int, expectedMessage string) error {
	if err := suite.categoryCreationResponseShouldHaveStatusCode(statusCode); err != nil {
		return err
	}

	var response struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("category error response is not valid JSON: %w", err)
	}
	if response.Error != expectedMessage {
		return fmt.Errorf("expected category error %q, got body %s", expectedMessage, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) categoryCreationResponseShouldHaveStatusCode(statusCode int) error {
	if suite.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) categoryCreationResponse() (*categoryCreationResponse, error) {
	if err := suite.categoryCreationResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return nil, err
	}

	var response categoryCreationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return nil, fmt.Errorf("category creation response is not valid JSON: %w", err)
	}
	return &response, nil
}

func (suite *testSuite) categoryIDFor(name string) (int, error) {
	if categoryID, ok := suite.categoryIDsByName[name]; ok && categoryID != 0 {
		return categoryID, nil
	}

	return 0, fmt.Errorf("category %q does not exist", name)
}

func (suite *testSuite) postCategoryCreation(payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding category creation request: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, suite.server.URL+"/categories", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating category creation request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if suite.currentAuth0ID != "" && !suite.invalidSession {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("requesting category creation: %w", err)
	}
	return response, nil
}
