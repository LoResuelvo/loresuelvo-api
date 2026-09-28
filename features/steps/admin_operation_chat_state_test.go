package steps_test

import (
	"fmt"
	"reflect"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type operationChatReadState struct {
	conversationLabel string
	conversation      conversation.Conversation
	request           *jobrequest.JobRequest
	businessCounts    map[string]int64
	operatorAuthID    string
}

func registerAdminOperationChatStateSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" tiene mensajes y un estado persistidos$`, suite.operationChatHasPersistedReadBaseline)
	sc.Step(`^"([^"]*)" conserva sus mensajes y estado$`, suite.operationChatMessagesAndStateUnchanged)
	sc.Step(`^no se crean mensajes, no se acepta ninguna solicitud y el operador no queda suscripto a canales de participantes$`, suite.operationChatDoesNotMutateBusinessOrSubscribe)
}

func (suite *testSuite) operationChatHasPersistedReadBaseline(label string) error {
	conversationID, ok := suite.operationChat.conversations[label]
	if !ok {
		return fmt.Errorf("unknown conversation %q", label)
	}
	var requestLabel string
	for name, request := range suite.operationInbox.requests {
		if request.conversationID == conversationID {
			requestLabel = name
		}
	}
	if requestLabel == "" {
		return fmt.Errorf("conversation has no prepared request")
	}
	request := suite.operationInbox.requests[requestLabel]
	fixture := testsupport.OperationChatFixture{DB: suite.database}
	for index, role := range []string{conversation.SenderConsumer, conversation.SenderProvider} {
		_, err := fixture.AddMessage(suite.scenarioContext, conversationID, role, "Persisted private read-only message from "+role, request.createdOn.Add(time.Duration(index+1)*time.Minute))
		if err != nil {
			return err
		}
	}
	persisted, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
	if err != nil {
		return err
	}
	if persisted.Status() != conversation.StatusActive || len(persisted.Messages()) != 2 || persisted.UpdatedOn().IsZero() {
		return fmt.Errorf("expected active persisted conversation with two messages and update instant")
	}
	persistedRequest, err := suite.jobRequestRepository.FindByID(request.id)
	if err != nil {
		return err
	}
	counts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	operatorAuthID := suite.currentAuth0ID
	if operatorAuthID == "" || suite.dependencies.Runtime.Hub.HasConnectionsForAuthID(operatorAuthID) {
		return fmt.Errorf("read baseline requires an authenticated operator without participant connections")
	}
	suite.operationChat.readState = operationChatReadState{conversationLabel: label, conversation: persisted, request: persistedRequest, businessCounts: counts, operatorAuthID: operatorAuthID}
	return nil
}

func (suite *testSuite) operationChatMessagesAndStateUnchanged(label string) error {
	before := suite.operationChat.readState
	if before.conversationLabel != label || before.conversation == nil {
		return fmt.Errorf("conversation read baseline was not established")
	}
	after, err := suite.conversationRepository.FindByID(suite.scenarioContext, before.conversation.ID())
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.conversation, after) {
		return fmt.Errorf("administrative read changed persisted conversation messages, state or instants")
	}
	return nil
}

func (suite *testSuite) operationChatDoesNotMutateBusinessOrSubscribe() error {
	before := suite.operationChat.readState
	if before.request == nil || before.businessCounts == nil || before.operatorAuthID == "" {
		return fmt.Errorf("business and operator read baselines were not established")
	}
	afterRequest, err := suite.jobRequestRepository.FindByID(before.request.ID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.request, afterRequest) {
		return fmt.Errorf("administrative read changed persisted job request")
	}
	afterCounts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.businessCounts, afterCounts) {
		return fmt.Errorf("administrative read changed persisted business row counts")
	}
	if suite.currentAuth0ID != before.operatorAuthID || suite.dependencies.Runtime.Hub.HasConnectionsForAuthID(before.operatorAuthID) {
		return fmt.Errorf("administrative read subscribed operator to participant connections")
	}
	return nil
}
