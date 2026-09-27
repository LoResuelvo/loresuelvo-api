package steps_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type detailRequestEvidenceState struct {
	conversations         map[string]int
	conversationConsumers map[string]int
	messages              map[string]int
	messageContent        map[string]string
	assessments           map[string]int
	assessmentRows        map[string]map[string]string
	requestRows           map[string]map[string]string
	images                map[string]detailRequestImageFixture
}

type detailRequestImageFixture struct {
	id  string
	row map[string]string
}

func (state *detailRequestEvidenceState) initialize() {
	if state.conversations == nil {
		state.conversations = map[string]int{}
		state.conversationConsumers = map[string]int{}
		state.messages = map[string]int{}
		state.messageContent = map[string]string{}
		state.assessments = map[string]int{}
		state.assessmentRows = map[string]map[string]string{}
		state.requestRows = map[string]map[string]string{}
		state.images = map[string]detailRequestImageFixture{}
	}
}

func registerAdminOperationDetailRequestEvidenceSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el consumidor "([^"]*)" tiene la dirección actual "([^"]*)", número "([^"]*)", piso "([^"]*)", unidad "([^"]*)"$`, suite.detailConsumerHasAddress)
	sc.Step(`^que existe la conversación "([^"]*)" del consumidor "([^"]*)" con los siguientes mensajes persistidos:$`, suite.detailConversationHasMessages)
	sc.Step(`^que existen las siguientes evaluaciones persistidas de conversación:$`, suite.detailConversationHasAssessments)
	sc.Step(`^que "([^"]*)" tiene las siguientes imágenes persistidas:$`, suite.detailRequestHasImages)
	sc.Step(`^consulto el detalle administrativo de la operación de la solicitud "([^"]*)"$`, suite.queryAdminJobRequestDetail)
	sc.Step(`^el detalle informa el identificador "jr-" seguido del ID persistido de "([^"]*)"$`, suite.detailRequestHasStableID)
	sc.Step(`^la sección de solicitud informa el ID de "([^"]*)", estado "([^"]*)", título, descripción y el instante de creación "([^"]*)"$`, suite.detailRequestHasPersistedFields)
	sc.Step(`^el detalle informa la evaluación persistida "([^"]*)" versión (\d+), basada en el mensaje "([^"]*)", con su resultado, rubro, título, descripción y el instante "([^"]*)"$`, suite.detailRequestHasAssessment)
	sc.Step(`^la evaluación expuesta coincide con el origen guardado en "([^"]*)" aunque exista una evaluación más reciente en la conversación$`, suite.detailRequestUsesSourceAssessment)
	sc.Step(`^el contacto informa al consumidor "([^"]*)" y al prestador "([^"]*)" con sus IDs internos$`, suite.detailRequestHasParties)
	sc.Step(`^la dirección informa los campos guardados "([^"]*)", "([^"]*)", "([^"]*)" y "([^"]*)" con procedencia de la dirección actual del consumidor$`, suite.detailRequestHasAddress)
	sc.Step(`^la imagen de la solicitud informa el archivo "([^"]*)", nombre "([^"]*)" y propósito "([^"]*)"$`, suite.detailRequestHasImage)
	sc.Step(`^el detalle no expone mensajes ni contenido de la conversación$`, suite.detailRequestHasNoChat)
}

func (suite *testSuite) detailFixture() testsupport.OperationDetailRequestFixture {
	return testsupport.OperationDetailRequestFixture{DB: suite.database}
}

func (suite *testSuite) detailConsumerHasAddress(email, street, number, floor, unit string) error {
	id, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return err
	}
	return suite.detailFixture().SetConsumerAddress(suite.scenarioContext, id, street, number, floor, unit)
}

