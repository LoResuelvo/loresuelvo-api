package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type detailProposalState struct {
	proposals map[string]map[string]string
	intents   map[string]map[string]string
}

func (suite *testSuite) detailProposals() *detailProposalState {
	if suite.detailProposal.proposals == nil {
		suite.detailProposal.proposals = map[string]map[string]string{}
		suite.detailProposal.intents = map[string]map[string]string{}
	}
	return &suite.detailProposal
}

func registerAdminOperationDetailProposalSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe la siguiente propuesta de servicio:$`, suite.thereAreDetailServiceProposals)
	sc.Step(`^que existe la siguiente orden de trabajo:$`, suite.thereAreInboxWorkOrders)
	sc.Step(`^que "([^"]*)" tiene los siguientes intentos de pago de seña:$`, suite.detailProposalHasPaymentIntents)
	sc.Step(`^que "([^"]*)" tiene el siguiente intento de pago de seña:$`, suite.detailProposalHasPaymentIntents)
	sc.Step(`^que "([^"]*)" tiene el siguiente intento de pago de saldo:$`, suite.detailOrderHasPaymentIntents)
	sc.Step(`^consulto el detalle administrativo de la operación "(jr-|sp-)" seguido del ID persistido de "([^"]*)"$`, suite.queryDetailByPersistedIdentity)
	sc.Step(`^el detalle primario informa la identidad "jr-" seguida del ID persistido de "([^"]*)" y la solicitud "([^"]*)"$`, suite.detailPrimaryRequestIdentity)
	sc.Step(`^las propuestas relacionadas aparecen separadas con identidades "jr-" seguido del ID persistido de "([^"]*)" para "([^"]*)" y "sp-" seguido del ID persistido de "([^"]*)"$`, suite.detailRelatedProposalIdentities)
	sc.Step(`^la propuesta "([^"]*)" informa estado "([^"]*)", descripción, precio total (\d+) y moneda "([^"]*)", creación "([^"]*)", fecha programada "([^"]*)" y duración (\d+) minutos$`, suite.detailProposalFields)
	sc.Step(`^cada propuesta informa sus términos de contratación persistidos: seña, comisión total e inicial, saldo de servicio y saldo de comisión, sin recalcularlos desde la propuesta hermana$`, suite.detailAllProposalTerms)
	sc.Step(`^la cronología y los hitos de pago principales contienen solamente los de "([^"]*)" y su orden "([^"]*)"$`, suite.detailPrimaryTimelineAndPayments)
	sc.Step(`^los hitos incluyen las referencias internas de ambos intentos de seña y del intento de saldo con propósito, estado e instante, sin confundir sus estados con los de transacciones ni exponer montos, URLs de checkout, tokens o payloads del proveedor$`, suite.detailThreePaymentMilestones)
	sc.Step(`^la orden "([^"]*)" informa estado "([^"]*)", instante de aceptación "([^"]*)", finalización informada "([^"]*)" e instante de saldo pagado nulo$`, suite.detailOrderFields)
	sc.Step(`^el detalle primario informa la propuesta "([^"]*)" con identidad "sp-" seguida del ID persistido de "([^"]*)"$`, suite.detailPrimaryProposalIdentity)
	sc.Step(`^la propuesta hermana "([^"]*)" se informa por separado con identidad "jr-" seguida del ID persistido de "([^"]*)"$`, suite.detailSiblingIdentity)
	sc.Step(`^la cronología principal incluye únicamente los eventos de "([^"]*)" y su orden "([^"]*)", no los eventos de "([^"]*)"$`, suite.detailPrimaryProposalTimeline)
	sc.Step(`^los hitos de pago principales incluyen "([^"]*)" y no incluyen la referencia hermana "([^"]*)"$`, suite.detailOnlyPrimaryPayment)
	sc.Step(`^la propuesta primaria conserva sus términos persistidos de seña (\d+), comisión total (\d+), comisión inicial (\d+), saldo de servicio (\d+) y saldo de comisión (\d+)$`, suite.detailPrimaryTerms)
}

