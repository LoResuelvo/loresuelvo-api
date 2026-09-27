package steps_test

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type detailReportState struct {
	orderLabel        string
	description       string
	reportedOn        time.Time
	images            map[string]completionImageFixture
	reviewRating      int
	reviewDescription string
}

func registerAdminOperationDetailReportSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el reporte persistido de "([^"]*)" tiene descripción "([^"]*)" e imágenes privadas "([^"]*)" y "([^"]*)" creadas el "([^"]*)"$`, suite.detailOrderHasReport)
	sc.Step(`^que la review persistida de "([^"]*)" tiene calificación (\d+) y descripción "([^"]*)"$`, suite.detailOrderHasReview)
	sc.Step(`^consulto el detalle administrativo de la operación de la orden "([^"]*)"$`, suite.queryDetailByOrder)
	sc.Step(`^la orden "([^"]*)" informa el estado "([^"]*)", instante de finalización informada "([^"]*)" e instante de saldo pagado "([^"]*)"$`, suite.detailReportOrderFields)
	sc.Step(`^el reporte informa la descripción persistida, el instante "([^"]*)" y exactamente las imágenes "([^"]*)" y "([^"]*)"$`, suite.detailReportFields)
	sc.Step(`^la review informa calificación (\d+) y la descripción persistida, sin atribuirle un timestamp no persistido$`, suite.detailReviewFields)
	sc.Step(`^las imágenes se entregan como medios privados accesibles mediante autorización administrativa, incluso si el mecanismo usa URLs firmadas de corta duración$`, suite.detailReportImagesAreAdminAccessible)
	sc.Step(`^la respuesta no expone mensajes ni contenido de chat$`, suite.detailResponseHasNoChat)
}

func (suite *testSuite) detailOrderHasReport(orderLabel, description, first, second, instant string) error {
	orderID, ok := suite.operationInbox.orders[orderLabel]
	if !ok {
		return fmt.Errorf("unknown order %q", orderLabel)
	}
	reportedOn, err := parseInboxInstant(instant)
	if err != nil {
		return err
	}
	return suite.withInboxFixtureClock(func() error {
		order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
		if err != nil {
			return err
		}
		authID, err := suite.authIDForUserID(order.ProviderID())
		if err != nil {
			return err
		}
		suite.currentAuth0ID = authID
		if err := suite.setInboxFixtureClock(reportedOn); err != nil {
			return err
		}
		images := make(map[string]completionImageFixture, 2)
		fileIDs := make([]string, 0, 2)
		for _, alias := range []string{first, second} {
			if err := suite.uploadAndConfirmCompletionImage(alias); err != nil {
				return fmt.Errorf("uploading %s: %w", alias, err)
			}
			image := suite.completionImagesByName[alias]
			if image.FileID == "" {
				return fmt.Errorf("missing confirmed completion image %q", alias)
			}
			if err := (testsupport.OperationDetailMediaFixture{DB: suite.database}).SetFileCreatedOn(suite.scenarioContext, image.FileID, reportedOn); err != nil {
				return err
			}
			images[alias] = image
			fileIDs = append(fileIDs, image.FileID)
		}
		fixture := testsupport.OperationDetailReportFixture{DB: suite.database}
		if err := fixture.ReplaceCompletionReport(suite.scenarioContext, orderID, description, reportedOn, fileIDs); err != nil {
			return err
		}
		suite.detailReport = detailReportState{orderLabel: orderLabel, description: description, reportedOn: reportedOn, images: images}
		return nil
	})
}

func (suite *testSuite) detailOrderHasReview(orderLabel, ratingText, description string) error {
	if suite.detailReport.orderLabel != orderLabel {
		return fmt.Errorf("review order %q differs from report order %q", orderLabel, suite.detailReport.orderLabel)
	}
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return err
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.operationInbox.orders[orderLabel])
	if err != nil {
		return err
	}
	actor, err := suite.userRepository.FindByID(suite.scenarioContext, order.ConsumerID())
	if err != nil {
		return err
	}
	consumerActor, ok := actor.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("review actor has type %T", actor)
	}
	review, err := workorder.NewReview(rating, description)
	if err != nil {
		return err
	}
	if err := order.AddReview(consumerActor, review); err != nil {
		return err
	}
	if _, err := suite.workOrderRepository.Save(suite.scenarioContext, order); err != nil {
		return err
	}
	suite.detailReport.reviewRating, suite.detailReport.reviewDescription = rating, description
	return nil
}

func (suite *testSuite) queryDetailByOrder(label string) error {
	orderID, ok := suite.operationInbox.orders[label]
	if !ok {
		return fmt.Errorf("unknown order %q", label)
	}
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, orderID)
	if err != nil {
		return err
	}
	for _, proposal := range suite.operationInbox.proposals {
		if proposal.id == order.ServiceProposalID() {
			return suite.queryAdminJobRequestDetail(proposal.requestLabel)
		}
	}
	return fmt.Errorf("order %q has no labelled proposal", label)
}

