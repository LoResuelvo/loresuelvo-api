package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type detailAuditSuccessState struct {
	correlation string
	before      *operationDetailAuditSnapshot
	counts      map[string]int64
	verified    bool
}

func registerAdminOperationDetailAuditSuccessSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe un evento de auditoría para la correlación "([^"]*)"$`, suite.detailAuditCorrelationIsAbsent)
	sc.Step(`^consulto el detalle administrativo de la solicitud "([^"]*)" con la correlación "([^"]*)"$`, suite.queryDetailWithCorrelation)
	sc.Step(`^el sistema responde con estado 200 y el detalle completo de "([^"]*)"$`, suite.detailAuditSuccessResponse)
	sc.Step(`^antes de entregar el detalle queda preparado exactamente un evento de acceso a "([^"]*)" con el ID persistido de "([^"]*)", el operador "([^"]*)" y la correlación "([^"]*)"$`, suite.detailAuditPreparedOnce)
	sc.Step(`^la solicitud "([^"]*)" y sus relaciones conservan exactamente los estados y los instantes previos a la consulta$`, suite.detailAuditBusinessValuesUnchanged)
	sc.Step(`^la consulta no crea ni modifica propuestas, órdenes, pagos, conversaciones ni mensajes$`, suite.detailAuditBusinessCountsUnchanged)
}

func (suite *testSuite) detailAuditCorrelationIsAbsent(correlation string) error {
	if correlation == "" {
		return fmt.Errorf("audit correlation must not be empty")
	}
	if err := testsupport.ResetOperationDetailAuditCorrelation(suite.scenarioContext, suite.database, correlation); err != nil {
		return err
	}
	count, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("audit correlation %q already exists (%d events)", correlation, count)
	}
	suite.detailAuditSuccess.correlation = correlation
	return nil
}

func (suite *testSuite) queryDetailWithCorrelation(label, correlation string) error {
	if correlation != suite.detailAuditSuccess.correlation {
		return fmt.Errorf("unexpected audit correlation %q", correlation)
	}
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	before, err := suite.captureOperationDetailAuditSnapshot(label)
	if err != nil {
		return err
	}
	counts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	suite.detailAuditSuccess.before = before
	suite.detailAuditSuccess.counts = counts
	return suite.sendAdminGet(operationsInboxPath+"/jr-"+strconv.Itoa(fixture.id), nil, correlation)
}

func (suite *testSuite) detailAuditSuccessResponse(label string) error {
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("expected complete detail 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	var detail struct {
		ID         string          `json:"id"`
		JobRequest json.RawMessage `json:"job_request"`
		Consumer   json.RawMessage `json:"consumer"`
		Provider   json.RawMessage `json:"provider"`
		Timeline   json.RawMessage `json:"timeline"`
	}
	if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
		return err
	}
	if detail.ID != fmt.Sprintf("jr-%d", fixture.id) || len(detail.JobRequest) == 0 || len(detail.Consumer) == 0 || len(detail.Provider) == 0 || len(detail.Timeline) == 0 {
		return fmt.Errorf("response is missing operation detail: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) detailAuditPreparedOnce(resourceType, label, email, correlation string) error {
	if correlation != suite.detailAuditSuccess.correlation {
		return fmt.Errorf("unexpected audit correlation %q", correlation)
	}
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(email))
	if err != nil {
		return err
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceID := strconv.Itoa(fixture.id)
	filter := audit.LogFilter{OperatorID: &operatorID, ResourceType: &resourceType, ResourceID: &resourceID}
	events, err := reader.FindPage(suite.scenarioContext, filter, watermark, nil, 100)
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		if event.ResourceID() != resourceID || event.Action() != audit.ActionAccess || event.Result() != audit.ResultPrepared {
			return fmt.Errorf("audit event has incorrect access evidence: resource=%s action=%s result=%s", event.ResourceID(), event.Action(), event.Result())
		}
		persisted, err := reader.FindByID(suite.scenarioContext, event.ID())
		if err != nil || persisted == nil {
			return fmt.Errorf("audit event was not persisted before response: %v", err)
		}
		matches++
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	if matches != 1 || total != 1 || suite.operationDetailAuditCapture.attempts != 1 {
		return fmt.Errorf("expected one persisted access event and one synchronous save, got page matches=%d total=%d saves=%d", matches, total, suite.operationDetailAuditCapture.attempts)
	}
	suite.detailAuditSuccess.verified = true
	return nil
}

func (suite *testSuite) detailAuditBusinessValuesUnchanged(label string) error {
	before := suite.detailAuditSuccess.before
	if before == nil || !suite.detailAuditSuccess.verified {
		return fmt.Errorf("audit success baseline or event was not verified")
	}
	after, err := suite.captureOperationDetailAuditSnapshot(label)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.request, after.request) || !reflect.DeepEqual(before.conversation, after.conversation) || !reflect.DeepEqual(before.proposals, after.proposals) {
		return fmt.Errorf("operation values changed during detail read")
	}
	if after.auditMark != before.auditMark+1 {
		return fmt.Errorf("expected exactly one new audit event, watermark moved from %d to %d", before.auditMark, after.auditMark)
	}
	return nil
}

func (suite *testSuite) detailAuditBusinessCountsUnchanged() error {
	after, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(suite.detailAuditSuccess.counts, after) {
		return fmt.Errorf("business row counts changed during operation detail read: before=%v after=%v", suite.detailAuditSuccess.counts, after)
	}
	return nil
}
