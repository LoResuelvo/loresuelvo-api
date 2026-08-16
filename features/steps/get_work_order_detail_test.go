package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

type workOrderDetailAcceptanceResponse struct {
	ID                int                       `json:"id"`
	ServiceProposalID int                       `json:"service_proposal_id"`
	ConsumerID        int                       `json:"consumer_id"`
	ProviderID        int                       `json:"provider_id"`
	AmountCents       int64                     `json:"amount_cents"`
	ScheduledOn       time.Time                 `json:"scheduled_on"`
	Description       string                    `json:"description"`
	Status            string                    `json:"status"`
	AcceptedOn        time.Time                 `json:"accepted_on"`
	PaidOn            *time.Time                `json:"paid_on"`
	CompletionReport  *completionReportResponse `json:"completion_report"`
}

func registerGetWorkOrderDetailSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una orden de trabajo en estado "([^"]*)" para "([^"]*)" y "([^"]*)"$`, suite.thereIsWorkOrderInCompletionState)
	sc.Step(`^que no existe una orden de trabajo con identificador "([^"]*)"$`, suite.thereIsNoWorkOrderWithID)
	sc.Step(`^que la orden tiene el reporte de finalización con la descripción:$`, suite.workOrderDetailReportDescriptionIs)
	sc.Step(`^que el reporte tiene las imágenes privadas "([^"]*)" y "([^"]*)" en ese orden$`, suite.workOrderDetailHasPrivateImages)
	sc.Step(`^que el reporte tiene la imagen privada "([^"]*)"$`, suite.workOrderDetailHasPrivateImage)
	sc.Step(`^consulto el detalle de la orden de trabajo$`, suite.requestWorkOrderDetail)
	sc.Step(`^consulto el detalle de la orden de trabajo inexistente$`, suite.requestMissingWorkOrderDetail)
	sc.Step(`^el sistema responde con estado (\d+)$`, suite.systemRespondsWithStatus)
	sc.Step(`^el detalle informa el estado "([^"]*)"$`, suite.workOrderDetailHasStatus)
	sc.Step(`^el detalle incluye el importe "([^"]*)"$`, suite.workOrderDetailHasAmount)
	sc.Step(`^el detalle incluye la fecha de servicio "([^"]*)"$`, suite.workOrderDetailHasScheduledOn)
	sc.Step(`^el detalle incluye la descripción:$`, suite.workOrderDetailHasDescription)
	sc.Step(`^el detalle no incluye un reporte de finalización$`, suite.workOrderDetailHasNoCompletionReport)
	sc.Step(`^el detalle incluye los datos contractuales de la propuesta aceptada$`, suite.workOrderDetailHasContractualData)
	sc.Step(`^el detalle incluye la descripción del reporte:$`, suite.workOrderDetailHasReportDescription)
	sc.Step(`^el detalle incluye la fecha del reporte$`, suite.workOrderDetailHasReportDate)
	sc.Step(`^el detalle incluye las imágenes "([^"]*)" y "([^"]*)" en ese orden$`, suite.workOrderDetailHasImagesInOrder)
	sc.Step(`^cada imagen incluye una URL temporal privada$`, suite.workOrderDetailImagesHavePrivateURLs)
	sc.Step(`^el detalle incluye la fecha del reporte y la fecha de pago$`, suite.workOrderDetailHasReportAndPaymentDates)
	sc.Step(`^el detalle incluye la imagen "([^"]*)" con una URL temporal privada$`, suite.workOrderDetailHasImageWithPrivateURL)
}

func (suite *testSuite) thereIsNoWorkOrderWithID(id string) error {
	workOrderID, err := strconv.Atoi(id)
	if err != nil || workOrderID <= 0 {
		return fmt.Errorf("parsing missing work order id %q", id)
	}
	suite.missingWorkOrderID = workOrderID
	return nil
}

