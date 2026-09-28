package steps_test

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func registerAdminOperationChatValidationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con la cabecera X-Audit-Reason ausente$`, suite.queryOperationChatWithoutReasonHeader)
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con la cabecera X-Audit-Reason en blanco$`, suite.queryOperationChatWithBlankReasonHeader)
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con una cabecera X-Audit-Reason que supera la longitud máxima documentada$`, suite.queryOperationChatWithExcessiveReasonHeader)
	sc.Step(`^la respuesta no contiene mensajes ni URLs de adjuntos$`, suite.operationChatErrorHasNoPrivateData)
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
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	// Keep a real existing operation so invalid reason checks cannot mask absence.
	persisted, err := suite.jobRequestRepository.FindByID(request.id)
	if err != nil {
		return err
	}
	return suite.sendOperationChatGetWithAuditReason("jr-"+strconv.Itoa(persisted.ID), "", nil, reason)
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