func detailInt(row map[string]string, key string) (int64, error) {
	n, err := strconv.ParseInt(row[key], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: %w", key, row[key], err)
	}
	return n, nil
}

func (suite *testSuite) thereAreDetailServiceProposals(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if _, hasTerms := rows[0]["precio total"]; !hasTerms {
		return suite.thereAreInboxServiceProposals(table)
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			if err := suite.createDetailProposal(row); err != nil {
				return err
			}
		}
		return nil
	})
}

func (suite *testSuite) createDetailProposal(row map[string]string) error {
	label, requestLabel := row["propuesta"], row["solicitud"]
	request, ok := suite.operationInbox.requests[requestLabel]
	if !ok {
		return fmt.Errorf("unknown request %q", requestLabel)
	}
	if _, ok := suite.operationInbox.proposals[label]; ok {
		return fmt.Errorf("duplicate proposal %q", label)
	}
	created, err := parseInboxInstant(row["creada"])
	if err != nil {
		return err
	}
	scheduled, err := parseInboxInstant(row["fecha programada"])
	if err != nil {
		return err
	}
	duration, err := detailInt(row, "duración")
	if err != nil {
		return err
	}
	amount, err := detailInt(row, "precio total")
	if err != nil {
		return err
	}
	terms, err := detailProposalBookingTerms(row, amount, scheduled)
	if err != nil {
		return err
	}
	providerUser, err := suite.userRepository.FindByAuthID(auth0IDForProviderEmail(request.providerEmail))
	if err != nil {
		return err
	}
	consumerUser, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(request.consumerEmail))
	if err != nil {
		return err
	}
	proposalProvider, ok := providerUser.(*provider.Provider)
	if !ok {
		return fmt.Errorf("provider fixture has type %T", providerUser)
	}
	proposalConsumer, ok := consumerUser.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("consumer fixture has type %T", consumerUser)
	}
	conversation, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.conversationID)
	if err != nil {
		return err
	}
	if err := suite.setInboxFixtureClock(created); err != nil {
		return err
	}
	proposal, err := serviceproposal.NewServiceProposal(proposalProvider, proposalConsumer, conversation, scheduled, row["descripción"], terms, suite.clock, int(duration))
	if err != nil {
		return err
	}
	if row["estado"] != "pending" && row["estado"] != "accepted" {
		return fmt.Errorf("unsupported proposal status %q", row["estado"])
	}
	saved, err := repositories.NewServiceProposalRepository(suite.database).Save(proposal)
	if err != nil {
		return err
	}
	suite.operationInbox.proposals[label] = inboxProposalFixture{id: saved.ID, requestLabel: requestLabel, createdOn: created}
	suite.operationInbox.acceptedProposal[label] = row["estado"] == "accepted"
	suite.detailProposals().proposals[label] = row
	return nil
}

func (suite *testSuite) detailProposalHasPaymentIntents(label string, table *godog.Table) error {
	return suite.createDetailPaymentIntents(label, false, table)
}
func (suite *testSuite) detailOrderHasPaymentIntents(label string, table *godog.Table) error {
	return suite.createDetailPaymentIntents(label, true, table)
}

