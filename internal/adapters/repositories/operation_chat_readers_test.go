package repositories_test

import (
	"context"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOperationConversationReaderAssociation(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	now := time.Now()
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusAccepted)
	first := fixture.proposal(t, request, now, serviceproposal.StatusPending)
	later := fixture.proposal(t, request, now, serviceproposal.StatusPending)
	reader := repositories.NewOperationConversationReader(fixture.testContext.database)
	found, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, consumerID, found.ConsumerID)
	require.Equal(t, providerID, found.ProviderID)
	require.Equal(t, request.ID, *found.JobRequestID)
	require.Equal(t, first, *found.ServiceProposalID)
	sibling, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: later})
	require.NoError(t, err)
	require.Equal(t, found.ConversationID, sibling.ConversationID)
	require.Equal(t, later, *sibling.ServiceProposalID)
	for _, id := range []readmodel.ID{{Kind: readmodel.KindServiceProposal, ResourceID: first}, {Kind: readmodel.KindJobRequest, ResourceID: 2147483647}} {
		missing, err := reader.FindConversationAssociation(context.Background(), id)
		require.NoError(t, err)
		require.Nil(t, missing)
	}

	otherConsumer := savedConsumerIDWithData(t, fixture.testContext, "auth0|chat-other-consumer", "chat.other.consumer@example.com", "Other", "Consumer")
	_, err = fixture.testContext.database.Exec(`UPDATE work_conversations SET consumer_id = $1 WHERE conversation_id = $2`, otherConsumer, found.ConversationID)
	require.NoError(t, err)
	missing, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Nil(t, missing)

}

func TestConversationMessagePageReaderReadsBoundedText(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	reader := repositories.NewConversationMessagePageReader(fixture.database)
	ctx := context.Background()
	conversationID := fixture.savedConversation.ID()
	now := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	for index := 0; index < 25; index++ {
		_, err := fixture.database.Exec(`INSERT INTO messages (conversation_id,sender_role,content,created_on) VALUES ($1,'consumer',$2,$3)`, conversationID, fmt.Sprintf("Message %02d", index), now)
		require.NoError(t, err)
	}
	messages, err := reader.FindPage(ctx, conversationID, 20)
	require.NoError(t, err)
	require.Len(t, messages, 20)
	require.Equal(t, fixture.initialMessage.Content, messages[0].Content)
	require.Equal(t, "Message 18", messages[19].Content)
	for index := 2; index < len(messages); index++ {
		require.True(t, messages[index-1].CreatedOn.Equal(messages[index].CreatedOn))
		require.Less(t, messages[index-1].ID, messages[index].ID)
	}
	for _, message := range messages {
		require.Equal(t, conversationID, message.ConversationID)
		require.Nil(t, message.Images)
		require.Nil(t, message.Audio)
		require.Nil(t, message.Video)
	}
	for _, limit := range []int{0, 101} {
		messages, err = reader.FindPage(ctx, conversationID, limit)
		require.Nil(t, messages)
		require.ErrorIs(t, err, conversation.ErrInvalidMessagePage)
	}
}
