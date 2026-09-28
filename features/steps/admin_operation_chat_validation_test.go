package steps_test

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/cucumber/godog"
)

func registerAdminOperationChatValidationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con la cabecera X-Audit-Reason ausente$`, suite.queryOperationChatWithoutReasonHeader)
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con la cabecera X-Audit-Reason en blanco$`, suite.queryOperationChatWithBlankReasonHeader)
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con una cabecera X-Audit-Reason que supera la longitud máxima documentada$`, suite.queryOperationChatWithExcessiveReasonHeader)
	sc.Step(`^la respuesta no contiene mensajes ni URLs de adjuntos$`, suite.operationChatErrorHasNoPrivateData)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" con motivo "([^"]*)" y límite 0$`, suite.queryOperationChatWithZeroLimit)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" con motivo "([^"]*)" y límite mayor que el máximo documentado$`, suite.queryOperationChatWithExcessiveLimit)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" con motivo "([^"]*)" y cursor "no-es-un-cursor"$`, suite.queryOperationChatWithUnreadableCursor)
	sc.Step(`^intento consultar el chat administrativo de la operación "([^"]*)" con motivo "([^"]*)"$`, suite.queryOperationChatWithRawID)
}

func (suite *testSuite) queryOperationChatWithoutReasonHeader(label string) error {
	return suite.queryOperationChatWithReasonHeader(label, nil)
}
func (suite *testSuite) queryOperationChatWithBlankReasonHeader(label string) error {
	reason := "   "
	return suite.queryOperationChatWithReasonHeader(label, &reason)
}
func (suite *testSuite) queryOperationChatWithExcessiveReasonHeader(label string) error {
	reason := strings.Repeat("a", 501)
	return suite.queryOperationChatWithReasonHeader(label, &reason)
}
func (suite *testSuite) queryOperationChatWithReasonHeader(label string, reason *string) error {
	operationID, err := suite.persistedOperationChatRequestID(label)
	if err != nil {
		return err
	}
	return suite.sendOperationChatGetWithAuditReason(operationID, "", nil, reason)
}
func (suite *testSuite) persistedOperationChatRequestID(label string) (string, error) {
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return "", fmt.Errorf("unknown request %q", label)
	}
	// Keep a real existing operation so input validation cannot mask absence.
	persisted, err := suite.jobRequestRepository.FindByID(request.id)
	if err != nil {
		return "", err
	}
	return "jr-" + strconv.Itoa(persisted.ID), nil
}
func (suite *testSuite) operationChatErrorHasNoPrivateData() error {
	if err := suite.adminDetailErrorHasNoData(); err != nil {
		return err
	}
	body := string(suite.lastBody)
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		return fmt.Errorf("chat error response exposes an attachment URL")
	}
	return nil
}

func (suite *testSuite) queryOperationChatWithZeroLimit(label, reason string) error {
	return suite.queryExistingOperationChatWithQuery(label, reason, url.Values{"limit": {"0"}})
}
func (suite *testSuite) queryOperationChatWithExcessiveLimit(label, reason string) error {
	return suite.queryExistingOperationChatWithQuery(label, reason, url.Values{"limit": {strconv.Itoa(operation.MaxChatPageSize + 1)}})
}
func (suite *testSuite) queryOperationChatWithUnreadableCursor(label, reason string) error {
	return suite.queryExistingOperationChatWithQuery(label, reason, url.Values{"cursor": {"no-es-un-cursor"}})
}
func (suite *testSuite) queryExistingOperationChatWithQuery(label, reason string, query url.Values) error {
	operationID, err := suite.persistedOperationChatRequestID(label)
	if err != nil {
		return err
	}
	return suite.sendOperationChatGetWithQuery(operationID, reason, "", query)
}
func (suite *testSuite) queryOperationChatWithRawID(id, reason string) error {
	return suite.sendOperationChatGet(url.PathEscape(id), reason, "")
}