func (suite *testSuite) createDetailPaymentIntents(label string, isOrder bool, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	proposalLabel := label
	if isOrder {
		orderID, ok := suite.operationInbox.orders[label]
		if !ok {
			return fmt.Errorf("unknown order %q", label)
		}
		order, err := suite.workOrderRepository.FindByID(context.Background(), orderID)
		if err != nil {
			return err
		}
		for name, fixture := range suite.operationInbox.proposals {
			if fixture.id == order.ServiceProposalID() {
				proposalLabel = name
				break
			}
		}
	}
	fixture, ok := suite.operationInbox.proposals[proposalLabel]
	if !ok {
		return fmt.Errorf("unknown proposal %q", proposalLabel)
	}
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(context.Background(), fixture.id)
	if err != nil {
		return err
	}
	for _, row := range rows {
		alias := row["referencia interna"]
		id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("admin-operation-detail/"+alias)).String()
		created, err := parseInboxInstant(row["creada"])
		if err != nil {
			return err
		}
		var intent *payment.Intent
		if isOrder {
			if row["propósito"] != string(payment.PurposeServiceBalance) {
				return fmt.Errorf("unexpected balance purpose %q", row["propósito"])
			}
			order, err := suite.workOrderRepository.FindByID(context.Background(), suite.operationInbox.orders[label])
			if err != nil {
				return err
			}
			intent, err = payment.NewServiceBalanceIntent(id, order, created)
			if err != nil {
				return err
			}
		} else {
			if row["propósito"] != string(payment.PurposeBookingDeposit) {
				return fmt.Errorf("unexpected deposit purpose %q", row["propósito"])
			}
			intent, err = payment.NewBookingDepositIntent(id, proposal.ID, proposal.BookingTerms, created)
			if err != nil {
				return err
			}
		}
		switch row["estado"] {
		case string(payment.StatusRejected), string(payment.StatusPaid), string(payment.StatusCheckoutReady):
			if err := intent.MarkCheckoutReady("checkout-"+id, "https://checkout.test/"+id, created.Add(time.Hour), created); err != nil {
				return err
			}
			if row["estado"] != string(payment.StatusCheckoutReady) {
				status := payment.ExternalPaymentStatusRejected
				if row["estado"] == string(payment.StatusPaid) {
					status = payment.ExternalPaymentStatusApproved
				}
				external := payment.ExternalPayment{ID: "external-" + id, SellerAccountID: "seller-test", ExternalReference: id, Status: status, Currency: intent.Currency, AmountCents: intent.TotalAmountCents}
				if status == payment.ExternalPaymentStatusApproved {
					err = intent.MarkPaid(external, created)
				} else {
					err = intent.MarkRejected(external, created)
				}
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported intent status %q", row["estado"])
		}
		if err := suite.paymentIntentRepository.Save(suite.scenarioContext, intent); err != nil {
			return err
		}
		copyRow := make(map[string]string, len(row)+2)
		for key, value := range row {
			copyRow[key] = value
		}
		copyRow["id"] = id
		copyRow["propuesta"] = proposalLabel
		suite.detailProposals().intents[alias] = copyRow
	}
	return nil
}

func (suite *testSuite) queryDetailByPersistedIdentity(prefix, label string) error {
	var id int
	if prefix == "jr-" {
		request, ok := suite.operationInbox.requests[label]
		if !ok {
			return fmt.Errorf("unknown request %q", label)
		}
		id = request.id
	} else {
		proposal, ok := suite.operationInbox.proposals[label]
		if !ok {
			return fmt.Errorf("unknown proposal %q", label)
		}
		id = proposal.id
	}
	return suite.queryAdminOperationDetail(fmt.Sprintf("%s%d", prefix, id))
}

func (suite *testSuite) detailJSON() (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(suite.lastBody, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func detailMap(value any, field string) (map[string]any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected object, got %T", field, value)
	}
	return m, nil
}

func detailArray(value any, field string) ([]any, error) {
	a, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected array, got %T", field, value)
	}
	return a, nil
}

func detailEqual(m map[string]any, field string, expected any) error {
	if !reflect.DeepEqual(m[field], expected) {
		return fmt.Errorf("%s: got %#v, want %#v", field, m[field], expected)
	}
	return nil
}

func (suite *testSuite) detailPrimaryRequestIdentity(label, requestLabel string) error {
	if label != requestLabel {
		return fmt.Errorf("request aliases differ")
	}
	request := suite.operationInbox.requests[label]
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	if err := detailEqual(detail, "id", fmt.Sprintf("jr-%d", request.id)); err != nil {
		return err
	}
	resource, err := detailMap(detail["job_request"], "job_request")
	if err != nil {
		return err
	}
	return detailEqual(resource, "id", float64(request.id))
}