func (suite *testSuite) thereIsWorkOrderInCompletionState(status, consumerEmail, providerEmail string) error {
	if status != string(workorder.StatusAwaitingPayment) && status != string(workorder.StatusPaid) {
		return fmt.Errorf("unsupported work order detail fixture status %q", status)
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

	suite.workOrderDetailTargetStatus = status
	suite.workOrderDetailCompletionDescription = ""
	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	return nil
}

func (suite *testSuite) workOrderDetailReportDescriptionIs(description *godog.DocString) error {
	suite.workOrderDetailCompletionDescription = normalizeDocString(description)
	if strings.TrimSpace(suite.workOrderDetailCompletionDescription) == "" {
		return fmt.Errorf("expected a non-empty completion report description fixture")
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasPrivateImages(first, second string) error {
	return suite.createWorkOrderDetailReport([]string{first, second})
}

func (suite *testSuite) workOrderDetailHasPrivateImage(name string) error {
	return suite.createWorkOrderDetailReport([]string{name})
}

func (suite *testSuite) createWorkOrderDetailReport(imageNames []string) error {
	if suite.workOrderDetailTargetStatus == "" {
		return fmt.Errorf("expected a work order detail completion state fixture")
	}
	if strings.TrimSpace(suite.workOrderDetailCompletionDescription) == "" {
		return fmt.Errorf("expected a completion report description before preparing images")
	}

	for _, imageName := range imageNames {
		if err := suite.uploadAndConfirmCompletionImage(imageName); err != nil {
			return fmt.Errorf("preparing work order detail image %q: %w", imageName, err)
		}
	}

	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	reportedOn := order.ScheduledOn().UTC().Add(time.Minute)
	if err := suite.requestTestClockMock(reportedOn.Format(time.RFC3339)); err != nil {
		return err
	}
	if err := suite.reportCompletion(suite.workOrderDetailCompletionDescription, imageNames); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating work order detail completion report returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}

	if suite.workOrderDetailTargetStatus == string(workorder.StatusPaid) {
		order, err = suite.persistedWorkOrderForLastServiceProposal()
		if err != nil {
			return err
		}
		if err := order.RegisterApprovedBalancePayment(suite.clock.Now().UTC().Add(time.Minute)); err != nil {
			return fmt.Errorf("registering paid work order detail fixture: %w", err)
		}
		if _, err := suite.workOrderRepository.Save(context.Background(), order); err != nil {
			return fmt.Errorf("saving paid work order detail fixture: %w", err)
		}
	}
	return nil
}

func (suite *testSuite) requestWorkOrderDetail() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	return suite.requestWorkOrderDetailByID(order.ID())
}

func (suite *testSuite) requestMissingWorkOrderDetail() error {
	if suite.missingWorkOrderID <= 0 {
		return fmt.Errorf("expected a missing work order id fixture")
	}
	return suite.requestWorkOrderDetailByID(suite.missingWorkOrderID)
}

func (suite *testSuite) systemRespondsWithStatus(status string) error {
	expectedStatus, err := strconv.Atoi(status)
	if err != nil || expectedStatus <= 0 {
		return fmt.Errorf("parsing expected HTTP status %q", status)
	}
	return suite.lastResponseShouldHaveStatusCode(expectedStatus)
}

func (suite *testSuite) requestWorkOrderDetailByID(workOrderID int) error {
	request, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/work-orders/%d", suite.server.URL, workOrderID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("creating work order detail request: %w", err)
	}
	if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting work order detail: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading work order detail response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) workOrderDetailHasStatus(expected string) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.Status != expected {
		return fmt.Errorf("expected work order detail status %q, got %q", expected, detail.Status)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasAmount(amount string) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	expected, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}
	if detail.AmountCents != expected {
		return fmt.Errorf("expected work order detail amount_cents %d, got %d", expected, detail.AmountCents)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasScheduledOn(scheduledOn string) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	expected, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing expected work order detail scheduled_on: %w", err)
	}
	if !detail.ScheduledOn.Equal(expected.UTC()) {
		return fmt.Errorf("expected work order detail scheduled_on %s, got %s", expected.UTC(), detail.ScheduledOn)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasDescription(description *godog.DocString) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	expected := normalizeDocString(description)
	if detail.Description != expected {
		return fmt.Errorf("expected work order detail description %q, got %q", expected, detail.Description)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasNoCompletionReport() error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport != nil {
		return fmt.Errorf("expected work order detail without completion report, got body %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasContractualData() error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.ID <= 0 || detail.ServiceProposalID <= 0 || detail.ConsumerID <= 0 || detail.ProviderID <= 0 {
		return fmt.Errorf("expected work order detail contractual identifiers, got body %s", suite.lastBody)
	}
	if detail.AmountCents <= 0 || detail.ScheduledOn.IsZero() || detail.AcceptedOn.IsZero() || strings.TrimSpace(detail.Description) == "" {
		return fmt.Errorf("expected complete work order detail contractual data, got body %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasReportDescription(description *godog.DocString) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil {
		return fmt.Errorf("expected work order detail completion report, got body %s", suite.lastBody)
	}
	expected := normalizeDocString(description)
	if detail.CompletionReport.Description != expected {
		return fmt.Errorf("expected completion report description %q, got %q", expected, detail.CompletionReport.Description)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasReportDate() error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil || detail.CompletionReport.ReportedOn.IsZero() {
		return fmt.Errorf("expected a non-zero completion report reported_on, got body %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasImagesInOrder(first, second string) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil {
		return fmt.Errorf("expected completion report images, got body %s", suite.lastBody)
	}
	expectedNames := []string{first, second}
	if len(detail.CompletionReport.Images) != len(expectedNames) {
		return fmt.Errorf("expected %d completion report images, got %d", len(expectedNames), len(detail.CompletionReport.Images))
	}
	for index, expectedName := range expectedNames {
		if detail.CompletionReport.Images[index].OriginalName != expectedName {
			return fmt.Errorf("expected completion report image %d to be %q, got %q", index+1, expectedName, detail.CompletionReport.Images[index].OriginalName)
		}
	}
	return nil
}

func (suite *testSuite) workOrderDetailImagesHavePrivateURLs() error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil || len(detail.CompletionReport.Images) == 0 {
		return fmt.Errorf("expected completion report images with private URLs, got body %s", suite.lastBody)
	}
	for _, image := range detail.CompletionReport.Images {
		if !isTemporaryPrivateURL(image.URL) {
			return fmt.Errorf("expected image %q to include a temporary private URL, got %q", image.OriginalName, image.URL)
		}
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasReportAndPaymentDates() error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil || detail.CompletionReport.ReportedOn.IsZero() {
		return fmt.Errorf("expected paid work order detail report date, got body %s", suite.lastBody)
	}
	if detail.PaidOn == nil || detail.PaidOn.IsZero() {
		return fmt.Errorf("expected paid work order detail paid_on, got body %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) workOrderDetailHasImageWithPrivateURL(name string) error {
	detail, err := suite.workOrderDetailResponse()
	if err != nil {
		return err
	}
	if detail.CompletionReport == nil {
		return fmt.Errorf("expected completion report image, got body %s", suite.lastBody)
	}
	for _, image := range detail.CompletionReport.Images {
		if image.OriginalName == name {
			if !isTemporaryPrivateURL(image.URL) {
				return fmt.Errorf("expected image %q to include a temporary private URL, got %q", name, image.URL)
			}
			return nil
		}
	}
	return fmt.Errorf("expected completion report image %q, got body %s", name, suite.lastBody)
}

func (suite *testSuite) workOrderDetailResponse() (workOrderDetailAcceptanceResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return workOrderDetailAcceptanceResponse{}, err
	}
	var detail workOrderDetailAcceptanceResponse
	if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
		return workOrderDetailAcceptanceResponse{}, fmt.Errorf("response is not a valid work order detail: %w", err)
	}
	return detail, nil
}

func isTemporaryPrivateURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "http")
}
