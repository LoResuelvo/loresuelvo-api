package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

const consumerHistoryPath = "/admin/consumers"

var errInjectedConsumerHistoryAudit = errors.New("injected consumer history audit failure")

type consumerHistoryTestCapture struct {
	failAudit          bool
	auditAttempts      int
	lastEvent          *audit.Event
	registrationWindow map[string]consumerRegistrationWindow
}

type consumerRegistrationWindow struct {
	startedOn   time.Time
	completedOn time.Time
}

func (capture *consumerHistoryTestCapture) reset() {
	capture.failAudit = false
	capture.auditAttempts = 0
	capture.lastEvent = nil
	capture.registrationWindow = make(map[string]consumerRegistrationWindow)
}

type consumerHistoryAuditWriterDecorator struct {
	inner   audit.Writer
	capture *consumerHistoryTestCapture
}

func (writer consumerHistoryAuditWriterDecorator) Save(ctx context.Context, event *audit.Event) error {
	writer.capture.auditAttempts++
	writer.capture.lastEvent = event
	if writer.capture.failAudit {
		return errInjectedConsumerHistoryAudit
	}
	return writer.inner.Save(ctx, event)
}

type consumerHistoryResponse struct {
	Consumer struct {
		ID           int                             `json:"id"`
		Role         string                          `json:"role"`
		Name         string                          `json:"name"`
		Surname      string                          `json:"surname"`
		Email        string                          `json:"email"`
		ProfilePhoto *string                         `json:"profile_photo_url"`
		CreatedOn    time.Time                       `json:"created_on"`
		Address      *consumerHistoryAddressResponse `json:"address"`
		CoverageZone *consumerHistoryZoneResponse    `json:"coverage_zone"`
	} `json:"consumer"`
	Summary struct {
		JobRequests      int `json:"job_requests"`
		ServiceProposals int `json:"service_proposals"`
		WorkOrders       int `json:"work_orders"`
	} `json:"summary"`
	Page struct {
		Items      []consumerHistoryItemResponse `json:"items"`
		Limit      int                           `json:"limit"`
		NextCursor *string                       `json:"next_cursor"`
	} `json:"page"`
}

type consumerHistoryAddressResponse struct {
	Street       string  `json:"street"`
	StreetNumber string  `json:"street_number"`
	Floor        *string `json:"floor"`
	Unit         *string `json:"unit"`
	Source       string  `json:"source"`
}

type consumerHistoryZoneResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}

type consumerHistoryPartyResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type consumerHistoryOperationResponse struct {
	ID                     string `json:"id"`
	URL                    string `json:"url"`
	RequiredPermission     string `json:"required_permission"`
	ChatRequiredPermission string `json:"chat_required_permission"`
}

type consumerHistoryItemResponse struct {
	Type                   string                           `json:"type"`
	ID                     int                              `json:"id"`
	Status                 string                           `json:"status"`
	Provider               consumerHistoryPartyResponse     `json:"provider"`
	OccurredOn             time.Time                        `json:"occurred_on"`
	Operation              consumerHistoryOperationResponse `json:"operation"`
	CreatedOn              *time.Time                       `json:"created_on"`
	ScheduledOn            *time.Time                       `json:"scheduled_on"`
	EstimatedDuration      *int                             `json:"estimated_duration_minutes"`
	BookingPaymentDeadline *time.Time                       `json:"booking_payment_deadline"`
	JobRequestID           *int                             `json:"job_request_id"`
	ServiceProposalID      *int                             `json:"service_proposal_id"`
	AcceptedOn             *time.Time                       `json:"accepted_on"`
	CompletionReportedOn   *time.Time                       `json:"completion_reported_on"`
	BalancePaidOn          *time.Time                       `json:"balance_paid_on"`
}

type consumerHistoryState struct {
	consumerEmail string
	consumerID    int
	correlation   string
	cursor        string
}

func registerAdminConsumerHistorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen los siguientes consumidores registrados con dirección:$`, suite.thereAreConsumerHistoryConsumersWithAddress)
	sc.Step(`^que "([^"]*)" no tiene foto de perfil persistida ni actividad de contratación$`, suite.consumerHistoryConsumerHasNoPhotoOrActivity)
	sc.Step(`^consulto la ficha y el historial administrativo de "([^"]*)"(?: con correlación "([^"]*)")?$`, suite.queryConsumerHistory)
	sc.Step(`^consulto el historial de "([^"]*)"(?: con tipo "([^"]*)" y estado "([^"]*)")?$`, suite.queryConsumerHistoryByTypeAndStatus)
	sc.Step(`^consulto el historial del consumidor "([^"]*)" filtrado por el prestador "([^"]*)" desde "([^"]*)" hasta "([^"]*)"$`, suite.queryConsumerHistoryByProviderAndWindow)
	sc.Step(`^consulto el historial de "([^"]*)" con límite (\d+)$`, suite.queryConsumerHistoryWithLimit)
	sc.Step(`^que consulté la primera página del historial de "([^"]*)" con límite (\d+) y guardé su cursor$`, suite.queryAndStoreConsumerHistoryCursor)
	sc.Step(`^continúo el historial de "([^"]*)" con el cursor guardado y límite (\d+)$`, suite.continueConsumerHistory)
	sc.Step(`^la ficha identifica al consumidor por su ID persistido, rol, nombre, apellido, correo y fecha de registro$`, suite.consumerHistoryHasConsumerIdentity)
	sc.Step(`^la foto de perfil se informa como nula$`, suite.consumerHistoryPhotoIsNull)
	sc.Step(`^la dirección informa "([^"]*)", "([^"]*)", "([^"]*)" y "([^"]*)", con procedencia "([^"]*)"$`, suite.consumerHistoryAddressEquals)
	sc.Step(`^las entradas no contienen un campo de domicilio histórico ni copian la dirección del perfil$`, suite.consumerHistoryHasNoHistoricalAddress)
	sc.Step(`^la zona informa el ID persistido, nombre "([^"]*)" y disponibilidad habilitada, con procedencia "([^"]*)"$`, suite.consumerHistoryZoneEquals)
	sc.Step(`^el resumen global informa (\d+) solicitudes, (\d+) propuestas y (\d+) órdenes$`, suite.consumerHistorySummaryEquals)
	sc.Step(`^la página informa una colección vacía, no nula, límite (\d+) y cursor siguiente nulo$`, suite.consumerHistoryEmptyPageEquals)
	sc.Step(`^antes de entregar la ficha queda preparado exactamente un evento de acceso al recurso "([^"]*)" con el ID persistido de "([^"]*)", el operador "([^"]*)" y la correlación "([^"]*)"$`, suite.consumerHistoryAuditPreparedOnce)
	sc.Step(`^el resultado del evento es "prepared", sin contenido de la ficha ni motivo manual$`, suite.consumerHistoryAuditContainsNoHistoryData)
	sc.Step(`^el historial contiene exactamente, en este orden:$`, suite.consumerHistoryItemsMatchTable)
	sc.Step(`^cada recurso conserva su ID persistido y el ID, nombre y apellido del prestador correspondiente$`, suite.consumerHistoryItemsHavePersistedProvider)
	sc.Step(`^las propuestas y órdenes conservan exactamente estas relaciones por IDs persistidos:$`, suite.consumerHistoryRelationsMatchTable)
	sc.Step(`^las entradas de solicitud no incluyen un vínculo singular a propuesta u orden$`, suite.consumerHistoryRequestsHaveNoVariantLinks)
	sc.Step(`^las solicitudes informan su created_on persistido y las propuestas su created_on, scheduled_on, estimated_duration_minutes y booking_payment_deadline persistidos, con zona horaria en las fechas$`, suite.consumerHistoryRequestAndProposalDatesArePersisted)
	sc.Step(`^las órdenes informan accepted_on, completion_reported_on y balance_paid_on desde sus datos persistidos, con zona horaria en las fechas disponibles$`, suite.consumerHistoryWorkOrderDatesArePersisted)
	sc.Step(`^"([^"]*)" informa completion_reported_on "([^"]*)" y balance_paid_on nulo$`, suite.consumerHistoryOrderCompletionAndNullPayment)
	sc.Step(`^"([^"]*)" informa completion_reported_on y balance_paid_on nulos$`, suite.consumerHistoryOrderDatesAreNull)
	sc.Step(`^no se incorporan campos de aceptación de solicitud o propuesta sin un instante propio persistido$`, suite.consumerHistoryOmitsUnpersistedAcceptDates)
	sc.Step(`^"([^"]*)" referencia la operación "jr-" seguida de su ID persistido$`, suite.consumerHistoryRequestOperationReference)
	sc.Step(`^"([^"]*)", "([^"]*)" y "([^"]*)" referencian la operación "jr-" seguida del ID persistido de "([^"]*)"$`, suite.consumerHistoryLinkedResourcesReferenceRequestOperation)
	sc.Step(`^"([^"]*)" y "([^"]*)" referencian la operación "jr-" seguida del ID persistido de "([^"]*)"$`, suite.consumerHistoryTwoLinkedResourcesReferenceRequestOperation)
	sc.Step(`^"([^"]*)" y "([^"]*)" referencian cada una su operación "sp-" seguida de su propio ID persistido$`, suite.consumerHistoryOrphanProposalsReferenceSelf)
	sc.Step(`^no aparece ningún recurso de "([^"]*)"$`, suite.consumerHistoryExcludesConsumer)
	sc.Step(`^la página contiene exactamente los recursos "([^"]*)"(?: en ese orden)?$`, suite.consumerHistoryPageContainsLabels)
	sc.Step(`^la página informa límite (\d+) y un cursor siguiente no nulo$`, suite.consumerHistoryPageHasCursor)
	sc.Step(`^la página informa un cursor siguiente no nulo$`, suite.consumerHistoryPageHasAnyCursor)
	sc.Step(`^la ficha y cada tipo de entrada contienen únicamente los campos del contrato público acotado$`, suite.consumerHistoryHasExactAllowlistedFields)
	sc.Step(`^el texto de los mensajes y los IDs de los adjuntos preparados no aparecen en la respuesta$`, suite.consumerHistoryDoesNotExposeChatContent)
	sc.Step(`^la respuesta no contiene credenciales, identificadores de autenticación ni documentos privados$`, suite.consumerHistoryDoesNotExposeSensitiveData)
	sc.Step(`^no se devuelven transacciones financieras completas ni payloads del procesador$`, suite.consumerHistoryDoesNotExposePaymentData)
	sc.Step(`^las referencias de operación señalan que el detalle requiere "([^"]*)", el chat "([^"]*)", independientemente de "([^"]*)"$`, suite.consumerHistoryReferencesIndependentPermissions)
	sc.Step(`^consulto el historial de "([^"]*)" con la consulta "([^"]*)"$`, suite.queryConsumerHistoryWithRawQuery)
	sc.Step(`^no se entrega la ficha ni el historial$`, suite.consumerHistoryErrorHasNoData)
	sc.Step(`^no se registra ningún evento de acceso para esta solicitud$`, suite.consumerHistoryNoAccessEvent)
	sc.Step(`^que consulto con autenticación "([^"]*)"$`, suite.consumerHistoryAuthentication)
	sc.Step(`^consulto la ficha y el historial administrativo con un ID positivo de consumidor inexistente$`, suite.queryMissingConsumerHistory)
	sc.Step(`^que falla la persistencia del evento de acceso administrativo$`, suite.failConsumerHistoryAudit)
	sc.Step(`^no se persiste ningún evento de acceso para esta solicitud$`, suite.consumerHistoryNoPersistedAuditForFailedRequest)
}

func (suite *testSuite) thereAreConsumerHistoryConsumersWithAddress(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	expectedHeaders := []string{"correo", "nombre", "apellido", "calle", "número", "piso", "unidad"}
	if len(table.Rows[0].Cells) != len(expectedHeaders) {
		return fmt.Errorf("consumer history address fixture headers must be %v", expectedHeaders)
	}
	for i, header := range expectedHeaders {
		if table.Rows[0].Cells[i].Value != header {
			return fmt.Errorf("consumer history address fixture header %d must be %q, got %q", i+1, header, table.Rows[0].Cells[i].Value)
		}
	}
	for _, row := range rows {
		if row["correo"] == "" || row["nombre"] == "" || row["apellido"] == "" || row["calle"] == "" || row["número"] == "" {
			return fmt.Errorf("consumer history fixture requires identity and a real address: %+v", row)
		}
		if strings.ToLower(strings.TrimSpace(row["calle"])) != "av. rivadavia" {
			return fmt.Errorf("consumer history address fixture only supports the configured geocoder's Av. Rivadavia address, got %q", row["calle"])
		}
		request := consumerRegistrationRequest{Email: row["correo"], Name: row["nombre"], Surname: row["apellido"], Address: &consumerRegistrationAddress{Street: row["calle"], StreetNumber: row["número"], Floor: row["piso"], Unit: row["unidad"]}}
		startedOn := time.Now().UTC()
		response, err := suite.postConsumerRegistrationWithAuth0ID(auth0IDForConsumerEmail(request.Email), request)
		if err != nil {
			return err
		}
		body, readErr := readHTTPBody(response)
		if readErr != nil {
			return readErr
		}
		completedOn := time.Now().UTC()
		if response.StatusCode != http.StatusCreated {
			return fmt.Errorf("registering consumer history fixture %q returned %d: %s", request.Email, response.StatusCode, body)
		}
		if suite.consumerHistoryCapture == nil {
			return fmt.Errorf("consumer history test capture is not configured")
		}
		suite.consumerHistoryCapture.registrationWindow[request.Email] = consumerRegistrationWindow{startedOn: startedOn, completedOn: completedOn}
		suite.rememberParticipantFullName(request.Name, request.Surname, participantRoleConsumer)
	}
	return nil
}

func readHTTPBody(response *http.Response) (string, error) {
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("reading consumer registration response: %w", err)
	}
	return string(data), nil
}

func (suite *testSuite) consumerHistoryConsumerHasNoPhotoOrActivity(email string) error {
	user, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(email))
	if err != nil {
		return err
	}
	profile, ok := user.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("expected persisted consumer %q, got %T", email, user)
	}
	if profile.ProfilePhoto() != nil {
		return fmt.Errorf("consumer history empty-state fixture unexpectedly has a persisted profile photo")
	}
	if len(suite.operationInbox.requests) != 0 || len(suite.operationInbox.proposals) != 0 || len(suite.operationInbox.orders) != 0 {
		return fmt.Errorf("consumer history empty-state fixture already contains operation resources")
	}
	return nil
}