func (suite *testSuite) detailPrimaryProposalIdentity(label, persistedLabel string) error {
	if label != persistedLabel {
		return fmt.Errorf("proposal aliases differ")
	}
	proposal := suite.operationInbox.proposals[label]
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	if err := detailEqual(detail, "id", fmt.Sprintf("sp-%d", proposal.id)); err != nil {
		return err
	}
	resource, err := detailMap(detail["service_proposal"], "service_proposal")
	if err != nil {
		return err
	}
	return detailEqual(resource, "id", float64(proposal.id))
}

func (suite *testSuite) detailRelatedProposalIdentities(requestLabel, primary, sibling string) error {
	if suite.operationInbox.proposals[primary].requestLabel != requestLabel || suite.operationInbox.proposals[sibling].requestLabel != requestLabel {
		return fmt.Errorf("proposals not linked to %q", requestLabel)
	}
	return suite.assertDetailRelatedIdentities(map[string]int{fmt.Sprintf("jr-%d", suite.operationInbox.requests[requestLabel].id): suite.operationInbox.proposals[primary].id, fmt.Sprintf("sp-%d", suite.operationInbox.proposals[sibling].id): suite.operationInbox.proposals[sibling].id})
}

func (suite *testSuite) detailSiblingIdentity(sibling, requestLabel string) error {
	primary := ""
	for label, proposal := range suite.operationInbox.proposals {
		if label != sibling && proposal.requestLabel == requestLabel {
			primary = label
			break
		}
	}
	if primary == "" {
		return fmt.Errorf("no primary proposal for request %q", requestLabel)
	}
	return suite.assertDetailRelatedIdentities(map[string]int{
		fmt.Sprintf("jr-%d", suite.operationInbox.requests[requestLabel].id): suite.operationInbox.proposals[sibling].id,
		fmt.Sprintf("sp-%d", suite.operationInbox.proposals[primary].id):     suite.operationInbox.proposals[primary].id,
	})
}

func (suite *testSuite) assertDetailRelatedIdentities(expected map[string]int) error {
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	items, err := detailArray(detail["related_proposals"], "related_proposals")
	if err != nil {
		return err
	}
	seen := map[string]int{}
	for _, item := range items {
		m, err := detailMap(item, "related proposal")
		if err != nil {
			return err
		}
		identity, ok := m["operation_id"].(string)
		if !ok {
			return fmt.Errorf("missing related operation_id")
		}
		id, ok := m["id"].(float64)
		if !ok {
			return fmt.Errorf("missing related proposal id")
		}
		seen[identity] = int(id)
	}
	for identity, id := range expected {
		if seen[identity] != id {
			return fmt.Errorf("related proposal %q: got %d, want %d (all: %#v)", identity, seen[identity], id, seen)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("unexpected related proposal identities: %#v", seen)
	}
	return nil
}

func (suite *testSuite) detailProposalResource(label string) (map[string]any, error) {
	detail, err := suite.detailJSON()
	if err != nil {
		return nil, err
	}
	primary, err := detailMap(detail["service_proposal"], "service_proposal")
	if err == nil && primary["id"] == float64(suite.operationInbox.proposals[label].id) {
		return primary, nil
	}
	items, err := detailArray(detail["related_proposals"], "related_proposals")
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		m, err := detailMap(item, "related proposal")
		if err != nil {
			return nil, err
		}
		if m["id"] == float64(suite.operationInbox.proposals[label].id) {
			return m, nil
		}
	}
	return nil, fmt.Errorf("proposal %q absent from detail", label)
}

