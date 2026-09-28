package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type diagnosticActivityResponse struct {
	Type       string    `json:"type"`
	ID         int       `json:"id"`
	Status     string    `json:"status"`
	OccurredOn time.Time `json:"occurred_on"`
}

type diagnosticReviewResponse struct {
	WorkOrderID int    `json:"work_order_id"`
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}

type diagnosticReputationResponse struct {
	Count   int     `json:"count"`
	Average float64 `json:"average"`
}

type diagnosticNavigationResponse struct {
	OperationsURL      string `json:"operations_url"`
	RequiredPermission string `json:"required_permission"`
}

type diagnosticOrderSyncResponse struct {
	WorkOrderID int        `json:"work_order_id"`
	SyncedOn    *time.Time `json:"synced_on"`
}

type calendarPublicationCountKey struct {
	workOrderID int
	userID      int
}

func registerAdminProviderDiagnosticActivitySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe una cuenta de Mercado Pago ni una conexión de Google Calendar para "([^\"]*)"$`, suite.providerDiagnosticHasNoPaymentOrCalendar)
	sc.Step(`^que no existe una cuenta de Mercado Pago para "([^\"]*)"$`, suite.providerDiagnosticHasNoPaymentAccount)
	sc.Step(`^identidad informa estado "([^\"]*)", resultado "([^\"]*)", reason_code "([^\"]*)" y evidencia nula$`, suite.providerDiagnosticIdentityWithoutEvidence)
	sc.Step(`^conexión de pago informa estado "([^\"]*)", resultado "([^\"]*)", reason_code "([^\"]*)" y evidencia nula$`, suite.providerDiagnosticPaymentWithoutEvidence)
	sc.Step(`^expiración del token informa resultado "([^\"]*)", reason_code "([^\"]*)" y evidencia nula$`, suite.providerDiagnosticExpiryWithoutEvidence)
	sc.Step(`^Calendar informa estado "([^\"]*)", resultado "([^\"]*)", reason_code "([^\"]*)" y evidencia nula$`, suite.providerDiagnosticCalendarWithoutEvidence)
	sc.Step(`^reputación informa cero reviews y promedio 0$`, suite.providerDiagnosticHasEmptyReputation)
	sc.Step(`^que "([^\"]*)" no tiene actividad ni reviews asociadas$`, suite.providerDiagnosticHasNoActivityOrReviews)
	sc.Step(`^el control de conexión de pagos informa "([^\"]*)" con reason_code "([^\"]*)"$`, suite.providerDiagnosticPaymentConnectionCheck)
	sc.Step(`^el control de expiración informa "([^\"]*)" con reason_code "([^\"]*)"$`, suite.providerDiagnosticPaymentExpiryCheck)
	sc.Step(`^la actividad incluye la solicitud "([^\"]*)" con estado "([^\"]*)"$`, suite.providerDiagnosticActivityIncludesRequest)
	sc.Step(`^la respuesta no incluye un campo ni un control que afirme que el prestador no puede recibir solicitudes$`, suite.providerDiagnosticDoesNotClaimRequestIneligibility)
	sc.Step(`^esa orden tiene evidencia persistida de sincronización para "([^\"]*)" el "([^\"]*)" y ninguna para "([^\"]*)"$`, suite.providerDiagnosticPersistsParticipantCalendarSync)
	sc.Step(`^el diagnóstico informa Calendar conectado y la orden sin sincronización confirmada para el prestador$`, suite.providerDiagnosticOrderIsNotSyncedForProvider)
	sc.Step(`^la evidencia de sincronización del consumidor no se atribuye al prestador$`, suite.providerDiagnosticConsumerSyncIsNotAttributed)
	sc.Step(`^la consulta no inicia ni reintenta sincronizaciones$`, suite.providerDiagnosticDoesNotPublishCalendarEvents)
	sc.Step(`^la actividad contiene como máximo 6 referencias en total y, para estos datos, en orden más reciente a más antigua:$`, suite.providerDiagnosticActivityMatchesTable)
	sc.Step(`^que la orden "([^\"]*)" tiene una review persistida con calificación (\d+) y descripción "([^\"]*)"$`, suite.providerDiagnosticPersistsReview)
	sc.Step(`^cada fila conserva una referencia tipada e ID estable distinto de sus entidades relacionadas; la solicitud antigua "([^\"]*)" queda fuera del límite$`, suite.providerDiagnosticActivityUsesStableReferences)
	sc.Step(`^la respuesta informa una review, cantidad (\d+) y promedio ([\d.]+), calculados sólo desde la review persistida de la orden pagada "([^\"]*)"$`, suite.providerDiagnosticReportsPersistedReputation)
	sc.Step(`^la orden "([^\"]*)" awaiting_payment no aparece como trabajo pagado ni contribuye al promedio de reviews$`, suite.providerDiagnosticUnpaidOrderIsNotReviewed)
	sc.Step(`^las referencias no incluyen detalle de operaciones ni chat, y la respuesta no contiene transacciones financieras completas o datos personales del consumidor$`, suite.providerDiagnosticActivityIsMinimized)
	sc.Step(`^actividad y reviews se informan como colecciones vacías no nulas$`, suite.providerDiagnosticCollectionsAreEmpty)
	sc.Step(`^reputación informa cantidad (\d+) y promedio ([\d.]+)$`, suite.providerDiagnosticReputationMatches)
}

func (suite *testSuite) providerDiagnosticIdentityWithoutEvidence(expectedState, expectedResult, expectedReason string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Provider.IdentityVerificationStatus != expectedState {
		return fmt.Errorf("expected identity state %q, got %q", expectedState, response.Provider.IdentityVerificationStatus)
	}
	return assertDiagnosticCheckWithoutEvidence(response.Checks, "identity_verification", expectedResult, expectedReason)
}

func (suite *testSuite) providerDiagnosticPaymentWithoutEvidence(expectedState, expectedResult, expectedReason string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Payment.State != expectedState {
		return fmt.Errorf("expected payment state %q, got %q", expectedState, response.Payment.State)
	}
	return assertDiagnosticCheckWithoutEvidence(response.Checks, "payment_account_connection", expectedResult, expectedReason)
}

func (suite *testSuite) providerDiagnosticExpiryWithoutEvidence(expectedResult, expectedReason string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Payment.TokenExpiresOn != nil {
		return fmt.Errorf("expected absent persisted token expiry, got %s", response.Payment.TokenExpiresOn.Format(time.RFC3339Nano))
	}
	return assertDiagnosticCheckWithoutEvidence(response.Checks, "payment_token_expiry", expectedResult, expectedReason)
}

func (suite *testSuite) providerDiagnosticCalendarWithoutEvidence(expectedState, expectedResult, expectedReason string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Calendar.State != expectedState {
		return fmt.Errorf("expected Calendar state %q, got %q", expectedState, response.Calendar.State)
	}
	return assertDiagnosticCheckWithoutEvidence(response.Checks, "calendar_connection", expectedResult, expectedReason)
}

func assertDiagnosticCheckWithoutEvidence(checks []providerDiagnosticCheckResponse, control, expectedResult, expectedReason string) error {
	check := findDiagnosticCheck(checks, control)
	if check == nil || check.Result != expectedResult || check.ReasonCode != expectedReason || check.EvidenceOn != nil {
		return fmt.Errorf("unexpected %s check or non-null evidence: %+v", control, check)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticHasNoPaymentOrCalendar(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	if err := suite.providerDiagnosticHasNoPaymentAccount(email); err != nil {
		return err
	}
	_, err = suite.calendarConnectionRepository.FindByUserID(suite.scenarioContext, providerID)
	if err == nil {
		return fmt.Errorf("provider %q unexpectedly has a Google Calendar connection", email)
	}
	if !errorsIsCalendarNotFound(err) {
		return fmt.Errorf("checking absent provider Calendar connection: %w", err)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticHasNoPaymentAccount(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	account, err := suite.paymentAccountRepository.FindByProviderID(suite.scenarioContext, providerID, "mercado_pago")
	if err != nil {
		if errors.Is(err, paymentaccount.ErrConnectionNotFound) {
			return nil
		}
		return fmt.Errorf("checking absent provider payment account: %w", err)
	}
	if account != nil {
		return fmt.Errorf("provider %q unexpectedly has a payment account", email)
	}
	return fmt.Errorf("payment account repository returned nil without a not-found error")
}

func errorsIsCalendarNotFound(err error) bool {
	return errors.Is(err, calendarconnection.ErrConnectionNotFound)
}

func (suite *testSuite) providerDiagnosticHasEmptyReputation() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Reputation.Count != 0 || response.Reputation.Average != 0 || response.Reviews == nil || len(response.Reviews) != 0 {
		return fmt.Errorf("expected no diagnostic reviews and zero reputation, got %+v and %+v", response.Reputation, response.Reviews)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticHasNoActivityOrReviews(email string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	counts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return fmt.Errorf("checking persisted business fixture is empty: %w", err)
	}
	for table, count := range counts {
		if count != 0 {
			return fmt.Errorf("expected no activity fixture rows, found %d in %s", count, table)
		}
	}
	stats, err := suite.workOrderRepository.FindRatingStatsByProviderID(suite.scenarioContext, providerID)
	if err != nil {
		return fmt.Errorf("checking persisted provider reviews: %w", err)
	}
	if stats.Count != 0 {
		return fmt.Errorf("expected no persisted provider reviews, got %d", stats.Count)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticPaymentConnectionCheck(result, reasonCode string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	check := findDiagnosticCheck(response.Checks, "payment_account_connection")
	if check == nil || check.Result != result || check.ReasonCode != reasonCode || response.Payment.State != "disconnected" {
		return fmt.Errorf("unexpected diagnostic payment connection state: %+v", check)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticPaymentExpiryCheck(result, reasonCode string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	check := findDiagnosticCheck(response.Checks, "payment_token_expiry")
	if check == nil || check.Result != result || check.ReasonCode != reasonCode || response.Payment.TokenExpiresOn != nil {
		return fmt.Errorf("unexpected diagnostic payment expiry state: %+v", check)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticActivityIncludesRequest(label, expectedStatus string) error {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request fixture %q", label)
	}
	activities, err := suite.diagnosticActivities()
	if err != nil {
		return err
	}
	for _, activity := range activities {
		if activity.Type == "job_request" && activity.ID == fixture.id && activity.Status == expectedStatus {
			return nil
		}
	}
	return fmt.Errorf("diagnostic activity does not contain job request %q in state %q", label, expectedStatus)
}

func (suite *testSuite) providerDiagnosticDoesNotClaimRequestIneligibility() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	allowedControls := map[string]bool{
		"identity_verification":      true,
		"payment_account_connection": true,
		"payment_token_expiry":       true,
		"calendar_connection":        true,
	}
	seenControls := make(map[string]bool, len(response.Checks))
	for _, check := range response.Checks {
		if !allowedControls[check.Control] || seenControls[check.Control] {
			return fmt.Errorf("diagnostic exposes unsupported or duplicate control %q", check.Control)
		}
		seenControls[check.Control] = true
	}
	if len(seenControls) != len(allowedControls) {
		return fmt.Errorf("expected exactly the four documented diagnostic controls, got %v", seenControls)
	}
	var body map[string]any
	if err := json.Unmarshal(suite.lastBody, &body); err != nil {
		return err
	}
	allowedFields := map[string]bool{
		"provider": true, "checks": true, "payment": true, "calendar": true,
		"activity": true, "reviews": true, "reputation": true, "navigation": true,
	}
	if len(body) != len(allowedFields) {
		return fmt.Errorf("expected only the eight documented diagnostic fields, got %v", body)
	}
	for key := range body {
		if !allowedFields[key] {
			return fmt.Errorf("diagnostic exposes undocumented field %q", key)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticPersistsParticipantCalendarSync(consumerEmail, syncedOnText, providerEmail string) error {
	serviceProposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, suite.lastServiceProposalID)
	if err != nil {
		return fmt.Errorf("finding scheduled diagnostic proposal: %w", err)
	}
	order, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, serviceProposal.ID)
	if err != nil {
		return fmt.Errorf("finding scheduled diagnostic work order: %w", err)
	}
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	consumerCalendar, err := suite.calendarConnectionRepository.FindByUserID(suite.scenarioContext, consumerID)
	if err != nil {
		if errorsIsCalendarNotFound(err) {
			oldAuthID := suite.currentAuth0ID
			defer func() { suite.currentAuth0ID = oldAuthID }()
			if err := suite.consumerAlreadyHasGoogleCalendarConnection(consumerEmail); err != nil {
				return fmt.Errorf("connecting consumer Calendar for persisted sync fixture: %w", err)
			}
			consumerCalendar, err = suite.calendarConnectionRepository.FindByUserID(suite.scenarioContext, consumerID)
		}
		if err != nil {
			return fmt.Errorf("finding consumer Calendar fixture: %w", err)
		}
	}
	if consumerCalendar.Status() != calendarconnection.StatusConnected {
		return fmt.Errorf("consumer Calendar must be connected to persist its sync evidence")
	}
	providerCalendar, err := suite.calendarConnectionRepository.FindByUserID(suite.scenarioContext, providerID)
	if err != nil || providerCalendar.Status() != calendarconnection.StatusConnected {
		return fmt.Errorf("provider Calendar fixture must be connected before asserting participant-specific sync: %v", err)
	}
	syncedOn, err := time.Parse(time.RFC3339Nano, syncedOnText)
	if err != nil {
		return fmt.Errorf("parsing persisted Calendar sync instant %q: %w", syncedOnText, err)
	}
	consumer, err := suite.userRepository.FindByID(suite.scenarioContext, consumerID)
	if err != nil {
		return err
	}
	event, err := workordercalendar.NewEvent(order, consumer)
	if err != nil {
		return fmt.Errorf("creating consumer Calendar event fixture: %w", err)
	}
	published, err := workordercalendar.NewPublishedEvent(consumerCalendar.CalendarID(), fmt.Sprintf("diagnostic-consumer-event-%d", order.ID()))
	if err != nil {
		return err
	}
	if err := event.MarkSynced(published, syncedOn); err != nil {
		return err
	}
	repository := suite.dependencies.Persistence.WorkOrderCalendarEventRepository
	if err := repository.Save(suite.scenarioContext, event); err != nil {
		return fmt.Errorf("persisting consumer Calendar event fixture: %w", err)
	}
	suite.providerDiagnostic.consumerCalendarSync = &syncedOn
	consumerKey, err := workordercalendar.NewEventKey(order.ID(), consumerID)
	if err != nil {
		return err
	}
	providerKey, err := workordercalendar.NewEventKey(order.ID(), providerID)
	if err != nil {
		return err
	}
	consumerExists, err := repository.Exists(suite.scenarioContext, consumerKey)
	if err != nil {
		return err
	}
	providerExists, err := repository.Exists(suite.scenarioContext, providerKey)
	if err != nil {
		return err
	}
	if !consumerExists || providerExists {
		return fmt.Errorf("expected persisted Calendar evidence only for consumer participant")
	}
	suite.providerDiagnostic.providerID = providerID
	return nil
}

func (suite *testSuite) snapshotProviderDiagnosticCalendarPublisherCounts() (map[calendarPublicationCountKey]int, error) {
	counter, ok := suite.calendarEventDetailsObserver.(calendarEventCountObserver)
	if !ok {
		return nil, fmt.Errorf("Calendar fake does not expose event publication counts")
	}
	orders, err := suite.workOrderRepository.FindScheduledAfter(suite.scenarioContext, suite.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("finding future orders before provider diagnostic request: %w", err)
	}
	counts := make(map[calendarPublicationCountKey]int, len(orders)*2)
	for _, order := range orders {
		for _, userID := range []int{order.ConsumerID(), order.ProviderID()} {
			count, err := counter.EventCountForUser(suite.scenarioContext, userID, order.ID())
			if err != nil {
				return nil, fmt.Errorf("reading Calendar publication count for order %d user %d: %w", order.ID(), userID, err)
			}
			counts[calendarPublicationCountKey{workOrderID: order.ID(), userID: userID}] = count
		}
	}
	return counts, nil
}

func (suite *testSuite) providerDiagnosticOrderIsNotSyncedForProvider() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Calendar.State != calendarconnection.StatusConnected {
		return fmt.Errorf("expected Calendar connection to remain connected, got %q", response.Calendar.State)
	}
	order, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, suite.lastServiceProposalID)
	if err != nil {
		return err
	}
	if response.Calendar.OrderSync == nil || len(response.Calendar.OrderSync) != 1 {
		return fmt.Errorf("expected exactly one Calendar sync row for scheduled order %d, got %+v", order.ID(), response.Calendar.OrderSync)
	}
	syncEvidence := response.Calendar.OrderSync[0]
	if syncEvidence.WorkOrderID != order.ID() {
		return fmt.Errorf("Calendar sync row references work order %d, want %d", syncEvidence.WorkOrderID, order.ID())
	}
	if syncEvidence.SyncedOn != nil {
		return fmt.Errorf("provider Calendar order sync was falsely reported: %+v", syncEvidence)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticConsumerSyncIsNotAttributed() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	consumerSyncedOn := suite.providerDiagnostic.consumerCalendarSync
	if consumerSyncedOn == nil {
		return fmt.Errorf("consumer Calendar sync fixture timestamp was not captured before diagnostic request")
	}
	order, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, suite.lastServiceProposalID)
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail("juan@example.com")
	if err != nil {
		return err
	}
	providerKey, err := workordercalendar.NewEventKey(order.ID(), providerID)
	if err != nil {
		return err
	}
	providerEventExists, err := suite.dependencies.Persistence.WorkOrderCalendarEventRepository.Exists(suite.scenarioContext, providerKey)
	if err != nil {
		return fmt.Errorf("checking persisted provider Calendar event after diagnostic request: %w", err)
	}
	if providerEventExists {
		return fmt.Errorf("diagnostic unexpectedly persisted a provider Calendar event for work order %d", order.ID())
	}
	for _, syncEvidence := range response.Calendar.OrderSync {
		if syncEvidence.SyncedOn != nil && syncEvidence.SyncedOn.Equal(*consumerSyncedOn) {
			return fmt.Errorf("consumer Calendar sync timestamp %s was attributed to provider", consumerSyncedOn.Format(time.RFC3339Nano))
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticDoesNotPublishCalendarEvents() error {
	before := suite.providerDiagnostic.calendarPublishCounts
	if before == nil {
		return fmt.Errorf("Calendar publisher counters were not captured before diagnostic request")
	}
	after, err := suite.snapshotProviderDiagnosticCalendarPublisherCounts()
	if err != nil {
		return err
	}
	if len(before) != len(after) {
		return fmt.Errorf("Calendar publisher participant set changed during read")
	}
	for key, count := range before {
		if after[key] != count {
			return fmt.Errorf("diagnostic triggered/retried Calendar publication for work order %d participant %d", key.workOrderID, key.userID)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticActivityMatchesTable(table *godog.Table) error {
	if err := requireTableHeaders(table, "tipo", "referencia", "estado", "instante de ordenamiento"); err != nil {
		return err
	}
	activities, err := suite.diagnosticActivities()
	if err != nil {
		return err
	}
	if len(activities) > readmodel.DiagnosticReferenceLimit || len(activities) != len(table.Rows)-1 {
		return fmt.Errorf("expected %d diagnostic activities within limit %d, got %d", len(table.Rows)-1, readmodel.DiagnosticReferenceLimit, len(activities))
	}
	for index, row := range table.Rows[1:] {
		if len(row.Cells) != 4 {
			return fmt.Errorf("expected four activity assertion columns")
		}
		kind, label, status, occurredText := row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, row.Cells[3].Value
		wantID, err := suite.providerDiagnosticActivityFixtureID(kind, label)
		if err != nil {
			return err
		}
		occurred, err := time.Parse(time.RFC3339Nano, occurredText)
		if err != nil {
			return fmt.Errorf("parsing expected activity instant %q: %w", occurredText, err)
		}
		got := activities[index]
		if got.Type != kind || got.ID != wantID || got.Status != status || !got.OccurredOn.Equal(occurred) {
			return fmt.Errorf("activity[%d]=%+v, expected type=%s id=%d status=%s occurred_on=%s", index, got, kind, wantID, status, occurredText)
		}
		if got.ID <= 0 || got.OccurredOn.IsZero() {
			return fmt.Errorf("activity row has invalid stable ID or sort instant: %+v", got)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticPersistsReview(label, ratingText, description string) error {
	orderID, ok := suite.operationInbox.orders[label]
	if !ok {
		return fmt.Errorf("unknown diagnostic review order %q", label)
	}
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}
	ctx := context.Background()
	order, err := suite.workOrderRepository.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("finding work order %q for review fixture: %w", label, err)
	}
	if order.Status() != workorder.StatusPaid {
		return fmt.Errorf("review fixture order %q must be paid, got %q", label, order.Status())
	}
	consumerUser, err := suite.userRepository.FindByID(ctx, order.ConsumerID())
	if err != nil {
		return fmt.Errorf("finding review author for order %q: %w", label, err)
	}
	consumerActor, ok := consumerUser.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("work order consumer has unexpected type %T", consumerUser)
	}
	review, err := workorder.NewReview(rating, description)
	if err != nil {
		return fmt.Errorf("creating review fixture: %w", err)
	}
	if err := order.AddReview(consumerActor, review); err != nil {
		return fmt.Errorf("adding review to paid work order %q: %w", label, err)
	}
	if _, err := suite.workOrderRepository.Save(ctx, order); err != nil {
		return fmt.Errorf("persisting review for work order %q: %w", label, err)
	}
	persisted, err := suite.workOrderRepository.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("verifying persisted review for work order %q: %w", label, err)
	}
	if persisted.Review() == nil || persisted.Review().Rating() != rating || persisted.Review().Description() != description {
		return fmt.Errorf("review for work order %q was not persisted as supplied", label)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticActivityFixtureID(kind, label string) (int, error) {
	switch kind {
	case "job_request":
		fixture, ok := suite.operationInbox.requests[label]
		if !ok {
			return 0, fmt.Errorf("unknown job request activity fixture %q", label)
		}
		return fixture.id, nil
	case "service_proposal":
		fixture, ok := suite.operationInbox.proposals[label]
		if !ok {
			return 0, fmt.Errorf("unknown proposal activity fixture %q", label)
		}
		return fixture.id, nil
	case "work_order":
		id, ok := suite.operationInbox.orders[label]
		if !ok {
			return 0, fmt.Errorf("unknown order activity fixture %q", label)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unsupported diagnostic activity type %q", kind)
	}
}

func (suite *testSuite) providerDiagnosticActivityUsesStableReferences(excludedRequest string) error {
	activities, err := suite.diagnosticActivities()
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(activities))
	for _, activity := range activities {
		key := fmt.Sprintf("%s:%d", activity.Type, activity.ID)
		if activity.ID <= 0 || seen[key] {
			return fmt.Errorf("diagnostic activity reference is missing or duplicated: %s", key)
		}
		seen[key] = true
	}
	oldRequest, ok := suite.operationInbox.requests[excludedRequest]
	if !ok {
		return fmt.Errorf("unknown excluded request fixture %q", excludedRequest)
	}
	for _, activity := range activities {
		if activity.Type == "job_request" && activity.ID == oldRequest.id {
			return fmt.Errorf("old request %q is outside the six-reference window but appeared", excludedRequest)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticReportsPersistedReputation(countText, averageText, paidOrderLabel string) error {
	count, err := strconv.Atoi(countText)
	if err != nil {
		return err
	}
	average, err := strconv.ParseFloat(averageText, 64)
	if err != nil {
		return err
	}
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(response.Provider.Email)
	if err != nil {
		return err
	}
	persisted, err := suite.workOrderRepository.FindRatingStatsByProviderID(suite.scenarioContext, providerID)
	if err != nil {
		return fmt.Errorf("reading persisted provider rating stats: %w", err)
	}
	summary := persisted.Summary()
	if response.Reputation.Count != count || response.Reputation.Average != average || summary.Count != count || summary.Average != average {
		return fmt.Errorf("diagnostic reputation %+v differs from persisted rating summary %+v", response.Reputation, summary)
	}
	orderID, ok := suite.operationInbox.orders[paidOrderLabel]
	if !ok {
		return fmt.Errorf("unknown paid review order %q", paidOrderLabel)
	}
	if len(response.Reviews) != count {
		return fmt.Errorf("expected %d persisted reviews, got %d", count, len(response.Reviews))
	}
	paidOrder, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return fmt.Errorf("reading persisted paid work order review %q: %w", paidOrderLabel, err)
	}
	persistedReview := paidOrder.Review()
	if persistedReview == nil {
		return fmt.Errorf("paid work order %q has no persisted review", paidOrderLabel)
	}
	for _, review := range response.Reviews {
		if review.WorkOrderID != orderID || review.Rating != persistedReview.Rating() || review.Description != persistedReview.Description() {
			return fmt.Errorf("diagnostic review does not match persisted paid order %q: %+v", paidOrderLabel, review)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticUnpaidOrderIsNotReviewed(label string) error {
	orderID, ok := suite.operationInbox.orders[label]
	if !ok {
		return fmt.Errorf("unknown unpaid order %q", label)
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return err
	}
	if order.Status() != workorder.StatusAwaitingPayment {
		return fmt.Errorf("expected order %q to remain awaiting payment, got %q", label, order.Status())
	}
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	for _, review := range response.Reviews {
		if review.WorkOrderID == orderID {
			return fmt.Errorf("awaiting-payment order %q contributed an unearned review", label)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticActivityIsMinimized() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return err
	}
	var activity []map[string]json.RawMessage
	if err := json.Unmarshal(raw["activity"], &activity); err != nil {
		return err
	}
	for _, row := range activity {
		if len(row) != 4 {
			return fmt.Errorf("activity reference contains fields beyond type/id/status/occurred_on: %v", row)
		}
	}
	for _, field := range []string{"transactions", "payment_transactions", "messages", "conversation", "consumer", "consumers", "chat", "payment_intents"} {
		if _, exists := raw[field]; exists {
			return fmt.Errorf("diagnostic response exposes forbidden activity detail %q", field)
		}
	}
	body := string(suite.lastBody)
	for _, email := range []string{"ana@example.com", "beatriz@example.com"} {
		if strings.Contains(body, email) {
			return fmt.Errorf("diagnostic response exposes consumer email %q", email)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticCollectionsAreEmpty() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Activity == nil || len(response.Activity) != 0 || response.Reviews == nil || len(response.Reviews) != 0 {
		return fmt.Errorf("expected non-null empty activity and reviews collections")
	}
	return nil
}

func (suite *testSuite) providerDiagnosticReputationMatches(countText, averageText string) error {
	count, err := strconv.Atoi(countText)
	if err != nil {
		return err
	}
	average, err := strconv.ParseFloat(averageText, 64)
	if err != nil {
		return err
	}
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Reputation.Count != count || math.Abs(response.Reputation.Average-average) > 0.000001 {
		return fmt.Errorf("expected reputation count/average %d/%.2f, got %d/%.2f", count, average, response.Reputation.Count, response.Reputation.Average)
	}
	return nil
}

func (suite *testSuite) diagnosticActivities() ([]diagnosticActivityResponse, error) {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return nil, err
	}
	if response.Activity == nil {
		return nil, fmt.Errorf("diagnostic activity collection is null")
	}
	return response.Activity, nil
}