// thereIsDetailRequestWithAssessment is called by the shared request fixture
// dispatcher only when the table explicitly names an assessment source.
func (suite *testSuite) thereIsDetailRequestWithAssessment(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one assessed request, got %d", len(rows))
	}
	row := rows[0]
	state := &suite.detailRequestEvidence
	state.initialize()
	assessmentID, ok := state.assessments[row["evaluación origen"]]
	if !ok {
		return fmt.Errorf("unknown source assessment %q", row["evaluación origen"])
	}
	if _, ok := state.conversations[row["conversación"]]; !ok {
		return fmt.Errorf("unknown source conversation %q", row["conversación"])
	}
	created, err := parseInboxInstant(row["creada"])
	if err != nil {
		return err
	}
	if row["estado"] != "pending" {
		return fmt.Errorf("unsupported assessed request status %q", row["estado"])
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		providerID, err := suite.providerIDByEmail(row["prestador"])
		if err != nil {
			return err
		}
		if err := suite.setInboxFixtureClock(created); err != nil {
			return err
		}
		suite.currentAuth0ID = auth0IDForConsumerEmail(row["consumidor"])
		if err := suite.requestJobRequest(jobRequestCreationRequest{ProviderID: providerID, Title: row["título"], Description: row["descripción"]}); err != nil {
			return err
		}
		if suite.lastStatus != 201 {
			return fmt.Errorf("creating assessed request returned %d: %s", suite.lastStatus, suite.lastBody)
		}
		result, err := suite.jobRequestCreationResponseFromLastBody()
		if err != nil {
			return err
		}
		if err := suite.detailFixture().SetRequestAssessment(suite.scenarioContext, result.ID, assessmentID); err != nil {
			return err
		}
		suite.operationInbox.requests[row["solicitud"]] = inboxJobRequestFixture{createdOn: created, id: result.ID, conversationID: result.ConversationID, consumerEmail: row["consumidor"], providerEmail: row["prestador"]}
		state.requestRows[row["solicitud"]] = row
		return nil
	})
}

func (suite *testSuite) detailConversationHasMessages(label, email string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("conversation has no messages")
	}
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return err
	}
	created, err := parseInboxInstant(rows[0]["enviada"])
	if err != nil {
		return err
	}
	state := &suite.detailRequestEvidence
	state.initialize()
	id, err := suite.detailFixture().CreateChatbotConversation(suite.scenarioContext, consumerID, created.Add(-time.Minute))
	if err != nil {
		return err
	}
	state.conversations[label] = id
	state.conversationConsumers[label] = consumerID
	for _, row := range rows {
		instant, err := parseInboxInstant(row["enviada"])
		if err != nil {
			return err
		}
		messageID, err := suite.detailFixture().AddMessage(suite.scenarioContext, id, row["contenido"], instant)
		if err != nil {
			return err
		}
		state.messages[row["mensaje"]] = messageID
		state.messageContent[row["mensaje"]] = row["contenido"]
	}
	return nil
}

func (suite *testSuite) detailConversationHasAssessments(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	state := &suite.detailRequestEvidence
	state.initialize()
	for _, row := range rows {
		conversationID, ok := state.conversations[row["conversación"]]
		if !ok {
			return fmt.Errorf("unknown assessment conversation %q", row["conversación"])
		}
		messageID, ok := state.messages[row["mensaje base"]]
		if !ok {
			return fmt.Errorf("unknown base message %q", row["mensaje base"])
		}
		consumerID, err := suite.userRepository.FindIDByEmail(row["consumidor"])
		if err != nil {
			return err
		}
		if state.conversationConsumers[row["conversación"]] != consumerID {
			return fmt.Errorf("assessment consumer does not own source conversation")
		}
		version, err := strconv.Atoi(row["versión"])
		if err != nil {
			return err
		}
		instant, err := parseInboxInstant(row["creada"])
		if err != nil {
			return err
		}
		var categoryID *int
		if row["rubro"] != "" {
			id, ok := suite.categoryIDsByName[row["rubro"]]
			if !ok {
				return fmt.Errorf("unknown category %q", row["rubro"])
			}
			categoryID = &id
		}
		id, err := suite.detailFixture().AddAssessment(suite.scenarioContext, conversationID, version, messageID, row["resultado"], categoryID, row["título"], row["descripción"], instant)
		if err != nil {
			return err
		}
		state.assessments[row["evaluación"]] = id
		state.assessmentRows[row["evaluación"]] = row
	}
	return nil
}

