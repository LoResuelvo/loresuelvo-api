package steps_test

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	consumerdomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func (suite *testSuite) adminFunnelProvidersByCategory(table *godog.Table) error {
	if err := requireTableHeaders(table, "correo", "nombre", "apellido", "rubro"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 4 {
			return fmt.Errorf("provider fixture expects four cells")
		}
		if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, row.Cells[3].Value); err != nil {
			return fmt.Errorf("registering funnel provider %q: %w", row.Cells[0].Value, err)
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelEnsureMaps() {
	if suite.adminFunnel.assessmentAliases == nil {
		suite.adminFunnel.assessmentAliases = make(map[string]int)
	}
	if suite.adminFunnel.requestAliases == nil {
		suite.adminFunnel.requestAliases = make(map[string]int)
	}
	if suite.adminFunnel.proposalAliases == nil {
		suite.adminFunnel.proposalAliases = make(map[string]int)
	}
	if suite.adminFunnel.proposalRequests == nil {
		suite.adminFunnel.proposalRequests = make(map[string]string)
	}
	if suite.adminFunnel.orderAliases == nil {
		suite.adminFunnel.orderAliases = make(map[string]int)
	}
}

func (suite *testSuite) adminFunnelCreateAssessment(alias, email, categoryName, outcome string, version int, created time.Time) (int, error) {
	created = created.UTC()
	suite.adminFunnelEnsureMaps()
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return 0, fmt.Errorf("finding assessment consumer %q: %w", email, err)
	}
	categoryID := 0
	if categoryName != "" {
		var ok bool
		categoryID, ok = suite.categoryIDsByName[categoryName]
		if !ok {
			return 0, fmt.Errorf("category %q is not persisted", categoryName)
		}
	}
	fixture := testsupport.OperationDetailRequestFixture{DB: suite.database}
	conversationID := suite.adminFunnel.assessmentConversationID
	if version == 1 || conversationID == 0 {
		conversationID, err = fixture.CreateChatbotConversation(suite.scenarioContext, consumerID, created)
		if err != nil {
			return 0, err
		}
	} else {
		current, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
		if err != nil {
			return 0, err
		}
		chatbot, ok := current.(*conversation.ChatBotConversation)
		if !ok || chatbot.ConsumerID != consumerID {
			return 0, fmt.Errorf("assessment version belongs to another consumer conversation")
		}
	}
	messageID, err := fixture.AddMessage(suite.scenarioContext, conversationID, "Fixture evidence for professional assessment", created)
	if err != nil {
		return 0, err
	}
	var category *int
	if categoryID > 0 {
		category = &categoryID
	}
	title, description := "Fixture problem", "Fixture description"
	if outcome == "collecting_information" {
		title, description = "", ""
	}
	id, err := fixture.AddAssessment(suite.scenarioContext, conversationID, version, messageID, outcome, category, title, description, created)
	if err != nil {
		return 0, err
	}
	if alias != "" {
		suite.adminFunnel.assessmentAliases[alias] = id
	}
	suite.adminFunnel.assessmentConversationID, suite.adminFunnel.assessmentMessageID = conversationID, messageID
	return id, nil
}

func (suite *testSuite) adminFunnelCreateManualRequest(alias, consumerEmail, providerEmail string, created time.Time) (int, error) {
	suite.adminFunnelEnsureMaps()
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return 0, err
	}
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return 0, err
	}
	request, err := jobrequest.New(consumerID, providerID, "Funnel fixture request", "Request fixture", nil)
	if err != nil {
		return 0, err
	}
	request.CreatedOn = created.UTC()
	workConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	if err != nil {
		return 0, err
	}
	saved, err := suite.jobRequestRepository.SaveWithConversation(*request, workConversation)
	if err != nil {
		return 0, fmt.Errorf("persisting manual funnel request: %w", err)
	}
	if alias != "" {
		suite.adminFunnel.requestAliases[alias] = saved.ID
	}
	return saved.ID, nil
}

func (suite *testSuite) adminFunnelCreateAIRequest(alias, consumerEmail, providerEmail, assessmentAlias string, created time.Time) (int, error) {
	requestID, err := suite.adminFunnelCreateManualRequest(alias, consumerEmail, providerEmail, created)
	if err != nil {
		return 0, err
	}
	assessmentID := suite.adminFunnel.assessmentAliases[assessmentAlias]
	if assessmentID == 0 {
		return 0, fmt.Errorf("assessment alias %q is unknown", assessmentAlias)
	}
	if err := (testsupport.OperationDetailRequestFixture{DB: suite.database}).SetRequestAssessment(suite.scenarioContext, requestID, assessmentID); err != nil {
		return 0, err
	}
	return requestID, nil
}

