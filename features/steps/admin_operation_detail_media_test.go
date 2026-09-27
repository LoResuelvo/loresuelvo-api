package steps_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type detailMediaState struct {
	images map[string]detailMediaImage
}

type detailMediaImage struct {
	fileID    string
	requestID int
	mimeType  string
	sizeBytes int
}

func registerAdminOperationDetailMediaSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" tiene la siguiente imagen privada confirmada:$`, suite.detailRequestHasConfirmedPrivateImage)
	sc.Step(`^solicito la imagen privada "([^"]*)" como administrador de operaciones$`, suite.requestAdminOperationImage)
	sc.Step(`^el sistema responde con estado 200 y el contenido corresponde al archivo "([^"]*)"$`, suite.adminOperationImageHasContent)
	sc.Step(`^la respuesta de imagen incluye "([^"]*)" con valor "([^"]*)"$`, suite.adminResponseHeaderEquals)
	sc.Step(`^la respuesta no expone acceso público anónimo a la imagen$`, suite.operationImageIsNotPublic)
	sc.Step(`^no se entrega ningún byte de la imagen$`, suite.adminOperationImageIsNotDelivered)
}

func (suite *testSuite) detailRequestHasConfirmedPrivateImage(label string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one private image, got %d", len(rows))
	}
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request %q", label)
	}
	row := rows[0]
	if row["propósito"] != file.PurposeJobRequestImage {
		return fmt.Errorf("unsupported image purpose %q", row["propósito"])
	}
	createdOn, err := parseInboxInstant(row["creada"])
	if err != nil {
		return err
	}
	name := row["nombre original"]
	if err := suite.uploadAndRememberImage(auth0IDForConsumerEmail(request.consumerEmail), name, row["propósito"], true); err != nil {
		return err
	}
	image, ok := suite.messageImagesByName[name]
	if !ok || image.FileID == "" || image.MimeType != row["mime_type"] {
		return fmt.Errorf("confirmed image %q does not match fixture", name)
	}
	if err := suite.detailFixture().AddRequestImage(suite.scenarioContext, request.id, image.FileID); err != nil {
		return err
	}
	fixture := testsupport.OperationDetailMediaFixture{DB: suite.database}
	if err := fixture.SetFileCreatedOn(suite.scenarioContext, image.FileID, createdOn); err != nil {
		return err
	}
	if suite.detailMedia.images == nil {
		suite.detailMedia.images = map[string]detailMediaImage{}
	}
	suite.detailMedia.images[row["archivo"]] = detailMediaImage{
		fileID: image.FileID, requestID: request.id,
		mimeType: image.MimeType, sizeBytes: image.SizeBytes,
	}
	return nil
}

func (suite *testSuite) operationImagePath(image detailMediaImage) string {
	return fmt.Sprintf("%s/jr-%d/images/%s", operationsInboxPath, image.requestID, url.PathEscape(image.fileID))
}

func (suite *testSuite) requestAdminOperationImage(label string) error {
	image, ok := suite.detailMedia.images[label]
	if !ok {
		return fmt.Errorf("unknown image label %q", label)
	}
	return suite.sendAdminGet(suite.operationImagePath(image), nil, "")
}

func (suite *testSuite) adminOperationImageHasContent(label string) error {
	image, ok := suite.detailMedia.images[label]
	if !ok {
		return fmt.Errorf("unknown image label %q", label)
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("expected image status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	if got := suite.adminRequest.headers.Get("Content-Type"); got != image.mimeType {
		return fmt.Errorf("expected image content type %q, got %q", image.mimeType, got)
	}
	if !bytes.Equal(suite.lastBody, bytes.Repeat([]byte{0xff}, image.sizeBytes)) {
		return fmt.Errorf("image bytes differ from the confirmed upload for %q", label)
	}
	return nil
}

func (suite *testSuite) operationImageIsNotPublic() error {
	if len(suite.detailMedia.images) != 1 {
		return fmt.Errorf("expected one image fixture, got %d", len(suite.detailMedia.images))
	}
	for _, image := range suite.detailMedia.images {
		request, err := http.NewRequest(http.MethodGet, suite.server.URL+suite.operationImagePath(image), nil)
		if err != nil {
			return err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusUnauthorized || bytes.Contains(body, bytes.Repeat([]byte{0xff}, 32)) {
			return fmt.Errorf("anonymous image request returned status %d or leaked image bytes", response.StatusCode)
		}
	}
	return nil
}

func (suite *testSuite) adminOperationImageIsNotDelivered() error {
	if suite.lastStatus != http.StatusUnauthorized && suite.lastStatus != http.StatusForbidden {
		return fmt.Errorf("expected denied image response, got %d", suite.lastStatus)
	}
	for _, image := range suite.detailMedia.images {
		if bytes.Contains(suite.lastBody, bytes.Repeat([]byte{0xff}, 32)) || len(suite.lastBody) >= image.sizeBytes {
			return fmt.Errorf("denied request leaked image bytes")
		}
	}
	return nil
}
