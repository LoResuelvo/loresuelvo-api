package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const (
	completionImagePurpose       = "work_order_completion_image"
	completionReportPath         = "/work-orders/%d/completion-reports"
	validCompletionImageSize     = 1024 * 1024
	oversizedCompletionImageSize = 6 * 1024 * 1024
)

type completionImageFixture struct {
	FileID       string
	OriginalName string
	MimeType     string
	SizeBytes    int
}

type completionReportRequest struct {
	Description  string   `json:"description"`
	ImageFileIDs []string `json:"image_file_ids"`
}

type completionReportResponse struct {
	ID          int                       `json:"id"`
	Description string                    `json:"description"`
	ReportedOn  time.Time                 `json:"reported_on"`
	Images      []completionImageResponse `json:"images"`
}

type completionImageResponse struct {
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

func registerReportWorkCompletionSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que cargué y confirmé una imagen privada de finalización "([^"]*)" para la orden$`, suite.uploadAndConfirmCompletionImage)
	sc.Step(`^que cargué y confirmé tres imágenes privadas de finalización: "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.uploadAndConfirmThreeCompletionImages)
	sc.Step(`^que preparé (cero|cuatro) imágenes para el reporte de finalización$`, suite.prepareCompletionImageCount)
	sc.Step(`^que preparé una imagen "([^"]*)" (perteneciente a otro prestador|pendiente de confirmación|con propósito incorrecto|con formato no permitido|que supera los 5 MB) para el reporte de finalización$`, suite.prepareUnavailableCompletionImage)
	sc.Step(`^que la orden ya tiene un reporte de finalización válido$`, suite.thereIsValidCompletionReport)
	sc.Step(`^que el prestador "([^"]*)" informó la finalización con evidencia válida de la orden$`, suite.providerReportedValidCompletion)

	sc.Step(`^informo la finalización de la orden con la imagen "([^"]*)" y la descripción:$`, suite.reportCompletionWithImageAndDescription)
	sc.Step(`^informo la finalización de la orden con las imágenes "([^"]*)", "([^"]*)" y "([^"]*)" y la descripción:$`, suite.reportCompletionWithThreeImagesAndDescription)
	sc.Step(`^intento informar la finalización de la orden con la imagen "([^"]*)" y la descripción:$`, suite.tryReportCompletionWithImageAndDescription)
	sc.Step(`^intento informar nuevamente la finalización de la orden con la imagen "([^"]*)" y la descripción:$`, suite.tryReportCompletionWithImageAndDescription)
	sc.Step(`^intento informar la finalización de la orden con la descripción "([^"]*)"$`, suite.tryReportCompletionWithPreparedImages)
	sc.Step(`^informo la finalización de la orden con la descripción "([^"]*)" y la imagen "([^"]*)"$`, suite.reportCompletionWithDescriptionAndImage)

	sc.Step(`^el sistema registra el reporte de finalización$`, suite.systemRegistersCompletionReport)
	sc.Step(`^la orden de trabajo queda en estado "awaiting_payment"$`, suite.workOrderAwaitsPayment)
	sc.Step(`^el sistema registra el reporte de finalización con las tres imágenes en ese orden$`, suite.systemRegistersCompletionReportWithOrderedImages)
	sc.Step(`^el sistema rechaza el reporte de finalización con estado (\d+)$`, suite.systemRejectsCompletionReportWithStatus)
	sc.Step(`^el sistema registra la notificación de finalización para el consumidor "([^"]*)"$`, suite.systemRegistersCompletionNotification)
	sc.Step(`^el consumidor "([^"]*)" recibe en tiempo real la notificación de finalización$`, suite.consumerReceivesCompletionNotification)
	sc.Step(`^la orden conserva la evidencia de finalización$`, suite.workOrderKeepsCompletionEvidence)
}

func (suite *testSuite) uploadAndConfirmCompletionImage(name string) error {
	return suite.prepareCompletionImage(suite.currentAuth0ID, name, completionImagePurpose, completionImageMIMEType(name), validCompletionImageSize, true, false)
}