func (suite *testSuite) queryConsumerHistory(email, correlation string) error {
	query := url.Values{}
	return suite.sendConsumerHistoryRequest(email, query, correlation)
}

func (suite *testSuite) queryConsumerHistoryByTypeAndStatus(email, itemType, status string) error {
	query := url.Values{}
	if itemType != "" {
		query.Set("type", itemType)
	}
	if status != "" {
		query.Set("status", status)
	}
	return suite.sendConsumerHistoryRequest(email, query, "")
}

func (suite *testSuite) queryConsumerHistoryByProviderAndWindow(consumerEmail, providerEmail, from, to string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	query := url.Values{"provider_id": {strconv.Itoa(providerID)}, "from": {from}, "to": {to}}
	return suite.sendConsumerHistoryRequest(consumerEmail, query, "")
}

func (suite *testSuite) queryConsumerHistoryWithLimit(email, limit string) error {
	query := url.Values{"limit": {limit}}
	return suite.sendConsumerHistoryRequest(email, query, "")
}

func (suite *testSuite) queryAndStoreConsumerHistoryCursor(email, limit string) error {
	if err := suite.queryConsumerHistoryWithLimit(email, limit); err != nil {
		return err
	}
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if response.Page.NextCursor == nil || strings.TrimSpace(*response.Page.NextCursor) == "" {
		return fmt.Errorf("first consumer history page has no next cursor: %s", suite.lastBody)
	}
	suite.consumerHistory.cursor = *response.Page.NextCursor
	return nil
}

func (suite *testSuite) continueConsumerHistory(email, limit string) error {
	if suite.consumerHistory.cursor == "" {
		return fmt.Errorf("consumer history cursor was not saved from a real first-page response")
	}
	query := url.Values{"cursor": {suite.consumerHistory.cursor}, "limit": {limit}}
	return suite.sendConsumerHistoryRequest(email, query, "")
}

func (suite *testSuite) queryConsumerHistoryWithRawQuery(email, rawQuery string) error {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return fmt.Errorf("parsing consumer history query: %w", err)
	}
	return suite.sendConsumerHistoryRequest(email, query, "")
}

func (suite *testSuite) sendConsumerHistoryRequest(email string, query url.Values, correlation string) error {
	if correlation != "" {
		if err := testsupport.ResetOperationDetailAuditCorrelation(suite.scenarioContext, suite.database, correlation); err != nil {
			return err
		}
	}
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("resolving persisted consumer ID for %q: %w", email, err)
	}
	suite.consumerHistory = consumerHistoryState{consumerEmail: email, consumerID: consumerID, correlation: correlation}
	return suite.sendAdminGet(consumerHistoryPath+"/"+url.PathEscape(strconv.Itoa(consumerID))+"/history", query, correlation)
}

func (suite *testSuite) queryMissingConsumerHistory() error {
	suite.consumerHistory = consumerHistoryState{consumerID: 2147483647}
	return suite.sendAdminGet(consumerHistoryPath+"/2147483647/history", nil, "")
}

func (suite *testSuite) consumerHistoryAuthentication(authentication string) error {
	suite.adminRequest.omitBearer = false
	suite.adminRequest.invalidBearer = false
	suite.currentAuth0ID = auth0IDForAdminEmail("soporte@example.com")
	switch authentication {
	case "sin token":
		suite.adminRequest.omitBearer = true
	case "token inválido":
		suite.adminRequest.invalidBearer = true
	case "soporte con read:admin_operations":
		suite.currentPermissions = []string{"read:admin_operations"}
	default:
		return fmt.Errorf("unsupported consumer history authentication %q", authentication)
	}
	return nil
}

func (suite *testSuite) failConsumerHistoryAudit() error {
	if suite.consumerHistoryCapture == nil {
		return fmt.Errorf("consumer history audit capture is not configured")
	}
	suite.consumerHistoryCapture.failAudit = true
	return nil
}

func (suite *testSuite) decodedConsumerHistory() (consumerHistoryResponse, error) {
	if suite.lastStatus != http.StatusOK {
		return consumerHistoryResponse{}, fmt.Errorf("expected consumer history status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var response consumerHistoryResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return consumerHistoryResponse{}, fmt.Errorf("decoding consumer history response: %w", err)
	}
	if response.Consumer.ID <= 0 || response.Page.Items == nil {
		return consumerHistoryResponse{}, fmt.Errorf("consumer history response lacks persisted consumer/page data: %s", suite.lastBody)
	}
	return response, nil
}

func (suite *testSuite) consumerHistoryHasConsumerIdentity() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	persisted, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(suite.consumerHistory.consumerEmail))
	if err != nil {
		return err
	}
	profile, ok := persisted.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("expected persisted consumer, got %T", persisted)
	}
	if response.Consumer.ID != profile.ID() || response.Consumer.ID != suite.consumerHistory.consumerID || response.Consumer.Role != consumer.Role || response.Consumer.Name != profile.Name() || response.Consumer.Surname != profile.Surname() || response.Consumer.Email != profile.Email() || response.Consumer.CreatedOn.IsZero() {
		return fmt.Errorf("consumer identity does not match persisted profile: %s", suite.lastBody)
	}
	raw, err := consumerHistoryRaw(suite.lastBody)
	if err != nil {
		return err
	}
	created, ok := raw["consumer"].(map[string]any)["created_on"].(string)
	if !ok {
		return fmt.Errorf("created_on is not an RFC3339 timestamp: %s", suite.lastBody)
	}
	createdOn, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("created_on has no timezone: %q", created)
	}
	if suite.consumerHistoryCapture == nil {
		return fmt.Errorf("consumer history test capture is not configured")
	}
	window, ok := suite.consumerHistoryCapture.registrationWindow[suite.consumerHistory.consumerEmail]
	if !ok || createdOn.Before(window.startedOn) || createdOn.After(window.completedOn) {
		return fmt.Errorf("consumer created_on %s falls outside actual registration interval [%s, %s]", createdOn, window.startedOn, window.completedOn)
	}
	return nil
}

func (suite *testSuite) consumerHistoryPhotoIsNull() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	persisted, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(suite.consumerHistory.consumerEmail))
	if err != nil {
		return err
	}
	if persisted.ProfilePhoto() != nil {
		return fmt.Errorf("fixture consumer has a persisted profile photo; null-photo assertion is invalid")
	}
	if response.Consumer.ProfilePhoto != nil {
		return fmt.Errorf("expected null profile_photo_url, got %q", *response.Consumer.ProfilePhoto)
	}
	return assertHistoryNullableKey(suite.lastBody, "consumer", "profile_photo_url", true)
}