func (suite *testSuite) detailProposalFields(label, status, amountText, currency, created, scheduled, durationText string) error {
	row, ok := suite.detailProposals().proposals[label]
	if !ok {
		return fmt.Errorf("missing proposal fixture %q", label)
	}
	resource, err := suite.detailProposalResource(label)
	if err != nil {
		return err
	}
	checks := map[string]any{"id": float64(suite.operationInbox.proposals[label].id), "status": status, "description": row["descripción"], "currency": currency, "created_on": created, "scheduled_on": scheduled}
	amount, err := strconv.ParseFloat(amountText, 64)
	if err != nil {
		return err
	}
	checks["amount_cents"] = amount
	duration, err := strconv.ParseFloat(durationText, 64)
	if err != nil {
		return err
	}
	checks["estimated_duration_minutes"] = duration
	for key, want := range checks {
		if err := detailEqual(resource, key, want); err != nil {
			return fmt.Errorf("proposal %s: %w", label, err)
		}
	}
	return nil
}

func (suite *testSuite) detailAllProposalTerms() error {
	for label, row := range suite.detailProposals().proposals {
		resource, err := suite.detailProposalResource(label)
		if err != nil {
			return err
		}
		if err := detailCheckTerms(resource, row); err != nil {
			return fmt.Errorf("proposal %s: %w", label, err)
		}
	}
	return nil
}

