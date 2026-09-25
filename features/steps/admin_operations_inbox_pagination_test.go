package steps_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const maxInboxPagesWalked = 10

func registerAdminOperationsInboxPaginationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" tiene una propuesta con dos intentos de pago de la seña y una orden con finalización informada con tres imágenes$`, suite.inboxRequestHasOneToManyRelations)
	sc.Step(`^recorro la bandeja con páginas de (\d+) operaciones$`, suite.walkInboxPages)
	sc.Step(`^obtengo (\d+) páginas y sólo la última no tiene cursor siguiente$`, suite.inboxWalkHasPages)
	sc.Step(`^las páginas contienen las operaciones (`+quotedLabelList+`) una sola vez cada una$`, suite.inboxPagesContainOperationsOnce)
	sc.Step(`^las operaciones se ordenan por inicio descendente y las operaciones (`+quotedLabelList+`) se desempatan por identificador descendente$`, suite.inboxPagesAreOrdered)
	sc.Step(`^que existen (\d+) solicitudes de trabajo pendientes sintéticas entre pares distintos de consumidor y prestador$`, suite.thereAreSyntheticInboxJobRequests)
	sc.Step(`^consulto la bandeja administrativa de contrataciones sin indicar límite$`, suite.queryOperationsInbox)
	sc.Step(`^consulto la bandeja administrativa de contrataciones con límite (\d+)$`, suite.queryInboxWithLimit)
	sc.Step(`^la página contiene (\d+) operaciones y entrega un cursor siguiente$`, suite.inboxPageHasCountAndCursor)
	sc.Step(`^consulto la bandeja administrativa de contrataciones con los parámetros "([^"]*)"$`, suite.queryInboxWithRawParameters)
}

func (suite *testSuite) inboxRequestHasOneToManyRelations(requestLabel string) error {
	request, exists := suite.operationInbox.requests[requestLabel]
	if !exists {
		return fmt.Errorf("unknown job request label %q", requestLabel)
	}
	proposalLabel, orderLabel := "P-"+requestLabel, "O-"+requestLabel
	proposedOn := request.createdOn.Add(24 * time.Hour)
	scheduledOn := proposedOn.Add(25 * time.Hour)
	if err := suite.withInboxFixtureClock(func() error {
		if err := suite.createInboxServiceProposal(proposalLabel, requestLabel, proposedOn, scheduledOn, 60, string(serviceproposal.StatusAccepted)); err != nil {
			return err
		}
		return suite.createInboxWorkOrder(orderLabel, proposalLabel, proposedOn.Add(time.Hour), string(workorder.StatusScheduled), "", "")
	}); err != nil {
		return err
	}
	if err := suite.inboxProposalHadRejectedThenApprovedDeposit(proposalLabel); err != nil {
		return err
	}
	return suite.withInboxFixtureClock(func() error {
		order, err := suite.workOrderRepository.FindByID(context.Background(), suite.operationInbox.orders[orderLabel])
		if err != nil {
			return err
		}
		return suite.reportInboxWorkOrderCompletion(orderLabel, order, scheduledOn.Add(2*time.Hour), 3)
	})
}

func (suite *testSuite) walkInboxPages(pageSize int) error {
	suite.operationInbox.pages = nil
	query := url.Values{"limit": {strconv.Itoa(pageSize)}}
	for range maxInboxPagesWalked {
		if err := suite.queryOperationsInboxWith(query); err != nil {
			return err
		}
		page, err := suite.decodedInboxPage()
		if err != nil {
			return err
		}
		suite.operationInbox.pages = append(suite.operationInbox.pages, page)
		if page.NextCursor == nil {
			return nil
		}
		query = url.Values{"cursor": {*page.NextCursor}}
	}
	return fmt.Errorf("the inbox kept returning a next cursor after %d pages", maxInboxPagesWalked)
}

func (suite *testSuite) inboxWalkHasPages(expected int) error {
	pages := suite.operationInbox.pages
	if len(pages) != expected {
		return fmt.Errorf("expected %d pages, got %d", expected, len(pages))
	}
	for index, page := range pages[:len(pages)-1] {
		if page.NextCursor == nil {
			return fmt.Errorf("page %d has no next cursor", index+1)
		}
	}
	return nil
}