func (suite *testSuite) consumerHistoryAddressEquals(street, number, floor, unit, source string) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	persisted, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(suite.consumerHistory.consumerEmail))
	if err != nil {
		return err
	}
	profile, ok := persisted.(*consumer.Consumer)
	if !ok || response.Consumer.Address == nil {
		return fmt.Errorf("consumer current address is absent from response or fixture")
	}
	address := profile.Address()
	if address.Street != street || address.StreetNumber != number || stringValue(response.Consumer.Address.Floor) != floor || stringValue(response.Consumer.Address.Unit) != unit || response.Consumer.Address.Street != address.Street || response.Consumer.Address.StreetNumber != address.StreetNumber || stringValue(response.Consumer.Address.Floor) != address.Floor || stringValue(response.Consumer.Address.Unit) != address.Unit || response.Consumer.Address.Source != source {
		return fmt.Errorf("current address differs from persisted consumer profile: %s", suite.lastBody)
	}
	if response.Consumer.Address.Floor == nil {
		if err := assertHistoryNullableKey(suite.lastBody, "consumer.address", "floor", true); err != nil {
			return err
		}
	}
	if response.Consumer.Address.Unit == nil {
		if err := assertHistoryNullableKey(suite.lastBody, "consumer.address", "unit", true); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryZoneEquals(name, source string) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	persisted, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(suite.consumerHistory.consumerEmail))
	if err != nil {
		return err
	}
	profile, ok := persisted.(*consumer.Consumer)
	if !ok || response.Consumer.CoverageZone == nil {
		return fmt.Errorf("consumer coverage zone is absent from response or fixture")
	}
	zone := profile.CoverageZone()
	if response.Consumer.CoverageZone.ID != zone.ID || response.Consumer.CoverageZone.Name != name || response.Consumer.CoverageZone.Name != zone.Name || !response.Consumer.CoverageZone.Enabled || response.Consumer.CoverageZone.Enabled != zone.Enabled || response.Consumer.CoverageZone.Source != source {
		return fmt.Errorf("current coverage zone differs from persisted consumer profile: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) consumerHistorySummaryEquals(requests, proposals, orders int) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	actual := [3]int{response.Summary.JobRequests, response.Summary.ServiceProposals, response.Summary.WorkOrders}
	expected := [3]int{requests, proposals, orders}
	if actual != expected {
		return fmt.Errorf("expected global summary %v, got %v", expected, actual)
	}
	return nil
}

func (suite *testSuite) consumerHistoryEmptyPageEquals(limit int) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if response.Page.Items == nil || len(response.Page.Items) != 0 || response.Page.Limit != limit || response.Page.NextCursor != nil {
		return fmt.Errorf("expected empty non-null page limit=%d cursor=null, got %+v", limit, response.Page)
	}
	return assertHistoryNullableKey(suite.lastBody, "page", "next_cursor", true)
}

func (suite *testSuite) consumerHistoryAuditPreparedOnce(resourceType, email, operatorEmail, correlation string) error {
	if resourceType != "consumer" || suite.consumerHistory.consumerEmail != email || suite.consumerHistory.correlation != correlation {
		return fmt.Errorf("unexpected consumer history audit target or correlation")
	}
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(operatorEmail))
	if err != nil {
		return err
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	filter := audit.LogFilter{OperatorID: &operatorID, ResourceType: &resourceType}
	resourceID := strconv.Itoa(suite.consumerHistory.consumerID)
	filter.ResourceID = &resourceID
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return fmt.Errorf("capturing post-response audit watermark: %w", err)
	}
	events, err := reader.FindPage(suite.scenarioContext, filter, watermark, nil, 100)
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		if event.ResourceID() != resourceID || event.Action() != audit.ActionAccess || event.Result() != audit.ResultPrepared {
			return fmt.Errorf("consumer history audit event has incorrect resource/action/result")
		}
		persisted, err := reader.FindByID(suite.scenarioContext, event.ID())
		if err != nil || persisted == nil {
			return fmt.Errorf("consumer history audit event was not persisted: %v", err)
		}
		matches++
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	attempts := 0
	if suite.consumerHistoryCapture != nil {
		attempts = suite.consumerHistoryCapture.auditAttempts
	}
	if matches != 1 || total != 1 || attempts != 1 {
		return fmt.Errorf("expected one persisted/synchronous access event, got page=%d correlation=%d saves=%d", matches, total, attempts)
	}
	return nil
}

func (suite *testSuite) consumerHistoryAuditContainsNoHistoryData() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	_ = response
	if suite.consumerHistoryCapture != nil && suite.consumerHistoryCapture.lastEvent != nil {
		event := suite.consumerHistoryCapture.lastEvent
		if event.Result() != audit.ResultPrepared || event.Reason() != nil || event.StateChange() != nil || event.ConversationID() != nil {
			return fmt.Errorf("consumer history audit event contains an unexpected result, reason, or conversation id")
		}
		return nil
	}
	return fmt.Errorf("consumer history audit writer did not capture the event")
}

