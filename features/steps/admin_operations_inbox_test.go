package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const operationsInboxPath = "/admin/operations"

type operationInboxState struct {
	requests         map[string]inboxJobRequestFixture
	proposals        map[string]inboxProposalFixture
	orders           map[string]int
	recordedIDs      map[string]string
	acceptedProposal map[string]bool
	auditWatermark   *int64
	startWindow      *[2]time.Time
	pages            []inboxPageResponse
}

type inboxJobRequestFixture struct {
	createdOn      time.Time
	id             int
	conversationID int
	consumerEmail  string
	providerEmail  string
}

type inboxProposalFixture struct {
	createdOn    time.Time
	id           int
	requestLabel string
}

type inboxPageResponse struct {
	Operations []inboxOperationResponse `json:"operations"`
	NextCursor *string                  `json:"next_cursor"`
}

type inboxOperationResponse struct {
	ID                    string                 `json:"id"`
	Stage                 string                 `json:"stage"`
	StartedOn             time.Time              `json:"started_on"`
	JobRequest            *inboxResourceResponse `json:"job_request"`
	ServiceProposal       *inboxResourceResponse `json:"service_proposal"`
	WorkOrder             *inboxResourceResponse `json:"work_order"`
	Consumer              inboxPartyResponse     `json:"consumer"`
	Provider              inboxPartyResponse     `json:"provider"`
	Category              *inboxCategoryResponse `json:"category"`
	Alerts                []string               `json:"alerts"`
	NextActionOwner       *string                `json:"next_action_owner"`
	LastBusinessAdvanceOn *time.Time             `json:"last_business_advance_on"`
	Limitations           []string               `json:"limitations"`
}

type inboxResourceResponse struct {
	ID                       int        `json:"id"`
	Status                   string     `json:"status"`
	CreatedOn                *time.Time `json:"created_on"`
	ScheduledOn              *time.Time `json:"scheduled_on"`
	AcceptedOn               *time.Time `json:"accepted_on"`
	CompletionReportedOn     *time.Time `json:"completion_reported_on"`
	BalancePaidOn            *time.Time `json:"balance_paid_on"`
	EstimatedDurationMinutes int        `json:"estimated_duration_minutes"`
}

type inboxPartyResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type inboxCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var inboxAllowedFields = map[string][]string{
	"operation": {"id", "stage", "started_on", "job_request", "service_proposal", "work_order", "consumer", "provider", "category",
		"alerts", "next_action_owner", "last_business_advance_on", "limitations"},
	"job_request":      {"id", "status", "created_on"},
	"service_proposal": {"id", "status", "created_on", "scheduled_on", "estimated_duration_minutes", "booking_payment_deadline"},
	"work_order":       {"id", "status", "accepted_on", "completion_reported_on", "balance_paid_on"},
	"consumer":         {"id", "name", "surname"},
	"provider":         {"id", "name", "surname"},
	"category":         {"id", "name"},
}

const (
	inboxPrivateMessage              = "Mensaje privado de la conversación"
	inboxCompletionDescriptionPrefix = "Evidencia privada de finalización"
)

func registerAdminOperationsInboxSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen las siguientes solicitudes de trabajo:$`, suite.thereAreInboxJobRequests)
	sc.Step(`^que existen las siguientes propuestas de servicio:$`, suite.thereAreInboxServiceProposals)
	sc.Step(`^que existen las siguientes órdenes de trabajo:$`, suite.thereAreInboxWorkOrders)
	sc.Step(`^que la seña de "([^"]*)" tuvo un intento de pago rechazado antes del intento aprobado$`, suite.inboxProposalHadRejectedThenApprovedDeposit)
	sc.Step(`^que no existen solicitudes de trabajo$`, suite.thereAreNoInboxJobRequests)
	sc.Step(`^que consulté la bandeja y registré el identificador de la operación "([^"]*)"$`, suite.recordInboxOperationID)
	sc.Step(`^que luego "([^"]*)" fue aceptada, recibió la propuesta "([^"]*)" y la seña de "([^"]*)" fue aprobada generando la orden "([^"]*)"$`, suite.inboxRequestAdvancedToWorkOrder)
	sc.Step(`^consulto la bandeja administrativa de contrataciones$`, suite.queryOperationsInbox)
	sc.Step(`^intento consultar la bandeja administrativa de contrataciones$`, suite.queryOperationsInbox)
	sc.Step(`^la bandeja contiene exactamente las siguientes operaciones:$`, suite.inboxContainsExactlyOperations)
	sc.Step(`^cada referencia ausente se informa explícitamente como nula$`, suite.inboxAbsentReferencesAreExplicitNulls)
	sc.Step(`^la operación "([^"]*)" conserva el identificador registrado$`, suite.inboxOperationKeepsRecordedID)
	sc.Step(`^las operaciones "([^"]*)" y "([^"]*)" tienen identificadores distintos$`, suite.inboxOperationsHaveDistinctIDs)
	sc.Step(`^la respuesta no contiene operaciones$`, suite.inboxErrorResponseHasNoOperations)
	sc.Step(`^que "([^"]*)" tiene una imagen adjunta$`, suite.inboxConversationHasImageAttachment)
	sc.Step(`^que la conversación de "([^"]*)" tiene mensajes entre "([^"]*)" y "([^"]*)"$`, suite.inboxConversationHasMessagesBetween)
	sc.Step(`^la operación "([^"]*)" informa al consumidor "([^"]*)" y al prestador "([^"]*)" con sus identificadores$`, suite.inboxOperationInformsParties)
	sc.Step(`^la operación "([^"]*)" informa el rubro "([^"]*)"$`, suite.inboxOperationInformsCategory)
	sc.Step(`^la operación "([^"]*)" informa los estados de dominio "([^"]*)" de la solicitud, "([^"]*)" de la propuesta y "([^"]*)" de la orden$`, suite.inboxOperationInformsDomainStatuses)
	sc.Step(`^la operación "([^"]*)" informa las siguientes fechas con zona horaria explícita:$`, suite.inboxOperationInformsDates)
	sc.Step(`^la operación "([^"]*)" informa explícitamente como nula la fecha de pago del saldo$`, suite.inboxOperationHasNullBalancePayment)
	sc.Step(`^la respuesta no expone mensajes, extractos del chat, adjuntos, credenciales, biometría ni payloads de pagos$`, suite.inboxResponseIsMinimized)
	sc.Step(`^no se registra ningún evento de auditoría$`, suite.noAuditEventIsRecorded)
	sc.Step(`^que existe una operación entre "([^"]*)" y "([^"]*)" con (.+)$`, suite.thereIsInboxOperationInSituation)
	sc.Step(`^la operación entre "([^"]*)" y "([^"]*)" informa que el responsable de la siguiente acción es (.+)$`, suite.inboxOperationBetweenHasNextActionOwner)
}

func (state *operationInboxState) ensureMaps() {
	if state.requests == nil {
		state.requests = map[string]inboxJobRequestFixture{}
		state.proposals = map[string]inboxProposalFixture{}
		state.orders = map[string]int{}
		state.recordedIDs = map[string]string{}
		state.acceptedProposal = map[string]bool{}
	}
}