func (suite *testSuite) adminFunnelCreateProposal(alias, requestAlias string, created, scheduled time.Time, accepted bool) (int, error) {
	suite.adminFunnelEnsureMaps()
	requestID := suite.adminFunnel.requestAliases[requestAlias]
	if requestID == 0 {
		return 0, fmt.Errorf("request alias %q is unknown", requestAlias)
	}
	request, err := suite.jobRequestRepository.FindByID(requestID)
	if err != nil {
		return 0, err
	}
	providerUser, err := suite.userRepository.FindByID(suite.scenarioContext, request.ProviderID)
	if err != nil {
		return 0, err
	}
	providerProfile, ok := providerUser.(*provider.Provider)
	if !ok {
		return 0, fmt.Errorf("request %q provider is %T", requestAlias, providerUser)
	}
	consumerUser, err := suite.userRepository.FindByID(suite.scenarioContext, request.ConsumerID)
	if err != nil {
		return 0, err
	}
	consumerProfile, ok := consumerUser.(*consumerdomain.Consumer)
	if !ok {
		return 0, fmt.Errorf("request %q consumer is %T", requestAlias, consumerUser)
	}
	workConversation, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.ConversationID)
	if err != nil {
		return 0, err
	}
	terms, err := serviceproposal.NewBookingPolicy().Calculate(defaultServiceProposalAmount, scheduled)
	if err != nil {
		return 0, err
	}
	status := serviceproposal.StatusPending
	if accepted {
		status = serviceproposal.StatusAccepted
	}
	proposal := &serviceproposal.ServiceProposal{Provider: providerProfile, Consumer: consumerProfile, Conversation: workConversation, ScheduledOn: scheduled.UTC(), Description: "Funnel fixture proposal", EstimatedDurationMinutes: 60, Status: status, CreatedOn: created.UTC(), BookingTerms: terms}
	saved, err := repositories.NewServiceProposalRepository(suite.database).Save(proposal)
	if err != nil {
		return 0, err
	}
	suite.adminFunnel.proposalAliases[alias] = saved.ID
	suite.adminFunnel.proposalRequests[alias] = requestAlias
	return saved.ID, nil
}

func (suite *testSuite) adminFunnelCreateOrder(alias, proposalAlias string, acceptedOn time.Time) (int, error) {
	suite.adminFunnelEnsureMaps()
	proposalID := suite.adminFunnel.proposalAliases[proposalAlias]
	if proposalID == 0 {
		return 0, fmt.Errorf("proposal alias %q is unknown", proposalAlias)
	}
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, proposalID)
	if err != nil {
		return 0, err
	}
	order, err := workorder.New(proposal, acceptedOn.UTC())
	if err != nil {
		return 0, err
	}
	saved, err := suite.workOrderRepository.Save(suite.scenarioContext, order)
	if err != nil {
		return 0, err
	}
	suite.adminFunnel.orderAliases[alias] = saved.ID()
	return saved.ID(), nil
}