func (suite *testSuite) consumerHistoryItemsMatchTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if len(response.Page.Items) != len(rows) {
		return fmt.Errorf("expected exactly %d history items, got %d", len(rows), len(response.Page.Items))
	}
	for i, row := range rows {
		item := response.Page.Items[i]
		wantID, err := suite.consumerHistoryResourceID(row["recurso"], row["tipo"])
		if err != nil {
			return err
		}
		occurred, err := parseInboxInstant(row["instante"])
		if err != nil {
			return err
		}
		providerID, err := suite.providerIDByEmail(row["prestador"])
		if err != nil {
			return err
		}
		providerUser, err := suite.userRepository.FindByAuthID(auth0IDForProviderEmail(row["prestador"]))
		if err != nil {
			return err
		}
		if item.Type != row["tipo"] || item.ID != wantID || item.Status != row["estado"] || !item.OccurredOn.Equal(occurred) || item.Provider.ID != providerID || item.Provider.Name != providerUser.Name() || item.Provider.Surname != providerUser.Surname() {
			return fmt.Errorf("history item %d differs from persisted %s %q fixture: %+v", i, row["tipo"], row["recurso"], item)
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryResourceID(label, itemType string) (int, error) {
	switch itemType {
	case "job_request":
		request, ok := suite.operationInbox.requests[label]
		if !ok {
			return 0, fmt.Errorf("unknown request label %q", label)
		}
		return request.id, nil
	case "service_proposal":
		proposal, ok := suite.operationInbox.proposals[label]
		if !ok {
			return 0, fmt.Errorf("unknown proposal label %q", label)
		}
		return proposal.id, nil
	case "work_order":
		id, ok := suite.operationInbox.orders[label]
		if !ok {
			return 0, fmt.Errorf("unknown order label %q", label)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unsupported history item type %q", itemType)
	}
}

func (suite *testSuite) consumerHistoryItemsHavePersistedProvider() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		label := suite.consumerHistoryLabel(item)
		if label == "" {
			return fmt.Errorf("history item %s:%d has no persisted fixture label", item.Type, item.ID)
		}
		providerID, err := suite.consumerHistoryPersistedProviderID(item, label)
		if err != nil {
			return err
		}
		providerUser, err := suite.userRepository.FindByID(suite.scenarioContext, providerID)
		if err != nil {
			return fmt.Errorf("finding persisted provider %d for %s:%d: %w", providerID, item.Type, item.ID, err)
		}
		if item.ID <= 0 || item.Provider.ID != providerUser.ID() || item.Provider.Name != providerUser.Name() || item.Provider.Surname != providerUser.Surname() {
			return fmt.Errorf("history item %s:%d provider differs from persisted resource relationship: got %+v, want id=%d name=%q surname=%q", item.Type, item.ID, item.Provider, providerUser.ID(), providerUser.Name(), providerUser.Surname())
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryPersistedProviderID(item consumerHistoryItemResponse, label string) (int, error) {
	switch item.Type {
	case "job_request":
		fixture, ok := suite.operationInbox.requests[label]
		if !ok || fixture.id != item.ID {
			return 0, fmt.Errorf("request item %d does not match fixture label %q", item.ID, label)
		}
		request, err := suite.jobRequestRepository.FindByID(item.ID)
		if err != nil {
			return 0, err
		}
		return request.ProviderID, nil
	case "service_proposal":
		fixture, ok := suite.operationInbox.proposals[label]
		if !ok || fixture.id != item.ID {
			return 0, fmt.Errorf("proposal item %d does not match fixture label %q", item.ID, label)
		}
		proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, item.ID)
		if err != nil {
			return 0, err
		}
		if proposal.Provider == nil {
			return 0, fmt.Errorf("persisted proposal %d has no provider", item.ID)
		}
		request, ok := suite.operationInbox.requests[fixture.requestLabel]
		if !ok {
			return 0, fmt.Errorf("proposal %q has no linked persisted request fixture", label)
		}
		if proposal.Consumer == nil || proposal.Consumer.ID() != suite.consumerHistory.consumerID || request.id <= 0 {
			return 0, fmt.Errorf("proposal %q does not belong to the persisted consumer/request fixtures", label)
		}
		return proposal.Provider.ID(), nil
	case "work_order":
		fixtureID, ok := suite.operationInbox.orders[label]
		if !ok || fixtureID != item.ID {
			return 0, fmt.Errorf("work-order item %d does not match fixture label %q", item.ID, label)
		}
		order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, item.ID)
		if err != nil {
			return 0, err
		}
		if order.ID() != item.ID {
			return 0, fmt.Errorf("persisted work order ID %d differs from response ID %d", order.ID(), item.ID)
		}
		proposalID := order.ServiceProposalID()
		proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, proposalID)
		if err != nil {
			return 0, fmt.Errorf("finding work order %d proposal %d: %w", item.ID, proposalID, err)
		}
		var linkedProposal inboxProposalFixture
		foundProposal := false
		for _, proposal := range suite.operationInbox.proposals {
			if proposal.id == proposalID {
				linkedProposal, foundProposal = proposal, true
				break
			}
		}
		if !foundProposal {
			return 0, fmt.Errorf("work order %d links to unknown persisted proposal %d", item.ID, proposalID)
		}
		if _, ok := suite.operationInbox.requests[linkedProposal.requestLabel]; !ok {
			return 0, fmt.Errorf("work order %d proposal has no linked request fixture", item.ID)
		}
		if proposal.Provider == nil || proposal.Provider.ID() != order.ProviderID() {
			return 0, fmt.Errorf("work order %d provider differs from its persisted proposal %d", item.ID, proposalID)
		}
		return proposal.Provider.ID(), nil
	default:
		return 0, fmt.Errorf("unsupported history item type %q", item.Type)
	}
}

func (suite *testSuite) consumerHistoryRelationsMatchTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, row := range rows {
		item, err := suite.consumerHistoryFindItem(response.Page.Items, row["recurso"])
		if err != nil {
			return err
		}
		switch item.Type {
		case "service_proposal":
			proposal, proposalExists := suite.operationInbox.proposals[row["recurso"]]
			request, requestExists := suite.operationInbox.requests[row["solicitud"]]
			if !proposalExists || !requestExists {
				return fmt.Errorf("proposal relationship row has unknown fixture labels: %+v", row)
			}
			persisted, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, proposal.id)
			if err != nil {
				return fmt.Errorf("finding persisted proposal %q: %w", row["recurso"], err)
			}
			if persisted.ID != proposal.id || persisted.Consumer == nil || persisted.Provider == nil || item.JobRequestID == nil || *item.JobRequestID != request.id || proposal.requestLabel != row["solicitud"] {
				return fmt.Errorf("proposal %q request link differs from persisted IDs/fixture relation", row["recurso"])
			}
			persistedRequest, err := suite.jobRequestRepository.FindByID(request.id)
			if err != nil || persistedRequest.ID != request.id || persistedRequest.ConversationID != request.conversationID {
				return fmt.Errorf("proposal %q is not linked to its persisted request fixture: %v", row["recurso"], err)
			}
		case "work_order":
			proposal, proposalExists := suite.operationInbox.proposals[row["propuesta"]]
			request, requestExists := suite.operationInbox.requests[row["solicitud"]]
			if !proposalExists || !requestExists {
				return fmt.Errorf("work-order relationship row has unknown fixture labels: %+v", row)
			}
			persisted, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, proposal.id)
			if err != nil {
				return err
			}
			if persisted == nil || persisted.ID() != item.ID || item.JobRequestID == nil || *item.JobRequestID != request.id || item.ServiceProposalID == nil || *item.ServiceProposalID != proposal.id || persisted.ServiceProposalID() != proposal.id {
				return fmt.Errorf("work order %q links differ from persisted relation", row["recurso"])
			}
		default:
			return fmt.Errorf("relationship table row %q names unsupported type %q", row["recurso"], item.Type)
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryFindItem(items []consumerHistoryItemResponse, label string) (consumerHistoryItemResponse, error) {
	for _, item := range items {
		for _, itemType := range []string{"job_request", "service_proposal", "work_order"} {
			id, err := suite.consumerHistoryResourceID(label, itemType)
			if err == nil && item.Type == itemType && item.ID == id {
				return item, nil
			}
		}
	}
	return consumerHistoryItemResponse{}, fmt.Errorf("history response does not contain fixture resource %q", label)
}

func (suite *testSuite) consumerHistoryRequestsHaveNoVariantLinks() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		if item.Type == "job_request" {
			if item.JobRequestID != nil || item.ServiceProposalID != nil {
				return fmt.Errorf("job request contains singular proposal/order links: %+v", item)
			}
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryRequestAndProposalDatesArePersisted() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		switch item.Type {
		case "job_request":
			fixture, err := suite.consumerHistoryItemInstant(item, item.CreatedOn)
			if err != nil {
				return err
			}
			if !fixture.Equal(item.OccurredOn) {
				return fmt.Errorf("request occurred_on differs from persisted created_on")
			}
			if err := assertHistoryItemInstant(suite.lastBody, item.Type, item.ID, "created_on"); err != nil {
				return err
			}
		case "service_proposal":
			if item.CreatedOn == nil || item.ScheduledOn == nil || item.EstimatedDuration == nil || item.BookingPaymentDeadline == nil {
				return fmt.Errorf("proposal does not expose its persisted typed dates/terms: %+v", item)
			}
			fixture := suite.operationInbox.proposals[suite.consumerHistoryLabel(item)]
			persisted, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, fixture.id)
			if err != nil {
				return err
			}
			if !item.CreatedOn.Equal(persisted.CreatedOn) || !item.ScheduledOn.Equal(persisted.ScheduledOn) || *item.EstimatedDuration != persisted.EstimatedDurationMinutes || !item.BookingPaymentDeadline.Equal(persisted.BookingTerms.BookingPaymentDeadline()) || !item.OccurredOn.Equal(persisted.CreatedOn) {
				return fmt.Errorf("proposal date/terms differ from persisted proposal: %+v", item)
			}
			for _, field := range []string{"created_on", "scheduled_on", "booking_payment_deadline"} {
				if err := assertHistoryItemInstant(suite.lastBody, item.Type, item.ID, field); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryItemInstant(item consumerHistoryItemResponse, value *time.Time) (time.Time, error) {
	if value == nil {
		return time.Time{}, fmt.Errorf("resource %d has no created_on", item.ID)
	}
	var expected time.Time
	for _, fixture := range suite.operationInbox.requests {
		if fixture.id == item.ID && item.Type == "job_request" {
			expected = fixture.createdOn
			break
		}
	}
	if expected.IsZero() {
		return time.Time{}, fmt.Errorf("no persisted creation fixture for job request %d", item.ID)
	}
	if !value.Equal(expected) || !item.OccurredOn.Equal(expected) {
		return time.Time{}, fmt.Errorf("job request created_on/occurred_on differs from persisted request")
	}
	return expected, nil
}

func assertHistoryItemInstant(body []byte, itemType string, id int, field string) error {
	item, err := rawHistoryItem(body, itemType, id)
	if err != nil {
		return err
	}
	var encoded string
	if err := json.Unmarshal(item[field], &encoded); err != nil {
		return fmt.Errorf("history item %s:%d field %s is not a timestamp: %w", itemType, id, field, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, encoded); err != nil {
		return fmt.Errorf("history item %s:%d field %s lacks a timezone: %q", itemType, id, field, encoded)
	}
	return nil
}

func (suite *testSuite) consumerHistoryLabel(item consumerHistoryItemResponse) string {
	for label, fixture := range suite.operationInbox.requests {
		if item.Type == "job_request" && fixture.id == item.ID {
			return label
		}
	}
	for label, fixture := range suite.operationInbox.proposals {
		if item.Type == "service_proposal" && fixture.id == item.ID {
			return label
		}
	}
	for label, id := range suite.operationInbox.orders {
		if item.Type == "work_order" && id == item.ID {
			return label
		}
	}
	return ""
}

func (suite *testSuite) consumerHistoryWorkOrderDatesArePersisted() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		if item.Type != "work_order" {
			continue
		}
		label := suite.consumerHistoryLabel(item)
		proposalLabel := ""
		for pLabel, p := range suite.operationInbox.proposals {
			if suite.operationInbox.orders[label] > 0 && p.id == item.ServiceProposalIDValue() {
				proposalLabel = pLabel
				break
			}
		}
		if proposalLabel == "" {
			return fmt.Errorf("could not resolve persisted order proposal fixture for %q", label)
		}
		persisted, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, suite.operationInbox.proposals[proposalLabel].id)
		if err != nil {
			return err
		}
		if persisted.ID() != item.ID {
			return fmt.Errorf("persisted work order ID %d differs from response ID %d", persisted.ID(), item.ID)
		}
		if item.AcceptedOn == nil || !item.AcceptedOn.Equal(persisted.AcceptedOn()) || !item.OccurredOn.Equal(persisted.AcceptedOn()) {
			return fmt.Errorf("work order accepted_on/occurred_on differs from persisted order")
		}
		if err := assertHistoryItemInstant(suite.lastBody, item.Type, item.ID, "accepted_on"); err != nil {
			return err
		}
		report := persisted.CompletionReport()
		if (item.CompletionReportedOn == nil) != (report == nil) || item.CompletionReportedOn != nil && !item.CompletionReportedOn.Equal(report.ReportedOn()) {
			return fmt.Errorf("work order completion time differs from persisted order")
		}
		if item.CompletionReportedOn != nil {
			if err := assertHistoryItemInstant(suite.lastBody, item.Type, item.ID, "completion_reported_on"); err != nil {
				return err
			}
		}
		if item.CompletionReportedOn == nil {
			if err := assertHistoryItemNullableKey(suite.lastBody, item.Type, item.ID, "completion_reported_on"); err != nil {
				return err
			}
		}
		paidOn := persisted.PaidOn()
		if (item.BalancePaidOn == nil) != paidOn.IsZero() || item.BalancePaidOn != nil && !item.BalancePaidOn.Equal(paidOn) {
			return fmt.Errorf("work order balance payment time differs from persisted order")
		}
		if item.BalancePaidOn == nil {
			if err := assertHistoryItemNullableKey(suite.lastBody, item.Type, item.ID, "balance_paid_on"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (item consumerHistoryItemResponse) ServiceProposalIDValue() int {
	if item.ServiceProposalID == nil {
		return 0
	}
	return *item.ServiceProposalID
}

func (suite *testSuite) consumerHistoryOrderCompletionAndNullPayment(label, completion string) error {
	item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
	if err != nil {
		return err
	}
	want, err := parseInboxInstant(completion)
	if err != nil {
		return err
	}
	if item.CompletionReportedOn == nil || !item.CompletionReportedOn.Equal(want) || item.BalancePaidOn != nil {
		return fmt.Errorf("order %q completion/payment evidence differs: %+v", label, item)
	}
	if err := assertHistoryItemNullableKey(suite.lastBody, item.Type, item.ID, "balance_paid_on"); err != nil {
		return err
	}
	return nil
}

func (suite *testSuite) consumerHistoryOrderDatesAreNull(label string) error {
	item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
	if err != nil {
		return err
	}
	if item.CompletionReportedOn != nil || item.BalancePaidOn != nil {
		return fmt.Errorf("order %q contains unpersisted completion/payment values: %+v", label, item)
	}
	if err := assertHistoryItemNullableKey(suite.lastBody, item.Type, item.ID, "completion_reported_on"); err != nil {
		return err
	}
	if err := assertHistoryItemNullableKey(suite.lastBody, item.Type, item.ID, "balance_paid_on"); err != nil {
		return err
	}
	return nil
}

func (suite *testSuite) consumerHistoryFindItemFromCurrentPage(label string) (consumerHistoryItemResponse, error) {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return consumerHistoryItemResponse{}, err
	}
	return suite.consumerHistoryFindItem(response.Page.Items, label)
}

func (suite *testSuite) consumerHistoryOmitsUnpersistedAcceptDates() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		switch item.Type {
		case "job_request", "service_proposal":
			raw, err := rawHistoryItem(suite.lastBody, item.Type, item.ID)
			if err != nil {
				return err
			}
			for _, field := range []string{"accepted_on", "request_accepted_on", "proposal_accepted_on"} {
				if _, exists := raw[field]; exists {
					return fmt.Errorf("%s exposes unsupported acceptance instant field %q", item.Type, field)
				}
			}
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryRequestOperationReference(label string) error {
	item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
	if err != nil {
		return err
	}
	fixture := suite.operationInbox.requests[label]
	return validateHistoryOperation(item, fmt.Sprintf("jr-%d", fixture.id))
}

func (suite *testSuite) consumerHistoryLinkedResourcesReferenceRequestOperation(first, second, third, requestLabel string) error {
	request := suite.operationInbox.requests[requestLabel]
	if request.id <= 0 {
		return fmt.Errorf("unknown request fixture %q", requestLabel)
	}
	for _, label := range []string{first, second, third} {
		item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
		if err != nil {
			return err
		}
		if err := validateHistoryOperation(item, fmt.Sprintf("jr-%d", request.id)); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryTwoLinkedResourcesReferenceRequestOperation(first, second, requestLabel string) error {
	request, ok := suite.operationInbox.requests[requestLabel]
	if !ok {
		return fmt.Errorf("unknown request fixture %q", requestLabel)
	}
	expected := fmt.Sprintf("jr-%d", request.id)
	for _, label := range []string{first, second} {
		item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
		if err != nil {
			return err
		}
		if err := validateHistoryOperation(item, expected); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryOrphanProposalsReferenceSelf(first, second string) error {
	for _, label := range []string{first, second} {
		proposal, ok := suite.operationInbox.proposals[label]
		if !ok {
			return fmt.Errorf("unknown proposal fixture %q", label)
		}
		item, err := suite.consumerHistoryFindItemFromCurrentPage(label)
		if err != nil {
			return err
		}
		if err := validateHistoryOperation(item, fmt.Sprintf("sp-%d", proposal.id)); err != nil {
			return err
		}
	}
	return nil
}

func validateHistoryOperation(item consumerHistoryItemResponse, expectedID string) error {
	op := item.Operation
	if op.ID != expectedID || op.URL != consumerHistoryOperationURL(expectedID) || op.RequiredPermission != "read:admin_operations" || op.ChatRequiredPermission != "read:admin_chat_audit" {
		return fmt.Errorf("operation reference for %s differs from contract: %+v", item.Type, op)
	}
	return nil
}

func consumerHistoryOperationURL(id string) string { return operationsInboxPath + "/" + id }

func (suite *testSuite) consumerHistoryHasNoHistoricalAddress() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	persisted, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(suite.consumerHistory.consumerEmail))
	if err != nil {
		return err
	}
	profile, ok := persisted.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("expected persisted consumer profile, got %T", persisted)
	}
	currentAddress := profile.Address()
	for _, item := range response.Page.Items {
		raw, err := rawHistoryItem(suite.lastBody, item.Type, item.ID)
		if err != nil {
			return err
		}
		for _, forbidden := range []string{"address", "consumer_address", "street", "street_number", "floor", "unit"} {
			if _, exists := raw[forbidden]; exists {
				return fmt.Errorf("history resource %s:%d incorrectly exposes address field %q", item.Type, item.ID, forbidden)
			}
		}
		for field, encoded := range raw {
			var value string
			if err := json.Unmarshal(encoded, &value); err == nil && value == currentAddress.Street {
				return fmt.Errorf("history response item echoes the current profile street in field %q", field)
			}
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryExcludesConsumer(email string) error {
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return err
	}
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	for _, item := range response.Page.Items {
		for label, fixture := range suite.operationInbox.requests {
			if fixture.id == item.ID && item.Type == "job_request" && fixture.consumerEmail == email {
				return fmt.Errorf("history includes foreign consumer request %q", label)
			}
		}
	}
	if consumerID == suite.consumerHistory.consumerID {
		return fmt.Errorf("foreign-consumer assertion refers to target consumer")
	}
	return nil
}

func (suite *testSuite) consumerHistoryPageContainsLabels(labels string) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	want := []string{}
	if strings.TrimSpace(labels) != "" && labels != "ninguno" {
		want = strings.Split(labels, ", ")
	}
	if len(response.Page.Items) != len(want) {
		return fmt.Errorf("expected exact page labels %v, got %d entries", want, len(response.Page.Items))
	}
	for i, label := range want {
		item, err := suite.consumerHistoryFindItem([]consumerHistoryItemResponse{response.Page.Items[i]}, label)
		if err != nil {
			return fmt.Errorf("page item %d is not expected resource %q: %+v", i, label, response.Page.Items[i])
		}
		_ = item
	}
	return nil
}

func (suite *testSuite) consumerHistoryPageHasCursor(limit int) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if response.Page.Limit != limit || response.Page.NextCursor == nil || strings.TrimSpace(*response.Page.NextCursor) == "" {
		return fmt.Errorf("expected page limit=%d and a nonempty next cursor, got %+v", limit, response.Page)
	}
	return nil
}

func (suite *testSuite) consumerHistoryPageHasAnyCursor() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if response.Page.NextCursor == nil || strings.TrimSpace(*response.Page.NextCursor) == "" {
		return fmt.Errorf("expected a nonempty next cursor")
	}
	return nil
}

func (suite *testSuite) consumerHistoryHasExactAllowlistedFields() error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &root); err != nil {
		return err
	}
	if err := requireRawObjectKeys("root", root, []string{"consumer", "summary", "page"}); err != nil {
		return err
	}
	var consumerRaw, summaryRaw, pageRaw map[string]json.RawMessage
	_ = json.Unmarshal(root["consumer"], &consumerRaw)
	_ = json.Unmarshal(root["summary"], &summaryRaw)
	_ = json.Unmarshal(root["page"], &pageRaw)
	if err := requireRawObjectKeys("consumer", consumerRaw, []string{"id", "role", "name", "surname", "email", "profile_photo_url", "created_on", "address", "coverage_zone"}); err != nil {
		return err
	}
	if err := requireRawObjectKeys("summary", summaryRaw, []string{"job_requests", "service_proposals", "work_orders"}); err != nil {
		return err
	}
	if err := requireRawObjectKeys("page", pageRaw, []string{"items", "limit", "next_cursor"}); err != nil {
		return err
	}
	if response.Consumer.Address != nil {
		var address map[string]json.RawMessage
		_ = json.Unmarshal(consumerRaw["address"], &address)
		if err := requireRawObjectKeys("consumer.address", address, []string{"street", "street_number", "floor", "unit", "source"}); err != nil {
			return err
		}
	}
	if response.Consumer.CoverageZone != nil {
		var zone map[string]json.RawMessage
		_ = json.Unmarshal(consumerRaw["coverage_zone"], &zone)
		if err := requireRawObjectKeys("consumer.coverage_zone", zone, []string{"id", "name", "enabled", "source"}); err != nil {
			return err
		}
	}
	for _, item := range response.Page.Items {
		raw, err := rawHistoryItem(suite.lastBody, item.Type, item.ID)
		if err != nil {
			return err
		}
		allowed := []string{"type", "id", "status", "provider", "occurred_on", "operation"}
		switch item.Type {
		case "job_request":
			allowed = append(allowed, "created_on")
		case "service_proposal":
			allowed = append(allowed, "job_request_id", "created_on", "scheduled_on", "estimated_duration_minutes", "booking_payment_deadline")
		case "work_order":
			allowed = append(allowed, "job_request_id", "service_proposal_id", "accepted_on", "completion_reported_on", "balance_paid_on")
		default:
			return fmt.Errorf("unexpected history item type %q", item.Type)
		}
		if err := requireRawObjectKeys(fmt.Sprintf("%s item %d", item.Type, item.ID), raw, allowed); err != nil {
			return err
		}
		var party, operation map[string]json.RawMessage
		_ = json.Unmarshal(raw["provider"], &party)
		_ = json.Unmarshal(raw["operation"], &operation)
		if err := requireRawObjectKeys("provider", party, []string{"id", "name", "surname"}); err != nil {
			return err
		}
		if err := requireRawObjectKeys("operation", operation, []string{"id", "url", "required_permission", "chat_required_permission"}); err != nil {
			return err
		}
	}
	return nil
}

func requireRawObjectKeys(name string, got map[string]json.RawMessage, expected []string) error {
	if len(got) != len(expected) {
		return fmt.Errorf("%s fields mismatch: expected %v, got %v", name, expected, rawKeys(got))
	}
	for _, key := range expected {
		if _, ok := got[key]; !ok {
			return fmt.Errorf("%s is missing field %q", name, key)
		}
	}
	return nil
}

func rawKeys(fields map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	return keys
}

func (suite *testSuite) consumerHistoryDoesNotExposeChatContent() error {
	request, exists := suite.operationInbox.requests["S1"]
	if !exists {
		return fmt.Errorf("chat exclusion fixture has no persisted S1 request")
	}
	conversation, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.conversationID)
	if err != nil {
		return err
	}
	messages := conversation.Messages()
	if len(messages) < 3 {
		return fmt.Errorf("chat exclusion fixture does not contain two participant messages and an image attachment")
	}
	consumerMessage, providerMessage, imageAttached := false, false, false
	imageFixture, imageExists := suite.messageImagesByName["adjunto-s1.jpg"]
	if !imageExists || imageFixture.FileID == "" {
		return fmt.Errorf("persisted attachment fixture adjunto-s1.jpg is unavailable")
	}
	for _, message := range messages {
		if message.Content == inboxPrivateMessage {
			if message.SenderRole == "consumer" {
				consumerMessage = true
			}
			if message.SenderRole == "provider" {
				providerMessage = true
			}
			if strings.Contains(string(suite.lastBody), message.Content) {
				return fmt.Errorf("consumer history response contains persisted chat message content")
			}
		}
		for _, image := range message.Images {
			if image.FileID == imageFixture.FileID {
				imageAttached = true
			}
		}
	}
	if !consumerMessage || !providerMessage || !imageAttached {
		return fmt.Errorf("chat fixture lacks persisted consumer/provider content or its confirmed image: consumer=%t provider=%t image=%t", consumerMessage, providerMessage, imageAttached)
	}
	if strings.Contains(string(suite.lastBody), imageFixture.FileID) {
		return fmt.Errorf("consumer history response contains prepared attachment ID")
	}
	return nil
}

func (suite *testSuite) consumerHistoryDoesNotExposeSensitiveData() error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &root); err != nil {
		return err
	}
	return ensureHistoryJSONNoSensitiveKeys(root)
}