func (suite *testSuite) withInboxFixtureClock(create func() error) error {
	now := suite.clock.Now()
	auth0ID, permissions := suite.currentAuth0ID, suite.currentPermissions
	createErr := create()
	suite.currentAuth0ID, suite.currentPermissions = auth0ID, permissions
	if err := suite.requestTestClockMock(now.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return createErr
}

func inboxTableRows(table *godog.Table) ([]map[string]string, error) {
	if table == nil || len(table.Rows) < 2 {
		return nil, fmt.Errorf("expected a table with a header and at least one row")
	}
	header := table.Rows[0].Cells
	rows := make([]map[string]string, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != len(header) {
			return nil, fmt.Errorf("table row has %d cells, header has %d", len(row.Cells), len(header))
		}
		values := make(map[string]string, len(header))
		for index, cell := range row.Cells {
			values[header[index].Value] = strings.TrimSpace(cell.Value)
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func parseInboxInstant(value string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing instant %q: %w", value, err)
	}
	return instant.UTC(), nil
}

func (suite *testSuite) setInboxFixtureClock(instant time.Time) error {
	return suite.requestTestClockMock(instant.Format(time.RFC3339Nano))
}

func (suite *testSuite) thereAreInboxJobRequests(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			createdOn, err := parseInboxInstant(row["creada"])
			if err != nil {
				return err
			}
			if err := suite.createInboxJobRequest(row["solicitud"], row["consumidor"], row["prestador"], createdOn, row["estado"]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (suite *testSuite) createInboxJobRequest(label, consumerEmail, providerEmail string, createdOn time.Time, status string) error {
	if _, exists := suite.operationInbox.requests[label]; exists {
		return fmt.Errorf("duplicate job request label %q", label)
	}
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	if err := suite.setInboxFixtureClock(createdOn); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.requestJobRequest(jobRequestCreationRequest{
		ProviderID:  providerID,
		Title:       "Solicitud " + label,
		Description: "Solicitud preparada para la bandeja administrativa",
	}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating job request %q returned %d: %s", label, suite.lastStatus, suite.lastBody)
	}
	created, err := suite.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	suite.operationInbox.requests[label] = inboxJobRequestFixture{
		createdOn: createdOn, id: created.ID, conversationID: created.ConversationID,
		consumerEmail: consumerEmail, providerEmail: providerEmail,
	}
	switch status {
	case "pending":
		return nil
	case "accepted":
		return suite.acceptInboxJobRequest(label)
	default:
		return fmt.Errorf("unsupported job request status %q", status)
	}
}

func (suite *testSuite) acceptInboxJobRequest(label string) error {
	request := suite.operationInbox.requests[label]
	suite.lastJobRequestID = request.id
	suite.currentAuth0ID = auth0IDForProviderEmail(request.providerEmail)
	if err := suite.requestAcceptPendingJobRequest(); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("accepting job request %q returned %d: %s", label, suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) thereAreInboxServiceProposals(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			createdOn, err := parseInboxInstant(row["creada"])
			if err != nil {
				return err
			}
			scheduledOn, err := parseInboxInstant(row["fecha programada"])
			if err != nil {
				return err
			}
			duration, err := strconv.Atoi(row["duración"])
			if err != nil {
				return fmt.Errorf("parsing duration of %q: %w", row["propuesta"], err)
			}
			if err := suite.createInboxServiceProposal(row["propuesta"], row["solicitud"], createdOn, scheduledOn, duration, row["estado"]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (suite *testSuite) createInboxServiceProposal(label, requestLabel string, createdOn, scheduledOn time.Time, duration int, status string) error {
	request, exists := suite.operationInbox.requests[requestLabel]
	if !exists {
		return fmt.Errorf("unknown job request label %q", requestLabel)
	}
	if _, exists := suite.operationInbox.proposals[label]; exists {
		return fmt.Errorf("duplicate proposal label %q", label)
	}
	ctx := context.Background()
	foundProvider, err := suite.userRepository.FindByAuthID(auth0IDForProviderEmail(request.providerEmail))
	if err != nil {
		return fmt.Errorf("finding provider of %q: %w", label, err)
	}
	proposalProvider, ok := foundProvider.(*provider.Provider)
	if !ok {
		return fmt.Errorf("expected provider for %q, got %T", request.providerEmail, foundProvider)
	}
	foundConsumer, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(request.consumerEmail))
	if err != nil {
		return fmt.Errorf("finding consumer of %q: %w", label, err)
	}
	proposalConsumer, ok := foundConsumer.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("expected consumer for %q, got %T", request.consumerEmail, foundConsumer)
	}
	proposalConversation, err := suite.conversationRepository.FindByID(ctx, request.conversationID)
	if err != nil {
		return fmt.Errorf("finding conversation of %q: %w", label, err)
	}
	bookingTerms, err := serviceproposal.NewBookingPolicy().Calculate(defaultServiceProposalAmount, scheduledOn)
	if err != nil {
		return fmt.Errorf("calculating booking terms of %q: %w", label, err)
	}
	if err := suite.setInboxFixtureClock(createdOn); err != nil {
		return err
	}
	proposal, err := serviceproposal.NewServiceProposal(
		proposalProvider, proposalConsumer, proposalConversation, scheduledOn,
		"Propuesta "+label, bookingTerms, suite.clock, duration,
	)
	if err != nil {
		return fmt.Errorf("creating proposal %q: %w", label, err)
	}
	switch status {
	case string(serviceproposal.StatusPending), string(serviceproposal.StatusAccepted):
		// Accepted proposals are accepted when their work order is created.
	case string(serviceproposal.StatusRejected):
		proposal.Status = serviceproposal.StatusRejected
	default:
		return fmt.Errorf("unsupported proposal status %q", status)
	}
	saved, err := repositories.NewServiceProposalRepository(suite.database).Save(proposal)
	if err != nil {
		return err
	}
	suite.operationInbox.proposals[label] = inboxProposalFixture{createdOn: createdOn, id: saved.ID, requestLabel: requestLabel}
	suite.operationInbox.acceptedProposal[label] = status == string(serviceproposal.StatusAccepted)
	return nil
}

func (suite *testSuite) thereAreInboxWorkOrders(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			acceptedOn, err := parseInboxInstant(row["aceptada"])
			if err != nil {
				return err
			}
			if err := suite.createInboxWorkOrder(row["orden"], row["propuesta"], acceptedOn, row["estado"], row["finalización informada"], row["saldo pagado"]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (suite *testSuite) createInboxWorkOrder(label, proposalLabel string, acceptedOn time.Time, status, reportedOnValue, paidOnValue string) error {
	proposalFixture, exists := suite.operationInbox.proposals[proposalLabel]
	if !exists {
		return fmt.Errorf("unknown proposal label %q", proposalLabel)
	}
	if !suite.operationInbox.acceptedProposal[proposalLabel] {
		return fmt.Errorf("proposal %q must be declared accepted to have work order %q", proposalLabel, label)
	}
	ctx := context.Background()
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(ctx, proposalFixture.id)
	if err != nil {
		return fmt.Errorf("finding proposal %q: %w", proposalLabel, err)
	}
	if err := proposal.Accept(proposal.Consumer.ID(), acceptedOn); err != nil {
		return fmt.Errorf("accepting proposal %q: %w", proposalLabel, err)
	}
	order, err := workorder.New(proposal, acceptedOn)
	if err != nil {
		return fmt.Errorf("creating work order %q: %w", label, err)
	}
	unitOfWork := repositories.NewPaymentUnitOfWork(
		suite.database, suite.paymentIntentRepository, suite.paymentTransactionRepository,
		repositories.NewServiceProposalRepository(suite.database), suite.workOrderRepository, suite.notificationRepository,
	)
	if err := unitOfWork.Execute(ctx, func(store payment.TransactionalStore) error {
		if err := store.SaveServiceProposal(ctx, proposal); err != nil {
			return err
		}
		return store.SaveWorkOrder(ctx, order)
	}); err != nil {
		return fmt.Errorf("saving work order %q: %w", label, err)
	}
	persisted, err := suite.workOrderRepository.FindByServiceProposalID(ctx, proposalFixture.id)
	if err != nil {
		return err
	}
	suite.operationInbox.orders[label] = persisted.ID()

	switch status {
	case string(workorder.StatusScheduled):
		return nil
	case string(workorder.StatusAwaitingPayment), string(workorder.StatusPaid):
	default:
		return fmt.Errorf("unsupported work order status %q", status)
	}
	reportedOn, err := parseInboxInstant(reportedOnValue)
	if err != nil {
		return fmt.Errorf("work order %q needs its completion time: %w", label, err)
	}
	if err := suite.reportInboxWorkOrderCompletion(label, persisted, reportedOn, 1); err != nil {
		return err
	}
	if status == string(workorder.StatusAwaitingPayment) {
		return nil
	}
	paidOn, err := parseInboxInstant(paidOnValue)
	if err != nil {
		return fmt.Errorf("work order %q needs its balance payment time: %w", label, err)
	}
	reported, err := suite.workOrderRepository.FindByID(ctx, persisted.ID())
	if err != nil {
		return err
	}
	if err := reported.RegisterApprovedBalancePayment(paidOn); err != nil {
		return fmt.Errorf("registering balance payment of %q: %w", label, err)
	}
	_, err = suite.workOrderRepository.Save(ctx, reported)
	return err
}

func (suite *testSuite) reportInboxWorkOrderCompletion(label string, order *workorder.WorkOrder, reportedOn time.Time, imageCount int) error {
	providerAuthID, err := suite.authIDForUserID(order.ProviderID())
	if err != nil {
		return err
	}
	if err := suite.setInboxFixtureClock(reportedOn); err != nil {
		return err
	}
	suite.currentAuth0ID = providerAuthID
	fileIDs := make([]string, 0, imageCount)
	for index := 1; index <= imageCount; index++ {
		imageName := fmt.Sprintf("bandeja-%s-%d.jpg", strings.ToLower(label), index)
		if err := suite.uploadAndConfirmCompletionImage(imageName); err != nil {
			return fmt.Errorf("preparing completion image of %q: %w", label, err)
		}
		fileIDs = append(fileIDs, suite.completionImagesByName[imageName].FileID)
	}
	report, err := workorder.NewCompletionReport(inboxCompletionDescriptionPrefix+" "+label, fileIDs, reportedOn)
	if err != nil {
		return err
	}
	if err := order.ReportCompletion(order.ProviderID(), report); err != nil {
		return fmt.Errorf("reporting completion of %q: %w", label, err)
	}
	_, err = suite.workOrderRepository.Save(context.Background(), order)
	return err
}

func (suite *testSuite) authIDForUserID(userID int) (string, error) {
	found, err := suite.userRepository.FindByID(context.Background(), userID)
	if err != nil {
		return "", fmt.Errorf("finding user %d: %w", userID, err)
	}
	return found.AuthID(), nil
}

func (suite *testSuite) inboxProposalHadRejectedThenApprovedDeposit(proposalLabel string) error {
	proposalFixture, exists := suite.operationInbox.proposals[proposalLabel]
	if !exists {
		return fmt.Errorf("unknown proposal label %q", proposalLabel)
	}
	ctx := context.Background()
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(ctx, proposalFixture.id)
	if err != nil {
		return err
	}
	now := proposal.CreatedOn
	for _, approved := range []bool{false, true} {
		intent, err := payment.NewBookingDepositIntent(uuid.NewString(), proposal.ID, proposal.BookingTerms, now)
		if err != nil {
			return err
		}
		if err := intent.MarkCheckoutReady("pref-"+intent.ID, "https://checkout.test/"+intent.ID, now.Add(30*time.Minute), now); err != nil {
			return err
		}
		externalPayment := payment.ExternalPayment{
			ID: "mp-" + intent.ID, SellerAccountID: "mp-inbox", ExternalReference: intent.ID,
			Status: payment.ExternalPaymentStatusRejected, Currency: intent.Currency, AmountCents: intent.TotalAmountCents,
		}
		if approved {
			externalPayment.Status = payment.ExternalPaymentStatusApproved
			err = intent.MarkPaid(externalPayment, now)
		} else {
			err = intent.MarkRejected(externalPayment, now)
		}
		if err != nil {
			return err
		}
		if err := suite.paymentIntentRepository.Save(ctx, intent); err != nil {
			return err
		}
		now = now.Add(time.Minute)
	}
	return nil
}

func (suite *testSuite) thereAreNoInboxJobRequests() error {
	if len(suite.operationInbox.requests) != 0 {
		return fmt.Errorf("scenario already prepared job requests")
	}
	return nil
}

func (suite *testSuite) queryOperationsInbox() error {
	return suite.queryOperationsInboxWith(nil)
}

func (suite *testSuite) queryOperationsInboxWith(query url.Values) error {
	watermark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return fmt.Errorf("capturing audit ingest watermark: %w", err)
	}
	suite.operationInbox.auditWatermark = &watermark
	return suite.sendAdminGet(operationsInboxPath, query, "")
}

func (suite *testSuite) decodedInboxPage() (inboxPageResponse, error) {
	if suite.lastStatus != http.StatusOK {
		return inboxPageResponse{}, fmt.Errorf("expected inbox status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var page inboxPageResponse
	if err := json.Unmarshal(suite.lastBody, &page); err != nil {
		return inboxPageResponse{}, fmt.Errorf("invalid inbox page: %w", err)
	}
	return page, nil
}

func (suite *testSuite) inboxOperationID(label string) (string, error) {
	if request, exists := suite.operationInbox.requests[label]; exists {
		return fmt.Sprintf("jr-%d", request.id), nil
	}
	if proposal, exists := suite.operationInbox.proposals[label]; exists {
		return fmt.Sprintf("sp-%d", proposal.id), nil
	}
	return "", fmt.Errorf("unknown operation label %q", label)
}

func (suite *testSuite) inboxResourceID(label string, ids func(string) (int, bool)) (*int, error) {
	if label == "" {
		return nil, nil
	}
	id, exists := ids(label)
	if !exists {
		return nil, fmt.Errorf("unknown resource label %q", label)
	}
	return &id, nil
}

func inboxResourceIDOf(resource *inboxResourceResponse) *int {
	if resource == nil {
		return nil
	}
	return &resource.ID
}

func formatOptionalID(id *int) string {
	if id == nil {
		return "null"
	}
	return strconv.Itoa(*id)
}

func (suite *testSuite) inboxContainsExactlyOperations(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	expected := make([]string, 0, len(rows))
	for _, row := range rows {
		operationID, err := suite.inboxOperationID(row["operación"])
		if err != nil {
			return err
		}
		requestID, err := suite.inboxResourceID(row["solicitud"], func(label string) (int, bool) {
			request, exists := suite.operationInbox.requests[label]
			return request.id, exists
		})
		if err != nil {
			return err
		}
		proposalID, err := suite.inboxResourceID(row["propuesta"], func(label string) (int, bool) {
			proposal, exists := suite.operationInbox.proposals[label]
			return proposal.id, exists
		})
		if err != nil {
			return err
		}
		orderID, err := suite.inboxResourceID(row["orden"], func(label string) (int, bool) {
			id, exists := suite.operationInbox.orders[label]
			return id, exists
		})
		if err != nil {
			return err
		}
		expected = append(expected, fmt.Sprintf("%s request=%s proposal=%s order=%s stage=%s",
			operationID, formatOptionalID(requestID), formatOptionalID(proposalID), formatOptionalID(orderID), row["etapa"]))
	}
	actual := make([]string, 0, len(page.Operations))
	for _, found := range page.Operations {
		actual = append(actual, fmt.Sprintf("%s request=%s proposal=%s order=%s stage=%s",
			found.ID, formatOptionalID(inboxResourceIDOf(found.JobRequest)), formatOptionalID(inboxResourceIDOf(found.ServiceProposal)),
			formatOptionalID(inboxResourceIDOf(found.WorkOrder)), found.Stage))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if strings.Join(expected, "\n") != strings.Join(actual, "\n") {
		return fmt.Errorf("unexpected inbox operations\nexpected:\n%s\nactual:\n%s", strings.Join(expected, "\n"), strings.Join(actual, "\n"))
	}
	return nil
}

func (suite *testSuite) inboxAbsentReferencesAreExplicitNulls() error {
	var raw struct {
		Operations []map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("invalid inbox page: %w", err)
	}
	if len(raw.Operations) == 0 {
		return fmt.Errorf("expected operations to inspect")
	}
	for _, found := range raw.Operations {
		for _, field := range []string{"job_request", "service_proposal", "work_order"} {
			if _, exists := found[field]; !exists {
				return fmt.Errorf("operation %s omits %s instead of reporting null", found["id"], field)
			}
		}
	}
	return nil
}

func (suite *testSuite) recordInboxOperationID(label string) error {
	if err := suite.queryOperationsInbox(); err != nil {
		return err
	}
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	operationID, err := suite.inboxOperationID(label)
	if err != nil {
		return err
	}
	for _, found := range page.Operations {
		if found.ID == operationID {
			suite.operationInbox.recordedIDs[label] = found.ID
			return nil
		}
	}
	return fmt.Errorf("operation %q is not in the inbox", label)
}

func (suite *testSuite) inboxRequestAdvancedToWorkOrder(requestLabel, proposalLabel, depositProposalLabel, orderLabel string) error {
	if proposalLabel != depositProposalLabel {
		return fmt.Errorf("the deposit must belong to proposal %q", proposalLabel)
	}
	return suite.withInboxFixtureClock(func() error {
		now := suite.clock.Now()
		if err := suite.acceptInboxJobRequest(requestLabel); err != nil {
			return err
		}
		if err := suite.createInboxServiceProposal(proposalLabel, requestLabel, now, now.Add(72*time.Hour), 60, string(serviceproposal.StatusAccepted)); err != nil {
			return err
		}
		return suite.createInboxWorkOrder(orderLabel, proposalLabel, now, string(workorder.StatusScheduled), "", "")
	})
}

func (suite *testSuite) inboxOperationKeepsRecordedID(label string) error {
	recorded, exists := suite.operationInbox.recordedIDs[label]
	if !exists {
		return fmt.Errorf("no identifier was recorded for %q", label)
	}
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	for _, found := range page.Operations {
		if found.ID == recorded {
			return nil
		}
	}
	return fmt.Errorf("operation identifier %q recorded for %q is no longer in the inbox", recorded, label)
}

func (suite *testSuite) inboxOperationsHaveDistinctIDs(first, second string) error {
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, label := range []string{first, second} {
		operationID, err := suite.inboxOperationID(label)
		if err != nil {
			return err
		}
		found := false
		for _, operation := range page.Operations {
			found = found || operation.ID == operationID
		}
		if !found {
			return fmt.Errorf("operation %q is not in the inbox", label)
		}
		if seen[operationID] {
			return fmt.Errorf("operations %q and %q share identifier %q", first, second, operationID)
		}
		seen[operationID] = true
	}
	return nil
}

func (suite *testSuite) inboxErrorResponseHasNoOperations() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("invalid inbox error response JSON: %w", err)
	}
	if _, exists := raw["error"]; !exists {
		return fmt.Errorf("inbox error response is missing its error field: %s", suite.lastBody)
	}
	for field := range raw {
		if field != "error" && field != "message" {
			return fmt.Errorf("inbox error response has unexpected field %q", field)
		}
	}
	return nil
}
