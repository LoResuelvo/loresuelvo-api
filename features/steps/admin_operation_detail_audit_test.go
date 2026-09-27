package steps_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/cucumber/godog"
)

var errInjectedOperationDetailAudit = errors.New("injected operation detail audit failure")

type operationDetailAuditCapture struct {
	writer   audit.Writer
	failSave bool
	attempts int
}

func (capture *operationDetailAuditCapture) decorate(writer audit.Writer) audit.Writer {
	capture.writer = writer
	return capture
}

func (capture *operationDetailAuditCapture) Save(ctx context.Context, event *audit.Event) error {
	capture.attempts++
	if capture.failSave {
		return errInjectedOperationDetailAudit
	}
	return capture.writer.Save(ctx, event)
}

func (capture *operationDetailAuditCapture) reset() {
	capture.failSave = false
	capture.attempts = 0
}

type operationDetailAuditSnapshot struct {
	request      *jobrequest.JobRequest
	conversation conversation.Conversation
	proposals    []*serviceproposal.ServiceProposal
	auditMark    int64
}

func registerAdminOperationDetailAuditSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe la siguiente solicitud de trabajo:$`, suite.thereIsAuditedDetailJobRequest)
	sc.Step(`^que falla el almacenamiento del evento de auditoría de esta consulta$`, suite.failOperationDetailAuditSave)
	sc.Step(`^consulto el detalle administrativo de la solicitud "([^"]*)"$`, suite.queryAuditedJobRequestDetail)
	sc.Step(`^no se entregan parcialmente los datos de "([^"]*)"$`, suite.auditFailureDoesNotLeakDetail)
	sc.Step(`^la solicitud "([^"]*)" y sus relaciones permanecen sin cambios$`, suite.auditFailureLeavesOperationUnchanged)
}

func (suite *testSuite) thereIsAuditedDetailJobRequest(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one job request, got %d", len(rows))
	}
	row := rows[0]
	createdOn, err := parseInboxInstant(row["creada"])
	if err != nil {
		return err
	}
	if row["estado"] != "pending" && row["estado"] != "accepted" {
		return fmt.Errorf("unsupported job request status %q", row["estado"])
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		providerID, err := suite.providerIDByEmail(row["prestador"])
		if err != nil {
			return err
		}
		if err := suite.setInboxFixtureClock(createdOn); err != nil {
			return err
		}
		suite.currentAuth0ID = auth0IDForConsumerEmail(row["consumidor"])
		if err := suite.requestJobRequest(jobRequestCreationRequest{ProviderID: providerID, Title: row["título"], Description: row["descripción"]}); err != nil {
			return err
		}
		if suite.lastStatus != http.StatusCreated {
			return fmt.Errorf("creating job request %q returned %d: %s", row["solicitud"], suite.lastStatus, suite.lastBody)
		}
		created, err := suite.jobRequestCreationResponseFromLastBody()
		if err != nil {
			return err
		}
		suite.operationInbox.requests[row["solicitud"]] = inboxJobRequestFixture{createdOn: createdOn, id: created.ID, conversationID: created.ConversationID, consumerEmail: row["consumidor"], providerEmail: row["prestador"]}
		if row["estado"] == "accepted" {
			return suite.acceptInboxJobRequest(row["solicitud"])
		}
		return nil
	})
}

func (suite *testSuite) failOperationDetailAuditSave() error {
	if len(suite.operationInbox.requests) != 1 {
		return fmt.Errorf("expected exactly one request fixture, got %d", len(suite.operationInbox.requests))
	}
	for label := range suite.operationInbox.requests {
		snapshot, err := suite.captureOperationDetailAuditSnapshot(label)
		if err != nil {
			return err
		}
		suite.operationDetailAuditSnapshot = snapshot
	}
	suite.operationDetailAuditCapture.failSave = true
	return nil
}

func (suite *testSuite) captureOperationDetailAuditSnapshot(label string) (*operationDetailAuditSnapshot, error) {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return nil, fmt.Errorf("unknown job request label %q", label)
	}
	request, err := suite.jobRequestRepository.FindByID(fixture.id)
	if err != nil {
		return nil, err
	}
	thread, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.ConversationID)
	if err != nil {
		return nil, err
	}
	proposals, err := suite.dependencies.Persistence.ServiceProposalRepository.FindByUserID(suite.scenarioContext, request.ConsumerID)
	if err != nil {
		return nil, err
	}
	mark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return nil, err
	}
	return &operationDetailAuditSnapshot{request: request, conversation: thread, proposals: proposals, auditMark: mark}, nil
}

func (suite *testSuite) queryAuditedJobRequestDetail(label string) error {
	return suite.queryAdminJobRequestDetail(label)
}

func (suite *testSuite) auditFailureDoesNotLeakDetail(label string) error {
	if err := suite.adminDetailErrorHasNoData(); err != nil {
		return err
	}
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	request, err := suite.jobRequestRepository.FindByID(fixture.id)
	if err != nil {
		return err
	}
	for _, text := range []string{request.Title, request.Description, fmt.Sprintf("jr-%d", fixture.id)} {
		if text != "" && containsJSONValue(suite.lastBody, text) {
			return fmt.Errorf("audit failure response exposes operation detail %q", text)
		}
	}
	return nil
}

func (suite *testSuite) auditFailureLeavesOperationUnchanged(label string) error {
	before := suite.operationDetailAuditSnapshot
	if before == nil {
		return fmt.Errorf("operation detail snapshot was not captured")
	}
	after, err := suite.captureOperationDetailAuditSnapshot(label)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.request, after.request) || !reflect.DeepEqual(before.conversation, after.conversation) || !reflect.DeepEqual(before.proposals, after.proposals) {
		return fmt.Errorf("operation changed after audit failure")
	}
	if after.auditMark != before.auditMark {
		return fmt.Errorf("audit event was persisted despite injected failure")
	}
	if suite.operationDetailAuditCapture.attempts != 1 {
		return fmt.Errorf("expected one failed audit save, got %d", suite.operationDetailAuditCapture.attempts)
	}
	return nil
}

func containsJSONValue(body []byte, value string) bool {
	quoted := fmt.Sprintf("%q", value)
	return len(value) > 0 && strings.Contains(string(body), quoted)
}