func ensureHistoryJSONNoSensitiveKeys(value any) error {
	var visit func(any, string) error
	forbidden := map[string]struct{}{"auth_id": {}, "auth0_id": {}, "password": {}, "credentials": {}, "access_token": {}, "refresh_token": {}, "identity_document": {}, "document_number": {}, "document_url": {}, "private_file_ids": {}, "biometric_data": {}}
	visit = func(node any, path string) error {
		switch typed := node.(type) {
		case map[string]json.RawMessage:
			for key, raw := range typed {
				if _, banned := forbidden[strings.ToLower(key)]; banned {
					return fmt.Errorf("consumer history response exposes sensitive key %q at %s", key, path)
				}
				var child any
				if err := json.Unmarshal(raw, &child); err != nil {
					return err
				}
				if err := visit(child, path+"."+key); err != nil {
					return err
				}
			}
		case []any:
			for index, child := range typed {
				if err := visit(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(value, "response")
}

func (suite *testSuite) consumerHistoryDoesNotExposePaymentData() error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &root); err != nil {
		return err
	}
	for _, forbidden := range []string{"payment_transactions", "payment_intents", "transactions", "processor_payload", "mercado_pago", "access_token", "refresh_token"} {
		if strings.Contains(strings.ToLower(string(suite.lastBody)), forbidden) {
			return fmt.Errorf("consumer history response exposes forbidden payment/auth content %q", forbidden)
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryReferencesIndependentPermissions(detailPermission, chatPermission, _ string) error {
	response, err := suite.decodedConsumerHistory()
	if err != nil {
		return err
	}
	if len(response.Page.Items) == 0 {
		return fmt.Errorf("cannot verify operation permissions without history items")
	}
	for _, item := range response.Page.Items {
		if item.Operation.RequiredPermission != detailPermission || item.Operation.ChatRequiredPermission != chatPermission {
			return fmt.Errorf("operation permission references mismatch: %+v", item.Operation)
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryErrorHasNoData() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("error response is not JSON: %w", err)
	}
	if len(raw) == 0 || raw["error"] == nil {
		return fmt.Errorf("error response has no error: %s", suite.lastBody)
	}
	for key := range raw {
		if key != "error" && key != "message" {
			return fmt.Errorf("error response exposes unexpected field %q", key)
		}
	}
	return nil
}

func (suite *testSuite) consumerHistoryNoAccessEvent() error {
	correlation := suite.adminRequest.headers.Get("X-Request-ID")
	if strings.TrimSpace(correlation) == "" {
		return fmt.Errorf("response has no request correlation ID")
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	if total != 0 {
		return fmt.Errorf("rejected request correlation %q unexpectedly has %d audit events", correlation, total)
	}
	if suite.consumerHistoryCapture == nil {
		return fmt.Errorf("consumer history test capture is not configured")
	}
	if suite.consumerHistoryCapture.auditAttempts != 0 {
		return fmt.Errorf("rejected request attempted to persist an audit event")
	}
	return nil
}

func (suite *testSuite) consumerHistoryNoPersistedAuditForFailedRequest() error {
	if suite.consumerHistoryCapture == nil || suite.consumerHistoryCapture.lastEvent == nil {
		return fmt.Errorf("failed audit attempt was not captured")
	}
	correlation := suite.consumerHistoryCapture.lastEvent.CorrelationID()
	count, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("audit event unexpectedly persisted for failed request correlation %q", correlation)
	}
	if suite.consumerHistoryCapture.auditAttempts != 1 {
		return fmt.Errorf("expected exactly one failed audit write, got %d", suite.consumerHistoryCapture.auditAttempts)
	}
	return nil
}

func consumerHistoryRaw(body []byte) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func assertHistoryItemNullableKey(body []byte, itemType string, id int, field string) error {
	item, err := rawHistoryItem(body, itemType, id)
	if err != nil {
		return err
	}
	value, exists := item[field]
	if !exists {
		return fmt.Errorf("nullable item field %s is missing", field)
	}
	if string(value) != "null" {
		return fmt.Errorf("expected item field %s to be explicit null, got %s", field, value)
	}
	return nil
}

func assertHistoryNullableKey(body []byte, parentPath, field string, wantNull bool) error {
	var container map[string]json.RawMessage
	if err := json.Unmarshal(body, &container); err != nil {
		return err
	}
	for _, parent := range strings.Split(parentPath, ".") {
		if parent == "" {
			continue
		}
		var child map[string]json.RawMessage
		if err := json.Unmarshal(container[parent], &child); err != nil {
			return fmt.Errorf("decoding %s: %w", parentPath, err)
		}
		container = child
	}
	value, exists := container[field]
	if !exists {
		return fmt.Errorf("nullable JSON field %s.%s is missing", parentPath, field)
	}
	if wantNull && string(value) != "null" {
		return fmt.Errorf("expected %s.%s to be explicit null, got %s", parentPath, field, value)
	}
	if !wantNull && string(value) == "null" {
		return fmt.Errorf("expected %s.%s not to be null", parentPath, field)
	}
	return nil
}
func rawHistoryItem(body []byte, itemType string, id int) (map[string]json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(root["page"], &page); err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(page["items"], &items); err != nil {
		return nil, err
	}
	for _, rawItem := range items {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, err
		}
		var gotType string
		var gotID int
		_ = json.Unmarshal(item["type"], &gotType)
		_ = json.Unmarshal(item["id"], &gotID)
		if gotType == itemType && gotID == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("history item %s:%d not found in response", itemType, id)
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