func detailCheckTerms(resource map[string]any, row map[string]string) error {
	for field, key := range map[string]string{"deposit_cents": "seña", "platform_fee_total_cents": "comisión total", "platform_fee_due_now_cents": "comisión inicial", "service_balance_cents": "saldo servicio", "platform_fee_balance_cents": "saldo comisión"} {
		n, err := detailInt(row, key)
		if err != nil {
			return err
		}
		if err := detailEqual(resource, field, float64(n)); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) detailPrimaryTerms(deposit, fee, feeNow, balance, feeBalance string) error {
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	resource, err := detailMap(detail["service_proposal"], "service_proposal")
	if err != nil {
		return err
	}
	return detailCheckTerms(resource, map[string]string{"seña": deposit, "comisión total": fee, "comisión inicial": feeNow, "saldo servicio": balance, "saldo comisión": feeBalance})
}

func (suite *testSuite) detailMilestones() ([]map[string]any, error) {
	detail, err := suite.detailJSON()
	if err != nil {
		return nil, err
	}
	items, err := detailArray(detail["payment_milestones"], "payment_milestones")
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, err := detailMap(item, "payment_milestone")
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, nil
}

func (suite *testSuite) detailPrimaryTimelineAndPayments(proposalLabel, orderLabel string) error {
	if _, ok := suite.operationInbox.orders[orderLabel]; !ok {
		return fmt.Errorf("unknown order %q", orderLabel)
	}
	items, err := suite.detailMilestones()
	if err != nil {
		return err
	}
	expected := 0
	for _, row := range suite.detailProposals().intents {
		if row["propuesta"] == proposalLabel {
			expected++
		}
	}
	if len(items) != expected {
		return fmt.Errorf("got %d payment milestones, want %d for %s", len(items), expected, proposalLabel)
	}
	for _, item := range items {
		id, ok := item["id"].(string)
		if !ok {
			return fmt.Errorf("milestone missing id")
		}
		matched := false
		for _, row := range suite.detailProposals().intents {
			if row["id"] == id && row["propuesta"] == proposalLabel {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("foreign milestone %q", id)
		}
	}
	return suite.assertTimelineSources(proposalLabel, orderLabel)
}

func (suite *testSuite) assertTimelineSources(proposalLabel, orderLabel string) error {
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	items, err := detailArray(detail["timeline"], "timeline")
	if err != nil {
		return err
	}
	proposal := suite.operationInbox.proposals[proposalLabel]
	orderID := suite.operationInbox.orders[orderLabel]
	order, err := suite.workOrderRepository.FindByID(context.Background(), orderID)
	if err != nil {
		return err
	}
	expected := map[string]int{}
	key := func(kind, sourceType, sourceID, occurred string) string {
		return strings.Join([]string{kind, sourceType, sourceID, occurred}, "\x00")
	}
	add := func(kind, sourceType, sourceID string, occurred time.Time) {
		expected[key(kind, sourceType, sourceID, occurred.UTC().Format(time.RFC3339Nano))]++
	}
	if strings.HasPrefix(fmt.Sprint(detail["id"]), "jr-") {
		request := suite.operationInbox.requests[proposal.requestLabel]
		add("job_request_created", "job_request", strconv.Itoa(request.id), request.createdOn)
	}
	add("service_proposal_created", "service_proposal", strconv.Itoa(proposal.id), proposal.createdOn)
	add("work_order_accepted", "work_order", strconv.Itoa(orderID), order.AcceptedOn())
	if report := order.CompletionReport(); report != nil {
		add("completion_reported", "work_order", strconv.Itoa(orderID), report.ReportedOn())
	}
	if paidOn := order.PaidOn(); !paidOn.IsZero() {
		add("balance_paid", "work_order", strconv.Itoa(orderID), paidOn)
	}
	for _, row := range suite.detailProposals().intents {
		if row["propuesta"] == proposalLabel {
			created, err := parseInboxInstant(row["creada"])
			if err != nil {
				return err
			}
			add("payment_intent_created", "payment_intent", row["id"], created)
		}
	}
	if len(items) != len(expected) {
		return fmt.Errorf("timeline has %d events, want %d: %#v", len(items), len(expected), items)
	}
	for _, item := range items {
		event, err := detailMap(item, "timeline event")
		if err != nil {
			return err
		}
		value := key(fmt.Sprint(event["type"]), fmt.Sprint(event["source_type"]), fmt.Sprint(event["source_id"]), fmt.Sprint(event["occurred_on"]))
		if expected[value] == 0 {
			return fmt.Errorf("unexpected timeline event: %#v", event)
		}
		expected[value]--
	}
	return nil
}

func (suite *testSuite) detailThreePaymentMilestones() error {
	items, err := suite.detailMilestones()
	if err != nil {
		return err
	}
	if len(items) != 3 {
		return fmt.Errorf("got %d milestones, want 3", len(items))
	}
	for alias, row := range suite.detailProposals().intents {
		matched := false
		for _, item := range items {
			if item["id"] != row["id"] {
				continue
			}
			matched = true
			for field, key := range map[string]string{"purpose": "propósito", "status": "estado", "created_on": "creada"} {
				if err := detailEqual(item, field, row[key]); err != nil {
					return fmt.Errorf("%s: %w", alias, err)
				}
			}
			for field := range item {
				if field != "id" && field != "purpose" && field != "status" && field != "created_on" {
					return fmt.Errorf("payment milestone exposes %q", field)
				}
			}
		}
		if !matched {
			return fmt.Errorf("missing milestone %s", alias)
		}
	}
	return nil
}

func (suite *testSuite) detailOrderFields(label, status, accepted, reported string) error {
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	order, err := detailMap(detail["work_order"], "work_order")
	if err != nil {
		return err
	}
	for key, want := range map[string]any{"id": float64(suite.operationInbox.orders[label]), "status": status, "accepted_on": accepted, "completion_reported_on": reported, "balance_paid_on": nil} {
		if err := detailEqual(order, key, want); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) detailResponseHasNoChat() error {
	text := strings.ToLower(string(suite.lastBody))
	for _, phrase := range []string{"\"messages\"", "\"conversation_messages\"", "\"message_content\"", "mensaje privado de la conversación"} {
		if strings.Contains(text, phrase) {
			return fmt.Errorf("detail leaks chat: %s", phrase)
		}
	}
	return nil
}

func (suite *testSuite) detailPrimaryProposalTimeline(primary, order, sibling string) error {
	if err := suite.assertTimelineSources(primary, order); err != nil {
		return err
	}
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	items, err := detailArray(detail["timeline"], "timeline")
	if err != nil {
		return err
	}
	siblingID := suite.operationInbox.proposals[sibling].id
	for _, item := range items {
		event, _ := detailMap(item, "timeline event")
		if event["source_id"] == float64(siblingID) || event["source_id"] == strconv.Itoa(siblingID) {
			return fmt.Errorf("sibling proposal appears in primary timeline")
		}
	}
	return nil
}

func (suite *testSuite) detailOnlyPrimaryPayment(primaryAlias, siblingAlias string) error {
	primary, ok := suite.detailProposals().intents[primaryAlias]
	if !ok {
		return fmt.Errorf("unknown intent %q", primaryAlias)
	}
	sibling, ok := suite.detailProposals().intents[siblingAlias]
	if !ok {
		return fmt.Errorf("unknown intent %q", siblingAlias)
	}
	items, err := suite.detailMilestones()
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("got %d milestones, want 1", len(items))
	}
	if items[0]["id"] != primary["id"] || items[0]["id"] == sibling["id"] {
		return fmt.Errorf("wrong primary payment milestones: %#v", items)
	}
	return nil
}

func detailProposalBookingTerms(row map[string]string, amount int64, scheduled time.Time) (serviceproposal.BookingTerms, error) {
	hasExplicitTerms := false
	for _, key := range []string{"seña", "comisión total", "comisión inicial", "saldo servicio", "saldo comisión"} {
		if _, ok := row[key]; ok {
			hasExplicitTerms = true
		}
	}
	if !hasExplicitTerms {
		terms, err := serviceproposal.NewBookingPolicy().Calculate(amount, scheduled)
		if err != nil {
			return serviceproposal.BookingTerms{}, err
		}
		if row["moneda"] != terms.Currency() {
			return serviceproposal.BookingTerms{}, fmt.Errorf("unsupported proposal fixture currency %q", row["moneda"])
		}
		return terms, nil
	}
	deposit, err := detailInt(row, "seña")
	if err != nil {
		return serviceproposal.BookingTerms{}, err
	}
	fee, err := detailInt(row, "comisión total")
	if err != nil {
		return serviceproposal.BookingTerms{}, err
	}
	feeNow, err := detailInt(row, "comisión inicial")
	if err != nil {
		return serviceproposal.BookingTerms{}, err
	}
	remaining, err := detailInt(row, "saldo servicio")
	if err != nil {
		return serviceproposal.BookingTerms{}, err
	}
	feeRemaining, err := detailInt(row, "saldo comisión")
	if err != nil {
		return serviceproposal.BookingTerms{}, err
	}
	if amount-deposit != remaining || fee-feeNow != feeRemaining {
		return serviceproposal.BookingTerms{}, fmt.Errorf("inconsistent persisted terms for %q", row["propuesta"])
	}
	return serviceproposal.NewBookingTerms(row["moneda"], amount, deposit, fee, feeNow, scheduled.Add(-24*time.Hour))
}

func TestDetailProposalBookingTermsUsesPolicyWhenTermColumnsAreAbsent(t *testing.T) {
	scheduled := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	expected, err := serviceproposal.NewBookingPolicy().Calculate(25000000, scheduled)
	require.NoError(t, err)
	found, err := detailProposalBookingTerms(map[string]string{"moneda": "ARS"}, 25000000, scheduled)
	require.NoError(t, err)
	require.Equal(t, expected, found)
}
func TestDetailProposalBookingTermsPreservesExplicitTerms(t *testing.T) {
	scheduled := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	row := map[string]string{"moneda": "ARS", "seña": "20000", "comisión total": "7000", "comisión inicial": "1000", "saldo servicio": "80000", "saldo comisión": "6000"}
	found, err := detailProposalBookingTerms(row, 100000, scheduled)
	require.NoError(t, err)
	expected, err := serviceproposal.NewBookingTerms("ARS", 100000, 20000, 7000, 1000, scheduled.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, expected, found)
}
func TestDetailProposalBookingTermsRejectsPartialExplicitTerms(t *testing.T) {
	_, err := detailProposalBookingTerms(map[string]string{"moneda": "ARS", "seña": "20000"}, 100000, time.Now().Add(72*time.Hour))
	require.Error(t, err)
}