func (suite *testSuite) adminFunnelAddMilestones(orderAlias string, reportedOn time.Time, paidOn time.Time, reviewed bool) error {
	orderID := suite.adminFunnel.orderAliases[orderAlias]
	if orderID == 0 {
		return fmt.Errorf("order alias %q is unknown", orderAlias)
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return err
	}
	providerID := order.ProviderID()
	providerAuthID := auth0IDForProviderEmail(order.Provider().Email())
	fileID := uuid.NewString()
	metadata, err := filedomain.NewFileMetadata("funnel-evidence.jpg", "image/jpeg", 16)
	if err != nil {
		return err
	}
	file, err := filedomain.NewFile(fileID, "bdd/"+fileID, "private", metadata, filedomain.StatusConfirmed, filedomain.VisibilityPrivate, filedomain.PurposeWorkOrderCompletionImage, providerAuthID, reportedOn.UTC(), reportedOn.UTC())
	if err != nil {
		return err
	}
	if err := suite.fileRepository.Save(suite.scenarioContext, *file); err != nil {
		return err
	}
	report, err := workorder.NewCompletionReport("Historical completion evidence", []string{fileID}, reportedOn.UTC())
	if err != nil {
		return err
	}
	if err := order.ReportCompletion(providerID, report); err != nil {
		return fmt.Errorf("adding completion report %q: %w", orderAlias, err)
	}
	order, err = suite.workOrderRepository.Save(suite.scenarioContext, order)
	if err != nil {
		return err
	}
	if !paidOn.IsZero() {
		if err := order.AuthorizeBalanceCheckout(order.ConsumerID(), paidOn.UTC()); err != nil {
			return err
		}
		if err := order.RegisterApprovedBalancePayment(paidOn.UTC()); err != nil {
			return err
		}
		order, err = suite.workOrderRepository.Save(suite.scenarioContext, order)
		if err != nil {
			return err
		}
	}
	if reviewed {
		consumerUser, err := suite.userRepository.FindByID(suite.scenarioContext, order.ConsumerID())
		if err != nil {
			return err
		}
		consumerProfile, ok := consumerUser.(*consumerdomain.Consumer)
		if !ok {
			return fmt.Errorf("order consumer is %T", consumerUser)
		}
		review, err := workorder.NewReview(5, "Funnel fixture review")
		if err != nil {
			return err
		}
		if err := order.AddReview(consumerProfile, review); err != nil {
			return err
		}
		if _, err := suite.workOrderRepository.Save(suite.scenarioContext, order); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelHistoricalAssessments(first, second string) error {
	for i, value := range []string{first, second} {
		instant, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return err
		}
		if _, err := suite.adminFunnelCreateAssessment(fmt.Sprintf("A%d", i+1), "ana@example.com", "Plomería", "professional_required", 1, instant); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelHistoricalManualRequest(value string) error {
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return err
	}
	_, err = suite.adminFunnelCreateManualRequest("S1", "beatriz@example.com", "luis@example.com", instant)
	return err
}
func (suite *testSuite) adminFunnelOneEach() error {
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1"); err != nil {
		return err
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "request", "1")
}
func (suite *testSuite) adminFunnelManualRequestByCategory(consumerEmail, categoryName string) error {
	providerEmail := "luis@example.com"
	id, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	found, err := suite.userRepository.FindByID(suite.scenarioContext, id)
	if err != nil {
		return err
	}
	profile, ok := found.(*provider.Provider)
	if !ok {
		return fmt.Errorf("user %q is not a provider", providerEmail)
	}
	categoryID, ok := suite.categoryIDsByName[categoryName]
	if !ok {
		return fmt.Errorf("unknown category %q", categoryName)
	}
	if !profile.HasCategory(categoryID) {
		return fmt.Errorf("provider %q current category does not match %q", providerEmail, categoryName)
	}
	_, err = suite.adminFunnelCreateManualRequest("S1", consumerEmail, providerEmail, suite.adminFunnelFixtureTime())
	return err
}
func (suite *testSuite) adminFunnelThreeAssessments(first, second, third string) error {
	consumers := []string{"ana@example.com", "beatriz@example.com", "carla@example.com"}
	for i, alias := range []string{first, second, third} {
		if _, err := suite.adminFunnelCreateAssessment(alias, consumers[i], "Plomería", "professional_required", 1, suite.adminFunnelFixtureTime().Add(time.Duration(i)*time.Second)); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelThreeUnrequestedAssessments() error {
	return suite.adminFunnelThreeAssessments("A1", "A2", "A3")
}
func (suite *testSuite) adminFunnelNoManualRequests() error {
	criteria := operation.FunnelCriteria{Period: operation.TimeWindow{From: suite.clock.Now().Add(-30 * 24 * time.Hour), To: suite.clock.Now()}}
	snapshot, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, criteria)
	if err != nil {
		return fmt.Errorf("verifying empty manual-origin fixture: %w", err)
	}
	if snapshot.Manual.Counts.Origins != 0 {
		return fmt.Errorf("expected no manual origins before query, got %d", snapshot.Manual.Counts.Origins)
	}
	return nil
}
func (suite *testSuite) adminFunnelAssessmentAtPeriodStart(alias string) error {
	created, err := time.Parse(time.RFC3339Nano, "2026-09-01T00:00:00-03:00")
	if err != nil {
		return err
	}
	_, err = suite.adminFunnelCreateAssessment(alias, "ana@example.com", "Plomería", "professional_required", 1, created)
	return err
}
func (suite *testSuite) adminFunnelPostPeriodBranch() error {
	assessmentID := suite.adminFunnel.assessmentAliases["A1"]
	if assessmentID == 0 {
		return fmt.Errorf("period-start assessment A1 is missing")
	}
	requestAt := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	requestID, err := suite.adminFunnelCreateManualRequest("S1", "ana@example.com", "juan@example.com", requestAt)
	if err != nil {
		return err
	}
	if err := (testsupport.OperationDetailRequestFixture{DB: suite.database}).SetRequestAssessment(suite.scenarioContext, requestID, assessmentID); err != nil {
		return err
	}
	scheduled := requestAt.Add(72 * time.Hour)
	_, err = suite.adminFunnelCreateProposal("P1", "S1", requestAt.Add(time.Hour), scheduled, true)
	if err != nil {
		return err
	}
	_, err = suite.adminFunnelCreateOrder("O1", "P1", requestAt.Add(2*time.Hour))
	if err != nil {
		return err
	}
	return suite.adminFunnelAddMilestones("O1", scheduled.Add(time.Hour), scheduled.Add(2*time.Hour), true)
}

func (suite *testSuite) adminFunnelAIAssessmentHasTwoRequests(alias string) error {
	if suite.adminFunnel.assessmentAliases[alias] == 0 {
		return fmt.Errorf("assessment alias %q is missing", alias)
	}
	base := suite.adminFunnelFixtureTime()
	if _, err := suite.adminFunnelCreateAIRequest("S1", "ana@example.com", "juan@example.com", alias, base.Add(time.Hour)); err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateAIRequest("S2", "ana@example.com", "pedro@example.com", alias, base.Add(2*time.Hour)); err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateProposal("P1", "S1", base.Add(3*time.Hour), base.Add(72*time.Hour), true); err != nil {
		return err
	}
	_, err := suite.adminFunnelCreateProposal("P3", "S2", base.Add(4*time.Hour), base.Add(73*time.Hour), true)
	return err
}

func (suite *testSuite) adminFunnelAIBranchHasThreeProposalsAndOrders(alias string) error {
	source := suite.adminFunnel.assessmentAliases[alias]
	for _, requestAlias := range []string{"S1", "S2"} {
		request, err := suite.jobRequestRepository.FindByID(suite.adminFunnel.requestAliases[requestAlias])
		if err != nil {
			return err
		}
		if request.SourceAssessmentID == nil || *request.SourceAssessmentID != source {
			return fmt.Errorf("request %q does not originate in assessment %q", requestAlias, alias)
		}
	}
	base := suite.adminFunnelFixtureTime()
	if _, err := suite.adminFunnelCreateProposal("P2", "S1", base.Add(3*time.Hour+time.Second), base.Add(74*time.Hour), true); err != nil {
		return err
	}
	for i, item := range []struct{ order, proposal string }{{"O1", "P1"}, {"O2", "P2"}, {"O3", "P3"}} {
		if _, err := suite.adminFunnelCreateOrder(item.order, item.proposal, base.Add(time.Duration(i+5)*time.Hour)); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelCreateProposalOrder(proposal, order, request string, created time.Time) error {
	scheduled := created.Add(72 * time.Hour)
	if _, err := suite.adminFunnelCreateProposal(proposal, request, created, scheduled, true); err != nil {
		return err
	}
	_, err := suite.adminFunnelCreateOrder(order, proposal, created.Add(time.Hour))
	return err
}
func (suite *testSuite) adminFunnelOneAssessmentBranch(assessment, unused string) error {
	if suite.adminFunnel.assessmentAliases[unused] == 0 {
		return fmt.Errorf("assessment alias %q is missing", unused)
	}
	for _, requestID := range suite.adminFunnel.requestAliases {
		request, err := suite.jobRequestRepository.FindByID(requestID)
		if err != nil {
			return err
		}
		if request.SourceAssessmentID != nil && *request.SourceAssessmentID == suite.adminFunnel.assessmentAliases[unused] {
			return fmt.Errorf("assessment %q unexpectedly has a request", unused)
		}
	}
	if _, err := suite.adminFunnelCreateAIRequest("S3", "beatriz@example.com", "luis@example.com", assessment, suite.adminFunnelFixtureTime().Add(8*time.Hour)); err != nil {
		return err
	}
	return suite.adminFunnelCreateProposalOrder("P4", "O4", "S3", suite.adminFunnelFixtureTime().Add(9*time.Hour))
}

func (suite *testSuite) adminFunnelAIBranchMilestones(first, second string) error {
	if err := suite.adminFunnelOrderOrigin("O1", first); err != nil {
		return err
	}
	if err := suite.adminFunnelOrderOrigin("O4", second); err != nil {
		return err
	}
	if err := suite.adminFunnelAddMilestones("O1", suite.adminFunnelFixtureTime().Add(80*time.Hour), suite.adminFunnelFixtureTime().Add(81*time.Hour), true); err != nil {
		return err
	}
	return suite.adminFunnelAddMilestones("O4", suite.adminFunnelFixtureTime().Add(84*time.Hour), time.Time{}, false)
}

func (suite *testSuite) adminFunnelPaymentAttempts(assessmentAlias string) error {
	if err := suite.adminFunnelOrderOrigin("O1", assessmentAlias); err != nil {
		return err
	}
	for i, status := range []payment.ExternalPaymentStatus{payment.ExternalPaymentStatusRejected, payment.ExternalPaymentStatusApproved} {
		alias := fmt.Sprintf("funnel-attempt-%d", i+1)
		intent, err := suite.adminFunnelPersistCheckout(alias, "P1", suite.adminFunnelFixtureTime().Add(4*time.Hour+time.Duration(i)*time.Minute))
		if err != nil {
			return err
		}
		external := payment.ExternalPayment{ID: "funnel-transaction-" + alias, SellerAccountID: "fixture-seller", ExternalReference: intent.ID, Status: status, Currency: intent.Currency, AmountCents: intent.TotalAmountCents}
		at := intent.CreatedOn.Add(time.Minute)
		if status == payment.ExternalPaymentStatusRejected {
			err = intent.MarkRejected(external, at)
		} else {
			err = intent.MarkPaid(external, at)
		}
		if err != nil {
			return err
		}
		if err := suite.paymentIntentRepository.Save(suite.scenarioContext, intent); err != nil {
			return err
		}
		transaction, err := payment.NewTransaction(intent.ID, paymentaccount.PaymentProvider("mercado_pago"), external, at)
		if err != nil {
			return err
		}
		if err := suite.paymentTransactionRepository.Save(suite.scenarioContext, transaction); err != nil {
			return err
		}
		suite.adminPaymentState().intents[alias] = intent
	}
	return nil
}

func (suite *testSuite) adminFunnelThreeManualRequests(first, second, third string) error {
	for i, alias := range []string{first, second, third} {
		if _, err := suite.adminFunnelCreateManualRequest(alias, []string{"ana@example.com", "beatriz@example.com", "carla@example.com"}[i], []string{"juan@example.com", "pedro@example.com", "juan@example.com"}[i], suite.adminFunnelFixtureTime().Add(time.Duration(i)*time.Minute)); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelManualProposals(first, second, third string) error {
	for _, item := range []struct {
		alias, request string
		offset         time.Duration
	}{{"P1", first, time.Second}, {"P2", first, 2 * time.Second}, {"P3", second, time.Minute + 3*time.Second}, {"P4", third, 2*time.Minute + 4*time.Second}} {
		at := suite.adminFunnelFixtureTime().Add(item.offset)
		if _, err := suite.adminFunnelCreateProposal(item.alias, item.request, at, at.Add(72*time.Hour), item.alias != "P4"); err != nil {
			return err
		}
	}
	_, err := suite.adminFunnelPersistCheckout("funnel-manual-checkout", "P4", suite.adminFunnelFixtureTime().Add(3*time.Minute))
	return err
}

func (suite *testSuite) adminFunnelManualOrders(first, second, order1, order1b, order2 string) error {
	for i, item := range []struct{ order, proposal, request string }{{order1, "P1", first}, {order1b, "P2", first}, {order2, "P3", second}} {
		if suite.adminFunnel.proposalRequests[item.proposal] != item.request {
			return fmt.Errorf("proposal %q does not belong to request %q", item.proposal, item.request)
		}
		if _, err := suite.adminFunnelCreateOrder(item.order, item.proposal, suite.adminFunnelFixtureTime().Add(time.Duration(i+5)*time.Minute)); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelManualMilestones(first, second, paid, noReport string) error {
	base := suite.adminFunnelFixtureTime()
	if paid != first && paid != second {
		return fmt.Errorf("paid order is not one of the reported orders")
	}
	for _, alias := range []string{first, second} {
		paidAt := time.Time{}
		reviewed := alias == paid
		if reviewed {
			paidAt = base.Add(81 * time.Hour)
		}
		if err := suite.adminFunnelAddMilestones(alias, base.Add(80*time.Hour), paidAt, reviewed); err != nil {
			return err
		}
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.adminFunnel.orderAliases[noReport])
	if err != nil {
		return err
	}
	if order.CompletionReport() != nil {
		return fmt.Errorf("order %q unexpectedly has a completion report", noReport)
	}
	return nil
}

func (suite *testSuite) adminFunnelNoOrderForCheckout(requestAlias string) error {
	proposalID := suite.adminFunnel.proposalAliases["P4"]
	if suite.adminFunnel.proposalRequests["P4"] != requestAlias {
		return fmt.Errorf("checkout proposal does not belong to request %q", requestAlias)
	}
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(suite.scenarioContext, proposalID, payment.PurposeBookingDeposit)
	if err != nil {
		return err
	}
	if intent == nil || intent.Status != payment.StatusCheckoutReady || intent.CheckoutSession == nil {
		return fmt.Errorf("proposal checkout_ready intent is not persisted")
	}
	order, err := suite.workOrderRepository.FindByServiceProposalID(suite.scenarioContext, proposalID)
	if !errors.Is(err, workorder.ErrDoesNotExist) {
		return fmt.Errorf("checkout-ready proposal must have no persisted order: order=%v error=%v", order, err)
	}
	return nil
}

func (suite *testSuite) adminFunnelTwoBranches(alias string) error {
	base := suite.adminFunnelFixtureTime()
	if suite.adminFunnel.assessmentAliases[alias] == 0 {
		if _, err := suite.adminFunnelCreateAssessment(alias, "ana@example.com", "Plomería", "professional_required", 1, base); err != nil {
			return err
		}
	}
	if _, err := suite.adminFunnelCreateAIRequest("S1", "ana@example.com", "juan@example.com", alias, base.Add(time.Hour)); err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateAIRequest("S2", "ana@example.com", "pedro@example.com", alias, base.Add(2*time.Hour)); err != nil {
		return err
	}
	if err := suite.adminFunnelCreateProposalOrder("P1", "O1", "S1", base.Add(3*time.Hour)); err != nil {
		return err
	}
	return suite.adminFunnelCreateProposalOrder("P2", "O2", "S2", base.Add(4*time.Hour))
}

func (suite *testSuite) adminFunnelBranchReported(alias string) error {
	if suite.adminFunnel.orderAliases[alias] == 0 {
		return fmt.Errorf("order %q is missing", alias)
	}
	return suite.adminFunnelAddMilestones(alias, suite.adminFunnelFixtureTime().Add(80*time.Hour), time.Time{}, false)
}
func (suite *testSuite) adminFunnelBranchPaidWithoutReport(alias string) error {
	if err := suite.adminFunnelAddMilestones(alias, suite.adminFunnelFixtureTime().Add(80*time.Hour), suite.adminFunnelFixtureTime().Add(81*time.Hour), false); err != nil {
		return err
	}
	return (testsupport.OperationDetailReportFixture{DB: suite.database}).RemoveCompletionReport(suite.scenarioContext, suite.adminFunnel.orderAliases[alias])
}
func (suite *testSuite) adminFunnelHistoricalAnomaly() error {
	// Read historical rows through the administrative projection. A paid order
	// without a report intentionally cannot restore a valid business aggregate.
	reader := repositories.NewOperationDetailReader(suite.database)
	first, err := reader.FindByID(suite.scenarioContext, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: suite.adminFunnel.requestAliases["S1"]})
	if err != nil {
		return err
	}
	second, err := reader.FindByID(suite.scenarioContext, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: suite.adminFunnel.requestAliases["S2"]})
	if err != nil {
		return err
	}
	if first == nil || second == nil || first.WorkOrder == nil || second.WorkOrder == nil {
		return fmt.Errorf("historical order branches are not persisted")
	}
	if first.WorkOrder.ID != suite.adminFunnel.orderAliases["O1"] || second.WorkOrder.ID != suite.adminFunnel.orderAliases["O2"] {
		return fmt.Errorf("historical order branches are not linked to their own requests")
	}
	if first.WorkOrder.CompletionReport == nil || first.WorkOrder.BalancePaidOn != nil || second.WorkOrder.CompletionReport != nil || second.WorkOrder.BalancePaidOn == nil {
		return fmt.Errorf("historical split-branch milestones are not persisted")
	}
	if first.ServiceProposal == nil || second.ServiceProposal == nil || first.ServiceProposal.ID == second.ServiceProposal.ID {
		return fmt.Errorf("historical orders must belong to distinct branches")
	}
	return nil
}

func (suite *testSuite) adminFunnelAnomalousRequest(alias, requestCreated, firstProposal, secondProposal string) error {
	created, err := time.Parse(time.RFC3339Nano, requestCreated)
	if err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateManualRequest(alias, "ana@example.com", "juan@example.com", created); err != nil {
		return err
	}
	for i, value := range []string{firstProposal, secondProposal} {
		proposalAt, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return err
		}
		scheduled := time.Date(2026, 9, 12, 10, 31, 0, 0, time.UTC)
		if _, err := suite.adminFunnelCreateProposal(fmt.Sprintf("P%d", i+1), alias, proposalAt, scheduled, true); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelAnomalousReport(accepted string) error {
	acceptedOn, err := time.Parse(time.RFC3339Nano, accepted)
	if err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateOrder("O1", "P1", acceptedOn); err != nil {
		return err
	}
	reportOn := time.Date(2026, 9, 12, 10, 32, 0, 0, time.UTC)
	if err := suite.adminFunnelAddMilestones("O1", reportOn, time.Time{}, false); err != nil {
		return err
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.adminFunnel.orderAliases["O1"])
	if err != nil {
		return err
	}
	if order.CompletionReport() == nil {
		return fmt.Errorf("generic completion report is missing before historical replacement")
	}
	return (testsupport.OperationDetailReportFixture{DB: suite.database}).ReplaceCompletionReport(suite.scenarioContext, order.ID(), "Historical report before acceptance", acceptedOn.Add(-time.Minute), order.CompletionReport().ImageFileIDs())
}
func (suite *testSuite) adminFunnelNoPaymentForFirstAnomaly() error {
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.adminFunnel.orderAliases["O1"])
	if err != nil {
		return err
	}
	if !order.PaidOn().IsZero() {
		return fmt.Errorf("first historical anomaly must not have paid_on")
	}
	return nil
}
func (suite *testSuite) adminFunnelValidSecondBranch(alias, requestAtText, proposalAtText, acceptedText, scheduledText, reportedText, paidText string) error {
	requestAt, err := time.Parse(time.RFC3339Nano, requestAtText)
	if err != nil {
		return err
	}
	proposalAt, err := time.Parse(time.RFC3339Nano, proposalAtText)
	if err != nil {
		return err
	}
	acceptedAt, err := time.Parse(time.RFC3339Nano, acceptedText)
	if err != nil {
		return err
	}
	scheduled, err := time.Parse(time.RFC3339Nano, scheduledText)
	if err != nil {
		return err
	}
	reported, err := time.Parse(time.RFC3339Nano, reportedText)
	if err != nil {
		return err
	}
	paid, err := time.Parse(time.RFC3339Nano, paidText)
	if err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateManualRequest(alias, "beatriz@example.com", "pedro@example.com", requestAt); err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateProposal("P3", alias, proposalAt, scheduled, true); err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateOrder("O2", "P3", acceptedAt); err != nil {
		return err
	}
	return suite.adminFunnelAddMilestones("O2", reported, paid, false)
}
func (suite *testSuite) adminFunnelZeroIntervalFixture(requestAtText string) error {
	created, err := time.Parse(time.RFC3339Nano, requestAtText)
	if err != nil {
		return err
	}
	if _, err := suite.adminFunnelCreateManualRequest("S1", "ana@example.com", "juan@example.com", created); err != nil {
		return err
	}
	_, err = suite.adminFunnelCreateProposal("P1", "S1", created, created.Add(48*time.Hour), false)
	return err
}

func (suite *testSuite) adminFunnelRequestsTable(table *godog.Table) error {
	if err := requireTableHeaders(table, "solicitud", "consumidor", "prestador", "creada"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 4 {
			return fmt.Errorf("manual request fixture expects four cells")
		}
		created, err := time.Parse(time.RFC3339Nano, row.Cells[3].Value)
		if err != nil {
			return err
		}
		if _, err := suite.adminFunnelCreateManualRequest(row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, created); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelProposalsTable(table *godog.Table) error {
	if err := requireTableHeaders(table, "propuesta", "solicitud", "creada", "programada"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 4 {
			return fmt.Errorf("proposal fixture expects four cells")
		}
		created, err := time.Parse(time.RFC3339Nano, row.Cells[2].Value)
		if err != nil {
			return err
		}
		scheduled, err := time.Parse(time.RFC3339Nano, row.Cells[3].Value)
		if err != nil {
			return err
		}
		if _, err := suite.adminFunnelCreateProposal(row.Cells[0].Value, row.Cells[1].Value, created, scheduled, true); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelOrdersTable(table *godog.Table) error {
	if err := requireTableHeaders(table, "orden", "propuesta", "aceptada", "reporte", "paid_on"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 5 {
			return fmt.Errorf("work order fixture expects five cells")
		}
		accepted, err := time.Parse(time.RFC3339Nano, row.Cells[2].Value)
		if err != nil {
			return err
		}
		if _, err := suite.adminFunnelCreateOrder(row.Cells[0].Value, row.Cells[1].Value, accepted); err != nil {
			return err
		}
		if row.Cells[3].Value != "" {
			reported, err := time.Parse(time.RFC3339Nano, row.Cells[3].Value)
			if err != nil {
				return err
			}
			var paid time.Time
			if row.Cells[4].Value != "" {
				paid, err = time.Parse(time.RFC3339Nano, row.Cells[4].Value)
				if err != nil {
					return err
				}
			}
			if err := suite.adminFunnelAddMilestones(row.Cells[0].Value, reported, paid, false); err != nil {
				return err
			}
		} else if row.Cells[4].Value != "" {
			return fmt.Errorf("a paid fixture order must first persist a completion report")
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelMissingProposalAndReport(noProposal, noReport string) error {
	if suite.adminFunnel.requestAliases[noProposal] == 0 {
		return fmt.Errorf("request %q is missing", noProposal)
	}
	for proposal, request := range suite.adminFunnel.proposalRequests {
		if request == noProposal {
			return fmt.Errorf("request %q unexpectedly has proposal %q", noProposal, proposal)
		}
	}
	orderID := suite.adminFunnel.orderAliases[noReport]
	if orderID == 0 {
		return fmt.Errorf("order %q is missing", noReport)
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return err
	}
	if order.CompletionReport() != nil {
		return fmt.Errorf("order %q unexpectedly has a completion report", noReport)
	}
	return nil
}

func (suite *testSuite) adminFunnelAssessmentByConsumerCategory(email, category string) error {
	_, err := suite.adminFunnelCreateAssessment("A1", email, category, "professional_required", 1, suite.adminFunnelFixtureTime())
	return err
}
func (suite *testSuite) adminFunnelAssessmentTable(email string, table *godog.Table) error {
	if err := requireTableHeaders(table, "evaluación", "versión", "resultado", "rubro", "creada"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 5 {
			return fmt.Errorf("assessment fixture expects five cells")
		}
		version, err := strconv.Atoi(row.Cells[1].Value)
		if err != nil {
			return err
		}
		created, err := time.Parse(time.RFC3339Nano, row.Cells[4].Value)
		if err != nil {
			return err
		}
		if _, err := suite.adminFunnelCreateAssessment(row.Cells[0].Value, email, row.Cells[3].Value, row.Cells[2].Value, version, created); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelCurrentAssessment(alias string) error {
	id := suite.adminFunnel.assessmentAliases[alias]
	if id == 0 {
		return fmt.Errorf("unknown assessment alias %q", alias)
	}
	found, err := suite.conversationRepository.FindByID(suite.scenarioContext, suite.adminFunnel.assessmentConversationID)
	if err != nil {
		return err
	}
	chatbot, ok := found.(*conversation.ChatBotConversation)
	if !ok || chatbot.CurrentAssessment == nil || chatbot.CurrentAssessment.ID != id {
		return fmt.Errorf("conversation current assessment is not persisted alias %q", alias)
	}
	return nil
}

func (suite *testSuite) adminFunnelLinkAssessment(requestAlias, assessmentAlias, currentAlias string) error {
	if err := suite.adminFunnelCurrentAssessment(currentAlias); err != nil {
		return err
	}
	if assessmentAlias == currentAlias {
		return fmt.Errorf("source fixture must distinguish original and current assessment")
	}
	if suite.adminFunnel.assessmentAliases[assessmentAlias] == 0 {
		return fmt.Errorf("unknown source assessment %q", assessmentAlias)
	}
	if suite.adminFunnel.requestAliases[requestAlias] != 0 {
		return fmt.Errorf("request %q is already persisted", requestAlias)
	}
	_, err := suite.adminFunnelCreateAIRequest(requestAlias, "ana@example.com", "juan@example.com", assessmentAlias, suite.adminFunnelFixtureTime().Add(time.Hour))
	return err
}

func (suite *testSuite) adminFunnelSimpleAssessment(alias, outcome string) error {
	category := "Plomería"
	if outcome != "professional_required" {
		category = ""
	}
	_, err := suite.adminFunnelCreateAssessment(alias, "ana@example.com", category, outcome, 1, suite.adminFunnelFixtureTime())
	return err
}
func (suite *testSuite) adminFunnelUnchangedAssessment(alias string) error {
	if err := suite.adminFunnelCurrentAssessment(alias); err != nil {
		return err
	}
	found, err := suite.conversationRepository.FindByID(suite.scenarioContext, suite.adminFunnel.assessmentConversationID)
	if err != nil {
		return err
	}
	chatbot, ok := found.(*conversation.ChatBotConversation)
	if !ok {
		return fmt.Errorf("unchanged fixture requires chatbot conversation")
	}
	userMessage, err := conversation.NewConsumerMessage("Additional detail without replacing the assessment")
	if err != nil {
		return err
	}
	assistantMessage, err := conversation.NewChatbotMessage("The existing professional assessment remains unchanged")
	if err != nil {
		return err
	}
	userMessage.CreatedOn = suite.adminFunnelFixtureTime().Add(time.Minute)
	assistantMessage.CreatedOn = userMessage.CreatedOn
	if err := chatbot.AddTurn(*userMessage, *assistantMessage); err != nil {
		return err
	}
	if err := chatbot.ApplyResponse(conversation.ChatbotResponse{Status: conversation.ChatbotResponseAnswered, Assessment: conversation.ChatbotAssessmentResponse{Action: conversation.ChatbotAssessmentUnchanged}}, nil); err != nil {
		return err
	}
	if _, err := suite.conversationRepository.SaveConversation(suite.scenarioContext, chatbot); err != nil {
		return err
	}
	return suite.adminFunnelCurrentAssessment(alias)
}

func (suite *testSuite) adminFunnelExcludedAssessments(first, second string) error {
	for i, outcome := range []string{first, second} {
		if _, err := suite.adminFunnelCreateAssessment(fmt.Sprintf("excluded-%d", i), "ana@example.com", "", outcome, i+1, suite.adminFunnelFixtureTime().Add(time.Duration(i)*time.Second)); err != nil {
			return err
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelManualRequest(consumerEmail, providerEmail string) error {
	_, err := suite.adminFunnelCreateManualRequest("S1", consumerEmail, providerEmail, suite.adminFunnelFixtureTime())
	return err
}
func (suite *testSuite) adminFunnelLinkedRequest(consumerEmail, providerEmail string) error {
	if len(suite.adminFunnel.assessmentAliases) != 0 {
		return fmt.Errorf("linked request fixture expects to introduce its professional assessment")
	}
	if _, err := suite.adminFunnelCreateAssessment("A1", consumerEmail, "Plomería", "professional_required", 1, suite.adminFunnelFixtureTime()); err != nil {
		return err
	}
	_, err := suite.adminFunnelCreateAIRequest("S2", consumerEmail, providerEmail, "A1", suite.adminFunnelFixtureTime().Add(time.Hour))
	return err
}

// Default fixtures are comfortably inside the thirty-day window; downstream
// milestones remain before observation unless the scenario specifies dates.
func (suite *testSuite) adminFunnelFixtureTime() time.Time {
	return suite.clock.Now().UTC().Add(-10 * 24 * time.Hour)
}

func (suite *testSuite) adminFunnelOrderOrigin(orderAlias, assessmentAlias string) error {
	orderID := suite.adminFunnel.orderAliases[orderAlias]
	assessmentID := suite.adminFunnel.assessmentAliases[assessmentAlias]
	if orderID == 0 || assessmentID == 0 {
		return fmt.Errorf("order or assessment alias is missing")
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return err
	}
	for proposalAlias, proposalID := range suite.adminFunnel.proposalAliases {
		if proposalID != order.ServiceProposalID() {
			continue
		}
		requestAlias := suite.adminFunnel.proposalRequests[proposalAlias]
		request, err := suite.jobRequestRepository.FindByID(suite.adminFunnel.requestAliases[requestAlias])
		if err != nil {
			return err
		}
		if request.SourceAssessmentID == nil || *request.SourceAssessmentID != assessmentID {
			return fmt.Errorf("order %q does not originate in assessment %q", orderAlias, assessmentAlias)
		}
		return nil
	}
	return fmt.Errorf("order %q has no tracked proposal branch", orderAlias)
}

func (suite *testSuite) adminFunnelPersistCheckout(alias, proposalAlias string, created time.Time) (*payment.Intent, error) {
	proposalID := suite.adminFunnel.proposalAliases[proposalAlias]
	if proposalID == 0 {
		return nil, fmt.Errorf("unknown checkout proposal %q", proposalAlias)
	}
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, proposalID)
	if err != nil {
		return nil, err
	}
	intent, err := payment.NewBookingDepositIntent(suite.adminPaymentID(alias), proposalID, proposal.BookingTerms, created)
	if err != nil {
		return nil, err
	}
	if err := intent.MarkCheckoutReady("preference-"+alias, "https://checkout.example.test/"+alias, suite.clock.Now().Add(time.Hour), created); err != nil {
		return nil, err
	}
	if err := suite.paymentIntentRepository.Save(suite.scenarioContext, intent); err != nil {
		return nil, err
	}
	suite.adminPaymentState().intents[alias] = intent
	return intent, nil
}

func (suite *testSuite) adminFunnelValidActivity() error {
	_, err := suite.adminFunnelCreateAssessment("A1", "ana@example.com", "Plomería", "professional_required", 1, suite.adminFunnelFixtureTime())
	return err
}

func (suite *testSuite) adminFunnelCompleteActivity() error {
	if err := suite.adminFunnelValidActivity(); err != nil {
		return err
	}
	base := suite.adminFunnelFixtureTime()
	if _, err := suite.adminFunnelCreateAIRequest("S1", "ana@example.com", "juan@example.com", "A1", base.Add(time.Hour)); err != nil {
		return err
	}
	return suite.adminFunnelCreateProposalOrder("P1", "O1", "S1", base.Add(2*time.Hour))
}

func (suite *testSuite) adminFunnelChronologyAnomaly(requestAlias string) error {
	request, err := suite.jobRequestRepository.FindByID(suite.adminFunnel.requestAliases[requestAlias])
	if err != nil {
		return err
	}
	// FindByID intentionally omits the conversation. The collection reader
	// hydrates it, allowing this Given to verify the persisted branch relation.
	proposals, err := repositories.NewServiceProposalRepository(suite.database).FindByUserID(suite.scenarioContext, request.ConsumerID)
	if err != nil {
		return err
	}
	var first, second *serviceproposal.ServiceProposal
	for _, proposal := range proposals {
		if proposal.ID == suite.adminFunnel.proposalAliases["P1"] {
			first = proposal
		}
		if proposal.ID == suite.adminFunnel.proposalAliases["P2"] {
			second = proposal
		}
	}
	if first == nil || second == nil || first.Conversation == nil || second.Conversation == nil {
		return fmt.Errorf("historical request proposals are not persisted with conversation links")
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.adminFunnel.orderAliases["O1"])
	if err != nil {
		return err
	}
	if first.Conversation.ID() != request.ConversationID || second.Conversation.ID() != request.ConversationID || !first.CreatedOn.Before(request.CreatedOn) || !second.CreatedOn.After(request.CreatedOn) {
		return fmt.Errorf("historical request proposal chronology is not persisted")
	}
	if order.CompletionReport() == nil || !order.CompletionReport().ReportedOn().Before(order.AcceptedOn()) || !order.PaidOn().IsZero() {
		return fmt.Errorf("historical order report chronology is not persisted")
	}
	return nil
}