func (suite *testSuite) detailReportOrderFields(label, status, reported, paid string) error {
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	order, err := detailMap(detail["work_order"], "work_order")
	if err != nil {
		return err
	}
	for field, want := range map[string]any{"id": float64(suite.operationInbox.orders[label]), "status": status, "completion_reported_on": reported, "balance_paid_on": paid} {
		if err := detailEqual(order, field, want); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) detailReportFields(instant, first, second string) error {
	if suite.detailReport.reportedOn.Format(time.RFC3339) != instant {
		return fmt.Errorf("report fixture instant differs from assertion")
	}
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	order, err := detailMap(detail["work_order"], "work_order")
	if err != nil {
		return err
	}
	report, err := detailMap(order["completion_report"], "completion_report")
	if err != nil {
		return err
	}
	for field, want := range map[string]any{"description": suite.detailReport.description, "reported_on": instant} {
		if err := detailEqual(report, field, want); err != nil {
			return err
		}
	}
	items, err := detailArray(report["images"], "completion_report.images")
	if err != nil {
		return err
	}
	if len(items) != 2 {
		return fmt.Errorf("expected exactly 2 completion images, got %d", len(items))
	}
	for position, alias := range []string{first, second} {
		image, ok := suite.detailReport.images[alias]
		if !ok {
			return fmt.Errorf("unknown image alias %q", alias)
		}
		resource, err := detailMap(items[position], "completion image")
		if err != nil {
			return err
		}
		for field, want := range map[string]any{"file_id": image.FileID, "original_name": alias, "mime_type": image.MimeType, "purpose": "work_order_completion_image", "created_on": instant} {
			if err := detailEqual(resource, field, want); err != nil {
				return fmt.Errorf("image %q: %w", alias, err)
			}
		}
		for key := range resource {
			if key == "url" || key == "storage_key" || key == "bucket" {
				return fmt.Errorf("completion image exposes private storage field %q", key)
			}
		}
	}
	return nil
}

func (suite *testSuite) detailReviewFields(ratingText string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return err
	}
	if rating != suite.detailReport.reviewRating {
		return fmt.Errorf("review rating assertion differs from fixture")
	}
	detail, err := suite.detailJSON()
	if err != nil {
		return err
	}
	order, err := detailMap(detail["work_order"], "work_order")
	if err != nil {
		return err
	}
	review, err := detailMap(order["review"], "review")
	if err != nil {
		return err
	}
	if len(review) != 2 {
		return fmt.Errorf("review exposes unexpected fields: %#v", review)
	}
	for field, want := range map[string]any{"rating": float64(rating), "description": suite.detailReport.reviewDescription} {
		if err := detailEqual(review, field, want); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) detailReportImagesAreAdminAccessible() error {
	order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, suite.operationInbox.orders[suite.detailReport.orderLabel])
	if err != nil {
		return err
	}
	var requestID int
	for _, proposal := range suite.operationInbox.proposals {
		if proposal.id == order.ServiceProposalID() {
			requestID = suite.operationInbox.requests[proposal.requestLabel].id
			break
		}
	}
	if requestID == 0 {
		return fmt.Errorf("completion order has no labelled request")
	}
	originalBody, originalStatus, originalHeaders := append([]byte(nil), suite.lastBody...), suite.lastStatus, suite.adminRequest.headers.Clone()
	defer func() {
		suite.lastBody, suite.lastStatus, suite.adminRequest.headers = originalBody, originalStatus, originalHeaders
	}()
	for _, alias := range []string{"completion-1", "completion-2"} {
		image, ok := suite.detailReport.images[alias]
		if !ok {
			return fmt.Errorf("missing completion image fixture %q", alias)
		}
		path := fmt.Sprintf("%s/jr-%d/images/%s", operationsInboxPath, requestID, url.PathEscape(image.FileID))
		if err := suite.sendAdminGet(path, nil, ""); err != nil {
			return err
		}
		if suite.lastStatus != 200 || suite.adminRequest.headers.Get("Content-Type") != image.MimeType || suite.adminRequest.headers.Get("Cache-Control") != "private, no-store" {
			return fmt.Errorf("private image %q returned status %d, type %q, cache %q", alias, suite.lastStatus, suite.adminRequest.headers.Get("Content-Type"), suite.adminRequest.headers.Get("Cache-Control"))
		}
		if !bytes.Equal(suite.lastBody, bytes.Repeat([]byte{0xff}, image.SizeBytes)) {
			return fmt.Errorf("private image %q content differs from confirmed upload", alias)
		}
	}
	if strings.Contains(string(originalBody), "\"url\"") {
		return fmt.Errorf("detail exposes a media URL instead of private file references")
	}
	return nil
}