func (suite *testSuite) uploadAndConfirmThreeCompletionImages(first, second, third string) error {
	for _, name := range []string{first, second, third} {
		if err := suite.uploadAndConfirmCompletionImage(name); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) prepareCompletionImageCount(quantity string) error {
	suite.completionImageNames = nil
	if quantity == "cero" {
		return nil
	}

	for index, name := range []string{"uno.jpg", "dos.jpg", "tres.jpg", "cuatro.jpg"} {
		if err := suite.uploadAndConfirmCompletionImage(name); err != nil {
			return fmt.Errorf("preparing completion image %d: %w", index+1, err)
		}
	}
	return nil
}

func (suite *testSuite) prepareUnavailableCompletionImage(name, condition string) error {
	switch condition {
	case "perteneciente a otro prestador":
		return suite.prepareCompletionImage(
			auth0IDForProviderEmail("pedro.plomero@example.com"),
			name,
			completionImagePurpose,
			completionImageMIMEType(name),
			validCompletionImageSize,
			true,
			false,
		)
	case "pendiente de confirmación":
		return suite.prepareCompletionImage(suite.currentAuth0ID, name, completionImagePurpose, completionImageMIMEType(name), validCompletionImageSize, false, false)
	case "con propósito incorrecto":
		return suite.prepareCompletionImage(suite.currentAuth0ID, name, "conversation_message_image", completionImageMIMEType(name), validCompletionImageSize, true, false)
	case "con formato no permitido":
		return suite.prepareCompletionImage(suite.currentAuth0ID, name, completionImagePurpose, "image/gif", validCompletionImageSize, false, true)
	case "que supera los 5 MB":
		return suite.prepareCompletionImage(suite.currentAuth0ID, name, completionImagePurpose, "image/jpeg", oversizedCompletionImageSize, false, true)
	default:
		return fmt.Errorf("unsupported completion image condition %q", condition)
	}
}

func (suite *testSuite) prepareCompletionImage(authID, name, purpose, mimeType string, sizeBytes int, confirm, toleratePresignFailure bool) error {
	if strings.TrimSpace(authID) == "" {
		return fmt.Errorf("expected an authenticated uploader before preparing completion image %q", name)
	}

	upload, err := suite.requestProfilePhotoPresign(authID, presignFileRequest{
		OriginalName: name,
		MimeType:     mimeType,
		SizeBytes:    sizeBytes,
		Purpose:      purpose,
	})
	if err != nil {
		return err
	}

	fixture := completionImageFixture{FileID: upload.FileID, OriginalName: name, MimeType: mimeType, SizeBytes: sizeBytes}
	if suite.completionImagesByName == nil {
		suite.completionImagesByName = map[string]completionImageFixture{}
	}
	suite.completionImagesByName[name] = fixture
	suite.completionImageNames = append(suite.completionImageNames, name)

	if suite.lastStatus != http.StatusOK {
		if toleratePresignFailure {
			return nil
		}
		return fmt.Errorf("preparing completion image %q: expected presign status 200, got %d with body %s", name, suite.lastStatus, suite.lastBody)
	}
	if upload.FileID == "" || upload.Key == "" || upload.UploadURL == "" {
		return fmt.Errorf("expected presign response for completion image %q to include file_id, key and upload_url", name)
	}

	if !confirm {
		return nil
	}
	if err := suite.putProfilePhotoObject(*upload, mimeType, sizeBytes); err != nil {
		return err
	}
	if err := suite.confirmFileUpload(authID, *upload, messageImageFixture{
		FileID:       upload.FileID,
		OriginalName: name,
		MimeType:     mimeType,
		SizeBytes:    sizeBytes,
	}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("confirming completion image %q returned status %d with body %s", name, suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) thereIsValidCompletionReport() error {
	if err := suite.requestTestClockMock("2026-08-15T16:00:00Z"); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForProviderEmail("juan.plomero@example.com")
	if err := suite.uploadAndConfirmCompletionImage("trabajo-inicial.jpg"); err != nil {
		return err
	}
	if err := suite.reportCompletion("Trabajo finalizado y funcionamiento verificado.", []string{"trabajo-inicial.jpg"}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating initial completion report returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) providerReportedValidCompletion(providerEmail string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	reportedOn := suite.clock.Now().UTC().Add(time.Minute)
	if reportedOn.IsZero() {
		return fmt.Errorf("expected a non-zero test clock before preparing completion report")
	}
	if err := suite.requestTestClockMock(reportedOn.Format(time.RFC3339)); err != nil {
		return err
	}
	if err := suite.uploadAndConfirmCompletionImage("trabajo-inicial.jpg"); err != nil {
		return err
	}
	if err := suite.reportCompletion("Trabajo finalizado y funcionamiento verificado.", []string{"trabajo-inicial.jpg"}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating completion report for payment fixture returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) reportCompletionWithImageAndDescription(name string, description *godog.DocString) error {
	return suite.reportCompletion(normalizeDocString(description), []string{name})
}

func (suite *testSuite) reportCompletionWithThreeImagesAndDescription(first, second, third string, description *godog.DocString) error {
	return suite.reportCompletion(normalizeDocString(description), []string{first, second, third})
}

func (suite *testSuite) tryReportCompletionWithImageAndDescription(name string, description *godog.DocString) error {
	return suite.reportCompletion(normalizeDocString(description), []string{name})
}

func (suite *testSuite) tryReportCompletionWithPreparedImages(description string) error {
	return suite.reportCompletion(description, suite.completionImageNames)
}

func (suite *testSuite) reportCompletionWithDescriptionAndImage(description, name string) error {
	return suite.reportCompletion(description, []string{name})
}

func (suite *testSuite) reportCompletion(description string, imageNames []string) error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	fileIDs := make([]string, 0, len(imageNames))
	for _, name := range imageNames {
		fixture, ok := suite.completionImagesByName[name]
		if !ok {
			return fmt.Errorf("expected completion image fixture %q", name)
		}
		fileIDs = append(fileIDs, fixture.FileID)
	}

	resp, err := suite.postJSONWithAuth(suite.currentAuth0ID, fmt.Sprintf(completionReportPath, order.ID()), completionReportRequest{
		Description:  description,
		ImageFileIDs: fileIDs,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading completion report response: %w", err)
	}
	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	suite.lastLocation = resp.Header.Get("Location")
	suite.lastCompletionReportWorkOrderID = order.ID()
	suite.lastCompletionReport = completionReportResponse{}
	if resp.StatusCode == http.StatusCreated {
		if err := json.Unmarshal(body, &suite.lastCompletionReport); err != nil {
			return fmt.Errorf("decoding completion report response: %w", err)
		}
	}
	return nil
}

func (suite *testSuite) systemRegistersCompletionReport() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	if suite.lastCompletionReport.ID == 0 {
		return fmt.Errorf("expected completion report id, got body %s", suite.lastBody)
	}
	if strings.TrimSpace(suite.lastCompletionReport.Description) == "" {
		return fmt.Errorf("expected completion report description, got body %s", suite.lastBody)
	}
	if suite.lastCompletionReport.ReportedOn.IsZero() {
		return fmt.Errorf("expected completion report reported_on, got body %s", suite.lastBody)
	}
	if len(suite.lastCompletionReport.Images) < 1 {
		return fmt.Errorf("expected completion report images, got body %s", suite.lastBody)
	}
	if suite.lastLocation != fmt.Sprintf("/work-orders/%d", suite.lastCompletionReportWorkOrderID) {
		return fmt.Errorf("expected completion report Location %q, got %q", fmt.Sprintf("/work-orders/%d", suite.lastCompletionReportWorkOrderID), suite.lastLocation)
	}
	return nil
}

func (suite *testSuite) workOrderAwaitsPayment() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.Status() != workorder.StatusAwaitingPayment {
		return fmt.Errorf("expected work order status %q, got %q", workorder.StatusAwaitingPayment, order.Status())
	}
	return nil
}

func (suite *testSuite) systemRegistersCompletionReportWithOrderedImages() error {
	if err := suite.systemRegistersCompletionReport(); err != nil {
		return err
	}
	expectedNames := []string{"antes.jpg", "durante.png", "después.webp"}
	if len(suite.lastCompletionReport.Images) != len(expectedNames) {
		return fmt.Errorf("expected %d completion images, got %d", len(expectedNames), len(suite.lastCompletionReport.Images))
	}
	for index, expectedName := range expectedNames {
		image := suite.lastCompletionReport.Images[index]
		if image.OriginalName != expectedName {
			return fmt.Errorf("expected completion image %d to be %q, got %q", index+1, expectedName, image.OriginalName)
		}
		if image.FileID == "" || image.URL == "" {
			return fmt.Errorf("expected completion image %q to include file_id and url", expectedName)
		}
	}
	return nil
}

func (suite *testSuite) systemRejectsCompletionReportWithStatus(statusCode string) error {
	var expected int
	if _, err := fmt.Sscanf(statusCode, "%d", &expected); err != nil {
		return fmt.Errorf("parsing expected completion report status: %w", err)
	}
	if err := suite.lastResponseShouldHaveStatusCode(expected); err != nil {
		return err
	}
	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemRegistersCompletionNotification(email string) error {
	userID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding completion notification consumer: %w", err)
	}
	found, err := suite.notificationRepository.FindLatestByUserAndResource(
		context.Background(),
		userID,
		notification.TypeWorkOrderCompletionReported,
		notification.ResourceWorkOrder,
		suite.lastCompletionReportWorkOrderID,
	)
	if err != nil {
		return fmt.Errorf("finding persisted completion notification: %w", err)
	}
	if found.ID == 0 || found.UserID != userID || found.Type != notification.TypeWorkOrderCompletionReported || found.ResourceType != notification.ResourceWorkOrder || found.ResourceID != suite.lastCompletionReportWorkOrderID {
		return fmt.Errorf("unexpected persisted completion notification: %+v", found)
	}
	return nil
}

func (suite *testSuite) workOrderKeepsCompletionEvidence() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.CompletionReport() == nil {
		return fmt.Errorf("expected work order to keep its completion report")
	}
	if strings.TrimSpace(order.CompletionReport().Description()) == "" {
		return fmt.Errorf("expected work order completion report description to be present")
	}
	if len(order.CompletionReport().ImageFileIDs()) == 0 {
		return fmt.Errorf("expected work order completion report images to be present")
	}
	return nil
}

func (suite *testSuite) consumerReceivesCompletionNotification(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return err
	}
	if event.Type != "notification.created" ||
		event.Notification.ID == 0 ||
		event.Notification.UserID != consumerID ||
		event.Notification.Type != string(notification.TypeWorkOrderCompletionReported) ||
		event.Notification.ResourceType != string(notification.ResourceWorkOrder) ||
		event.Notification.ResourceID != suite.lastCompletionReportWorkOrderID {
		return fmt.Errorf("unexpected realtime completion notification: %+v", event.Notification)
	}
	return nil
}

func completionImageMIMEType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