func (suite *testSuite) detailRequestHasImages(label string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	state := &suite.detailRequestEvidence
	state.initialize()
	for _, row := range rows {
		instant, err := parseInboxInstant(row["creado"])
		if err != nil {
			return err
		}
		if row["visibilidad"] != filedomain.VisibilityPrivate || row["propósito"] != filedomain.PurposeJobRequestImage {
			return fmt.Errorf("expected private request image")
		}
		metadata, err := filedomain.NewFileMetadata(row["nombre original"], row["mime_type"], 16)
		if err != nil {
			return err
		}
		id := uuid.NewString()
		file, err := filedomain.NewFile(id, "bdd/"+id, "private", metadata, filedomain.StatusConfirmed, row["visibilidad"], row["propósito"], auth0IDForConsumerEmail(request.consumerEmail), instant, instant)
		if err != nil {
			return err
		}
		if err := suite.fileRepository.Save(suite.scenarioContext, *file); err != nil {
			return err
		}
		if err := suite.detailFixture().AddRequestImage(suite.scenarioContext, request.id, id); err != nil {
			return err
		}
		state.images[row["archivo"]] = detailRequestImageFixture{id: id, row: row}
	}
	return nil
}

type detailEvidenceResponse struct {
	ID         string `json:"id"`
	JobRequest *struct {
		ID          int       `json:"id"`
		Status      string    `json:"status"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		CreatedOn   time.Time `json:"created_on"`
		Images      []struct {
			FileID       string    `json:"file_id"`
			OriginalName string    `json:"original_name"`
			MimeType     string    `json:"mime_type"`
			Purpose      string    `json:"purpose"`
			CreatedOn    time.Time `json:"created_on"`
		} `json:"images"`
	} `json:"job_request"`
	SourceAssessment *struct {
		ID       int    `json:"id"`
		Version  int    `json:"version"`
		Outcome  string `json:"outcome"`
		Category *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"category"`
		Title            string    `json:"title"`
		Description      string    `json:"description"`
		BasedOnMessageID int       `json:"based_on_message_id"`
		CreatedOn        time.Time `json:"created_on"`
	} `json:"source_assessment"`
	Consumer struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Surname string `json:"surname"`
	} `json:"consumer"`
	Provider struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Surname string `json:"surname"`
	} `json:"provider"`
	Address *struct {
		Street       string  `json:"street"`
		StreetNumber string  `json:"street_number"`
		Floor        *string `json:"floor"`
		Unit         *string `json:"unit"`
		Source       string  `json:"source"`
	} `json:"address"`
}

func (suite *testSuite) detailEvidenceBody() (detailEvidenceResponse, error) {
	var body detailEvidenceResponse
	err := json.Unmarshal(suite.lastBody, &body)
	return body, err
}

func (suite *testSuite) detailRequestHasStableID(label string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	request := suite.operationInbox.requests[label]
	expected := fmt.Sprintf("jr-%d", request.id)
	if body.ID != expected {
		return fmt.Errorf("expected detail id %s, got %s", expected, body.ID)
	}
	return nil
}

func (suite *testSuite) detailRequestHasPersistedFields(label, status, createdText string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	row := suite.detailRequestEvidence.requestRows[label]
	request := suite.operationInbox.requests[label]
	if body.JobRequest == nil {
		return fmt.Errorf("missing job request")
	}
	created, err := parseInboxInstant(createdText)
	if err != nil {
		return err
	}
	if body.JobRequest.ID != request.id || body.JobRequest.Status != status || body.JobRequest.Title != row["título"] || body.JobRequest.Description != row["descripción"] || !body.JobRequest.CreatedOn.Equal(created) {
		return fmt.Errorf("job request does not match persisted fixture: %+v", body.JobRequest)
	}
	return nil
}

