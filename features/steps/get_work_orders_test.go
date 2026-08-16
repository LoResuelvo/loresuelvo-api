package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

type workOrderSummaryResponse struct {
	ID                int                                `json:"id"`
	ServiceProposalID int                                `json:"service_proposal_id"`
	AmountCents       int64                              `json:"amount_cents"`
	ScheduledOn       time.Time                          `json:"scheduled_on"`
	Description       string                             `json:"description"`
	Status            string                             `json:"status"`
	AcceptedOn        time.Time                          `json:"accepted_on"`
	Counterpart       serviceProposalCounterpartResponse `json:"counterpart"`
}

func registerGetWorkOrdersSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una orden de trabajo programada para la propuesta aceptada de "([^"]*)" para "([^"]*)" por "([^"]*)" para la fecha y hora "([^"]*)" con la descripción:$`, suite.thereIsScheduledWorkOrderForAcceptedProposalWithDetails)
	sc.Step(`^que existe una orden de trabajo programada para la propuesta aceptada de "([^"]*)" para "([^"]*)"$`, suite.thereIsScheduledWorkOrderForAcceptedProposal)
	sc.Step(`^que existen varias órdenes de trabajo programadas para "([^"]*)" con distintas fechas y horas de servicio$`, suite.thereAreScheduledWorkOrdersAtDifferentServiceTimes)
	sc.Step(`^que existen órdenes de trabajo en estados "([^"]*)", "([^"]*)" y "([^"]*)" para "([^"]*)"$`, suite.thereAreWorkOrdersInStatuses)
	sc.Step(`^consulto mis órdenes de trabajo$`, suite.requestMyWorkOrders)
	sc.Step(`^intento consultar mis órdenes de trabajo$`, suite.requestMyWorkOrders)
	sc.Step(`^el sistema muestra un listado de órdenes de trabajo vacío$`, suite.systemShowsEmptyWorkOrderList)
	sc.Step(`^el sistema muestra la orden de trabajo programada por "([^"]*)" para la fecha y hora "([^"]*)"$`, suite.systemShowsScheduledWorkOrder)
	sc.Step(`^la orden de trabajo incluye la descripción:$`, suite.workOrderIncludesDescription)
	sc.Step(`^la orden de trabajo incluye el identificador de la propuesta de servicio aceptada$`, suite.workOrderIncludesAcceptedServiceProposalID)
	sc.Step(`^la orden de trabajo incluye la fecha y hora de aceptación de la propuesta$`, suite.workOrderIncludesAcceptedOn)
	sc.Step(`^la contraparte de la orden de trabajo es el prestador "([^"]*)" con rubro "([^"]*)" y su foto de perfil$`, suite.workOrderCounterpartIsProvider)
	sc.Step(`^la contraparte de la orden de trabajo es la consumidora "([^"]*)"$`, suite.workOrderCounterpartIsConsumer)
	sc.Step(`^la contraparte de la orden de trabajo no incluye un rubro$`, suite.workOrderCounterpartDoesNotIncludeCategory)
	sc.Step(`^el sistema muestra solamente la orden de trabajo entre "([^"]*)" y "([^"]*)"$`, suite.systemShowsOnlyWorkOrderBetween)
	sc.Step(`^el sistema muestra las órdenes de trabajo ordenadas desde la fecha y hora de servicio más próxima$`, suite.systemShowsClosestWorkOrdersFirst)
	sc.Step(`^el listado distingue los estados "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.workOrderListShowsStatuses)
	sc.Step(`^el listado no incluye reportes ni imágenes$`, suite.workOrderListDoesNotIncludeCompletionEvidence)
}

func (suite *testSuite) thereIsScheduledWorkOrderForAcceptedProposalWithDetails(providerEmail, consumerEmail, amount, scheduledOn string, description *godog.DocString) error {
	amountCents, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}

	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing work order scheduled_on: %w", err)
	}

	return suite.createScheduledWorkOrderFixture(
		providerEmail,
		consumerEmail,
		amountCents,
		scheduledAt.UTC(),
		normalizeDocString(description),
	)
}

func (suite *testSuite) thereIsScheduledWorkOrderForAcceptedProposal(providerEmail, consumerEmail string) error {
	return suite.createScheduledWorkOrderFixture(
		providerEmail,
		consumerEmail,
		defaultServiceProposalAmount,
		defaultServiceProposalScheduledOn,
		defaultServiceProposalDescription,
	)
}

func (suite *testSuite) thereAreScheduledWorkOrdersAtDifferentServiceTimes(consumerEmail string) error {
	fixtures := []struct {
		providerEmail string
		scheduledOn   time.Time
	}{
		{providerEmail: "pedro.electricista@example.com", scheduledOn: time.Date(2026, time.July, 6, 10, 0, 0, 0, time.UTC)},
		{providerEmail: "juan.plomero@example.com", scheduledOn: time.Date(2026, time.July, 5, 9, 30, 0, 0, time.UTC)},
	}

	for _, fixture := range fixtures {
		if err := suite.createScheduledWorkOrderFixture(
			fixture.providerEmail,
			consumerEmail,
			defaultServiceProposalAmount,
			fixture.scheduledOn,
			defaultServiceProposalDescription,
		); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) thereAreWorkOrdersInStatuses(firstStatus, secondStatus, thirdStatus, consumerEmail string) error {
	fixtures := []struct {
		status        string
		providerEmail string
		imageName     string
	}{
		{status: firstStatus, providerEmail: "juan.plomero@example.com"},
		{status: secondStatus, providerEmail: "pedro.electricista@example.com", imageName: "listado-awaiting.jpg"},
		{status: thirdStatus, providerEmail: "luis.electricista@example.com", imageName: "listado-paid.jpg"},
	}

	for _, fixture := range fixtures {
		if err := suite.createWorkOrderForListingStatus(
			fixture.status,
			fixture.providerEmail,
			consumerEmail,
			fixture.imageName,
		); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) createWorkOrderForListingStatus(status, providerEmail, consumerEmail, imageName string) error {
	if status != string(workorder.StatusScheduled) &&
		status != string(workorder.StatusAwaitingPayment) &&
		status != string(workorder.StatusPaid) {
		return fmt.Errorf("unsupported work order listing status %q", status)
	}

	scheduledOn := suite.clock.Now().UTC().Add(48 * time.Hour)
	if err := suite.createScheduledWorkOrderFixture(
		providerEmail,
		consumerEmail,
		defaultServiceProposalAmount,
		scheduledOn,
		defaultServiceProposalDescription,
	); err != nil {
		return err
	}
	if status == string(workorder.StatusScheduled) {
		return nil
	}

	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	if err := suite.uploadAndConfirmCompletionImage(imageName); err != nil {
		return fmt.Errorf("preparing listing evidence image %q: %w", imageName, err)
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if err := suite.requestTestClockMock(order.ScheduledOn().UTC().Add(time.Minute).Format(time.RFC3339)); err != nil {
		return err
	}
	if err := suite.reportCompletion(defaultServiceProposalDescription, []string{imageName}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating listing completion report returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}
	if status != string(workorder.StatusPaid) {
		return nil
	}

	order, err = suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if err := order.RegisterApprovedBalancePayment(suite.clock.Now().UTC().Add(time.Minute)); err != nil {
		return fmt.Errorf("registering paid listing fixture: %w", err)
	}
	if _, err := suite.workOrderRepository.Save(context.Background(), order); err != nil {
		return fmt.Errorf("saving paid listing fixture: %w", err)
	}
	return nil
}

func (suite *testSuite) createScheduledWorkOrderFixture(providerEmail, consumerEmail string, amountCents int64, scheduledOn time.Time, description string) error {
	if err := suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusPending,
		amountCents,
		scheduledOn,
		description,
	); err != nil {
		return err
	}

	proposal, err := repositories.NewServiceProposalRepository(suite.database).
		FindByID(context.Background(), suite.lastServiceProposalID)
	if err != nil {
		return fmt.Errorf("finding service proposal for work order fixture: %w", err)
	}
	acceptedOn := suite.clock.Now()
	if err := proposal.Accept(proposal.Consumer.ID(), acceptedOn); err != nil {
		return fmt.Errorf("accepting service proposal for work order fixture: %w", err)
	}
	order, err := workorder.New(proposal, acceptedOn)
	if err != nil {
		return fmt.Errorf("creating scheduled work order fixture: %w", err)
	}
	unitOfWork := repositories.NewPaymentUnitOfWork(
		suite.database,
		suite.paymentIntentRepository,
		suite.paymentTransactionRepository,
		repositories.NewServiceProposalRepository(suite.database),
		suite.workOrderRepository,
		suite.notificationRepository,
	)
	ctx := context.Background()
	if err := unitOfWork.Execute(ctx, func(store payment.TransactionalStore) error {
		if err := store.SaveServiceProposal(ctx, proposal); err != nil {
			return err
		}
		return store.SaveWorkOrder(ctx, order)
	}); err != nil {
		return fmt.Errorf("saving scheduled work order fixture: %w", err)
	}
	if err := suite.serviceProposalHasStatus(suite.lastServiceProposalID, serviceproposal.StatusAccepted); err != nil {
		return err
	}
	return suite.systemRegistersOneScheduledWorkOrder()
}

func (suite *testSuite) requestMyWorkOrders() error {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/work-orders", nil)
	if err != nil {
		return err
	}
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) systemShowsEmptyWorkOrderList() error {
	orders, err := suite.workOrderSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(orders) != 0 {
		return fmt.Errorf("expected empty work order list, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemShowsScheduledWorkOrder(amount, scheduledOn string) error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}

	expectedAmount, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}
	expectedScheduledOn, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return err
	}

	if order.Status != workOrderStatusScheduled {
		return fmt.Errorf("expected work order status %q, got %q", workOrderStatusScheduled, order.Status)
	}
	if order.AmountCents != expectedAmount {
		return fmt.Errorf("expected amount_cents %d, got %d", expectedAmount, order.AmountCents)
	}
	if !order.ScheduledOn.Equal(expectedScheduledOn.UTC()) {
		return fmt.Errorf("expected scheduled_on %s, got %s", expectedScheduledOn.UTC(), order.ScheduledOn)
	}
	return suite.assertValidWorkOrderSummary(order)
}

func (suite *testSuite) workOrderIncludesDescription(description *godog.DocString) error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if order.Description != normalizeDocString(description) {
		return fmt.Errorf("expected work order description %q, got %q", normalizeDocString(description), order.Description)
	}
	return nil
}

func (suite *testSuite) workOrderIncludesAcceptedServiceProposalID() error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if order.ServiceProposalID == 0 {
		return fmt.Errorf("expected work order to include service_proposal_id, got body %s", string(suite.lastBody))
	}
	if suite.lastServiceProposalID != 0 && order.ServiceProposalID != suite.lastServiceProposalID {
		return fmt.Errorf("expected service_proposal_id %d, got %d", suite.lastServiceProposalID, order.ServiceProposalID)
	}
	return nil
}

func (suite *testSuite) workOrderIncludesAcceptedOn() error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if order.AcceptedOn.IsZero() {
		return fmt.Errorf("expected work order to include accepted_on, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) workOrderCounterpartIsProvider(fullName, categoryName string) error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if !serviceProposalCounterpartMatches(order.Counterpart, participantRoleProvider, fullName) {
		return fmt.Errorf("expected provider counterpart %q, got body %s", fullName, string(suite.lastBody))
	}
	if order.Counterpart.CategoryName != categoryName {
		return fmt.Errorf("expected counterpart category %q, got %q", categoryName, order.Counterpart.CategoryName)
	}
	if strings.TrimSpace(order.Counterpart.ProfilePhotoURL) == "" {
		return fmt.Errorf("expected provider counterpart to include profile_photo_url, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) workOrderCounterpartIsConsumer(fullName string) error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if !serviceProposalCounterpartMatches(order.Counterpart, participantRoleConsumer, fullName) {
		return fmt.Errorf("expected consumer counterpart %q, got body %s", fullName, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) workOrderCounterpartDoesNotIncludeCategory() error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	if order.Counterpart.CategoryName != "" {
		return fmt.Errorf("expected consumer counterpart without category_name, got %q", order.Counterpart.CategoryName)
	}
	return nil
}

func (suite *testSuite) systemShowsOnlyWorkOrderBetween(consumerEmail, providerEmail string) error {
	order, err := suite.onlyWorkOrderSummary()
	if err != nil {
		return err
	}
	fixture, err := suite.serviceProposalFixtureBetween(consumerEmail, providerEmail)
	if err != nil {
		return err
	}
	if order.ServiceProposalID != suite.lastServiceProposalID {
		return fmt.Errorf("expected service_proposal_id %d, got %d", suite.lastServiceProposalID, order.ServiceProposalID)
	}
	if order.AmountCents != fixture.amountCents || !order.ScheduledOn.Equal(fixture.scheduledOn.UTC()) {
		return fmt.Errorf("expected work order between %q and %q, got body %s", consumerEmail, providerEmail, string(suite.lastBody))
	}
	return suite.assertValidWorkOrderSummary(order)
}

func (suite *testSuite) systemShowsClosestWorkOrdersFirst() error {
	orders, err := suite.workOrderSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(orders) < 2 {
		return fmt.Errorf("expected at least two work orders, got %d", len(orders))
	}
	for index, order := range orders {
		if err := suite.assertValidWorkOrderSummary(order); err != nil {
			return err
		}
		if index > 0 && orders[index-1].ScheduledOn.After(order.ScheduledOn) {
			return fmt.Errorf("expected work orders ordered by scheduled_on ascending, got body %s", string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) workOrderListShowsStatuses(firstStatus, secondStatus, thirdStatus string) error {
	expectedStatuses := []string{firstStatus, secondStatus, thirdStatus}
	orders, err := suite.workOrderSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(orders) != len(expectedStatuses) {
		return fmt.Errorf("expected %d work orders in status listing, got %d with body %s", len(expectedStatuses), len(orders), string(suite.lastBody))
	}

	expectedCounts := make(map[string]int, len(expectedStatuses))
	for _, status := range expectedStatuses {
		expectedCounts[status]++
	}
	actualCounts := make(map[string]int, len(orders))
	for _, order := range orders {
		actualCounts[order.Status]++
	}
	for status, expectedCount := range expectedCounts {
		if actualCounts[status] != expectedCount {
			return fmt.Errorf("expected status %q %d time(s), got %d with body %s", status, expectedCount, actualCounts[status], string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) workOrderListDoesNotIncludeCompletionEvidence() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &entries); err != nil {
		return fmt.Errorf("response is not a valid work order summary list: %w", err)
	}
	for index, entry := range entries {
		for _, forbiddenField := range []string{"completion_report", "images", "file_id"} {
			if _, exists := entry[forbiddenField]; exists {
				return fmt.Errorf("work order summary %d unexpectedly includes %q: %s", index, forbiddenField, string(suite.lastBody))
			}
		}
	}
	return nil
}

func (suite *testSuite) onlyWorkOrderSummary() (workOrderSummaryResponse, error) {
	orders, err := suite.workOrderSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return workOrderSummaryResponse{}, err
	}
	if len(orders) != 1 {
		return workOrderSummaryResponse{}, fmt.Errorf("expected exactly one work order, got %d with body %s", len(orders), string(suite.lastBody))
	}
	return orders[0], nil
}

func (suite *testSuite) workOrderSummaryResponsesShouldHaveStatusCode(statusCode int) ([]workOrderSummaryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return nil, err
	}

	var orders []workOrderSummaryResponse
	if err := json.Unmarshal(suite.lastBody, &orders); err != nil {
		return nil, fmt.Errorf("response is not a valid JSON work order summary list: %w", err)
	}
	return orders, nil
}

func (suite *testSuite) assertValidWorkOrderSummary(order workOrderSummaryResponse) error {
	if order.ID == 0 {
		return fmt.Errorf("expected work order to include id, got body %s", string(suite.lastBody))
	}
	if order.ServiceProposalID == 0 {
		return fmt.Errorf("expected work order to include service_proposal_id, got body %s", string(suite.lastBody))
	}
	if order.AmountCents <= 0 {
		return fmt.Errorf("expected work order to include positive amount_cents, got body %s", string(suite.lastBody))
	}
	if order.ScheduledOn.IsZero() {
		return fmt.Errorf("expected work order to include scheduled_on, got body %s", string(suite.lastBody))
	}
	if strings.TrimSpace(order.Description) == "" {
		return fmt.Errorf("expected work order to include description, got body %s", string(suite.lastBody))
	}
	if order.Status != workOrderStatusScheduled {
		return fmt.Errorf("expected work order to include scheduled status, got body %s", string(suite.lastBody))
	}
	if order.AcceptedOn.IsZero() {
		return fmt.Errorf("expected work order to include accepted_on, got body %s", string(suite.lastBody))
	}
	if order.Counterpart.ID == 0 ||
		strings.TrimSpace(order.Counterpart.Role) == "" ||
		strings.TrimSpace(order.Counterpart.Name) == "" ||
		strings.TrimSpace(order.Counterpart.Surname) == "" {
		return fmt.Errorf("expected work order to include a complete counterpart, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) serviceProposalFixtureBetween(consumerEmail, providerEmail string) (serviceProposalFixture, error) {
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return serviceProposalFixture{}, fmt.Errorf("finding expected work order consumer: %w", err)
	}
	providerID, err := suite.userRepository.FindIDByEmail(providerEmail)
	if err != nil {
		return serviceProposalFixture{}, fmt.Errorf("finding expected work order provider: %w", err)
	}

	for proposalID, fixture := range suite.serviceProposalFixtures {
		if fixture.consumerID == consumerID && fixture.providerID == providerID {
			suite.lastServiceProposalID = proposalID
			return fixture, nil
		}
	}
	return serviceProposalFixture{}, fmt.Errorf("expected a prepared work order between %q and %q", consumerEmail, providerEmail)
}