func (suite *testSuite) walkedInboxOperations() []inboxOperationResponse {
	operations := []inboxOperationResponse{}
	for _, page := range suite.operationInbox.pages {
		operations = append(operations, page.Operations...)
	}
	return operations
}

func (suite *testSuite) inboxPagesContainOperationsOnce(list string) error {
	expected := []string{}
	for _, label := range quotedLabels(list) {
		operationID, err := suite.inboxOperationID(label)
		if err != nil {
			return err
		}
		expected = append(expected, operationID)
	}
	actual := []string{}
	for _, found := range suite.walkedInboxOperations() {
		actual = append(actual, found.ID)
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if strings.Join(expected, ",") != strings.Join(actual, ",") {
		return fmt.Errorf("expected each of %v once across pages, got %v", expected, actual)
	}
	return nil
}

func (suite *testSuite) inboxPagesAreOrdered(tiedList string) error {
	operations := suite.walkedInboxOperations()
	for index := 1; index < len(operations); index++ {
		if operations[index].StartedOn.After(operations[index-1].StartedOn) {
			return fmt.Errorf("operation %s starts after %s", operations[index].ID, operations[index-1].ID)
		}
	}
	tied := map[string]bool{}
	for _, label := range quotedLabels(tiedList) {
		operationID, err := suite.inboxOperationID(label)
		if err != nil {
			return err
		}
		tied[operationID] = true
	}
	previous := -1
	for _, found := range operations {
		if !tied[found.ID] {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(found.ID, "jr-"))
		if err != nil {
			return fmt.Errorf("tied operation %s is not a job request operation", found.ID)
		}
		if previous != -1 && id >= previous {
			return fmt.Errorf("tied operations are not in descending identifier order at %s", found.ID)
		}
		previous = id
	}
	return nil
}

func (suite *testSuite) thereAreSyntheticInboxJobRequests(count int) error {
	const providers = 10
	consumers := (count + providers - 1) / providers
	for index := range providers {
		if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(
			fmt.Sprintf("prestador.sintetico%d@example.com", index), "Prestador", fmt.Sprintf("Sintético%d", index), "Plomería",
		); err != nil {
			return err
		}
	}
	for index := range consumers {
		if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(
			fmt.Sprintf("consumidor.sintetico%d@example.com", index), "Consumidor", fmt.Sprintf("Sintético%d", index),
		); err != nil {
			return err
		}
	}
	return suite.withInboxFixtureClock(func() error {
		for index := range count {
			providerID, err := suite.providerIDByEmail(fmt.Sprintf("prestador.sintetico%d@example.com", index%providers))
			if err != nil {
				return err
			}
			suite.currentAuth0ID = auth0IDForConsumerEmail(fmt.Sprintf("consumidor.sintetico%d@example.com", index/providers))
			if err := suite.requestJobRequest(jobRequestCreationRequest{ProviderID: providerID, Title: fmt.Sprintf("Solicitud sintética %d", index)}); err != nil {
				return err
			}
			if suite.lastStatus != http.StatusCreated {
				return fmt.Errorf("creating synthetic job request %d returned %d: %s", index, suite.lastStatus, suite.lastBody)
			}
		}
		return nil
	})
}

func (suite *testSuite) queryInboxWithLimit(limit int) error {
	return suite.queryOperationsInboxWith(url.Values{"limit": {strconv.Itoa(limit)}})
}

func (suite *testSuite) inboxPageHasCountAndCursor(count int) error {
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	if len(page.Operations) != count || page.NextCursor == nil {
		return fmt.Errorf("expected %d operations and a next cursor, got %d (cursor present: %t)", count, len(page.Operations), page.NextCursor != nil)
	}
	return nil
}

func (suite *testSuite) queryInboxWithRawParameters(rawQuery string) error {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return fmt.Errorf("parsing parameters %q: %w", rawQuery, err)
	}
	return suite.queryOperationsInboxWith(query)
}