func (suite *testSuite) detailRequestHasAssessment(label string, version int, messageLabel, createdText string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	a := body.SourceAssessment
	if a == nil {
		return fmt.Errorf("missing source assessment")
	}
	row := suite.detailRequestEvidence.assessmentRows[label]
	instant, err := parseInboxInstant(createdText)
	if err != nil {
		return err
	}
	if a.ID != suite.detailRequestEvidence.assessments[label] || a.Version != version || a.BasedOnMessageID != suite.detailRequestEvidence.messages[messageLabel] || a.Outcome != row["resultado"] || a.Title != row["título"] || a.Description != row["descripción"] || !a.CreatedOn.Equal(instant) {
		return fmt.Errorf("source assessment differs from persisted fixture: %+v", a)
	}
	if row["rubro"] == "" {
		if a.Category != nil {
			return fmt.Errorf("unexpected category: %+v", a.Category)
		}
	} else if a.Category == nil || a.Category.ID != suite.categoryIDsByName[row["rubro"]] || a.Category.Name != row["rubro"] {
		return fmt.Errorf("wrong source assessment category: %+v", a.Category)
	}
	return nil
}

func (suite *testSuite) detailRequestUsesSourceAssessment(label string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	row := suite.detailRequestEvidence.requestRows[label]
	sourceID := suite.detailRequestEvidence.assessments[row["evaluación origen"]]
	if body.SourceAssessment == nil || body.SourceAssessment.ID != sourceID {
		return fmt.Errorf("detail did not use request source assessment %d", sourceID)
	}
	for assessmentLabel, id := range suite.detailRequestEvidence.assessments {
		if id != sourceID && body.SourceAssessment.ID == id {
			return fmt.Errorf("detail used newer assessment %s", assessmentLabel)
		}
	}
	return nil
}

func (suite *testSuite) detailRequestHasParties(consumerName, providerName string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	row := suite.detailRequestEvidence.requestRows["S1"]
	consumerID, err := suite.userRepository.FindIDByEmail(row["consumidor"])
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(row["prestador"])
	if err != nil {
		return err
	}
	if body.Consumer.ID != consumerID || body.Provider.ID != providerID || strings.TrimSpace(body.Consumer.Name+" "+body.Consumer.Surname) != consumerName || strings.TrimSpace(body.Provider.Name+" "+body.Provider.Surname) != providerName {
		return fmt.Errorf("party evidence mismatch: consumer=%+v provider=%+v", body.Consumer, body.Provider)
	}
	return nil
}

func (suite *testSuite) detailRequestHasAddress(street, number, floor, unit string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	a := body.Address
	if a == nil || a.Floor == nil || a.Unit == nil || a.Street != street || a.StreetNumber != number || *a.Floor != floor || *a.Unit != unit || a.Source != "current_consumer_address" {
		return fmt.Errorf("address evidence mismatch: %+v", a)
	}
	return nil
}

func (suite *testSuite) detailRequestHasImage(label, name, purpose string) error {
	body, err := suite.detailEvidenceBody()
	if err != nil {
		return err
	}
	fixture, ok := suite.detailRequestEvidence.images[label]
	if !ok {
		return fmt.Errorf("unknown image %q", label)
	}
	if body.JobRequest == nil {
		return fmt.Errorf("missing job request")
	}
	for _, image := range body.JobRequest.Images {
		instant, err := parseInboxInstant(fixture.row["creado"])
		if err != nil {
			return err
		}
		if image.FileID == fixture.id && image.OriginalName == name && image.MimeType == fixture.row["mime_type"] && image.Purpose == purpose && image.CreatedOn.Equal(instant) {
			return nil
		}
	}
	return fmt.Errorf("request image %q metadata absent: %s", label, suite.lastBody)
}

func (suite *testSuite) detailRequestHasNoChat() error {
	var raw any
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return err
	}
	forbidden := map[string]bool{"messages": true, "conversation": true, "content": true, "chat": true}
	var visit func(any) error
	visit = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[key] {
					return fmt.Errorf("detail exposes forbidden field %q", key)
				}
				if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := visit(child); err != nil {
					return err
				}
			}
		case string:
			for _, content := range suite.detailRequestEvidence.messageContent {
				if strings.Contains(typed, content) {
					return fmt.Errorf("detail exposes conversation content")
				}
			}
		}
		return nil
	}
	return visit(raw)
}
