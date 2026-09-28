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
	reader := repositories.NewOperationConversationReader(fixture.testContext.database)
	unshared, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.NotNil(t, unshared.RelatedServiceProposalIDs)
	require.Empty(t, unshared.RelatedServiceProposalIDs)
	require.False(t, unshared.IsShared())
	first := fixture.proposal(t, request, now, serviceproposal.StatusPending)
	unshared, err = reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, []int{first}, unshared.RelatedServiceProposalIDs)
	require.False(t, unshared.IsShared())
	later := fixture.proposal(t, request, now, serviceproposal.StatusPending)
	found, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, consumerID, found.ConsumerID)
	require.Equal(t, providerID, found.ProviderID)
	require.Equal(t, request.ID, *found.JobRequestID)
	require.Equal(t, first, *found.ServiceProposalID)
	require.Equal(t, []int{first, later}, found.RelatedServiceProposalIDs)
	require.True(t, found.IsShared())
	sibling, err := reader.FindConversationAssociation(context.Background(), readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: later})
	require.NoError(t, err)
	require.Equal(t, found.ConversationID, sibling.ConversationID)
	require.Equal(t, later, *sibling.ServiceProposalID)
	require.Equal(t, []int{first, later}, sibling.RelatedServiceProposalIDs)
	require.True(t, sibling.IsShared())
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
	messages, err := reader.FindPage(ctx, conversationID, nil, 20)
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
	for _, limit := range []int{0, 102} {
		messages, err = reader.FindPage(ctx, conversationID, nil, limit)
		require.Nil(t, messages)
		require.ErrorIs(t, err, conversation.ErrInvalidMessagePage)
	}
}

func TestConversationMessagePageReaderContinuesAcrossTimestampTies(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	ctx := context.Background()
	reader := repositories.NewConversationMessagePageReader(fixture.database)
	conversationID := fixture.savedConversation.ID()
	now := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	for _, content := range []string{"Tie A", "Tie B", "Tie C"} {
		_, err := fixture.database.Exec(`INSERT INTO messages (conversation_id,sender_role,content,created_on) VALUES ($1,'consumer',$2,$3)`, conversationID, content, now)
		require.NoError(t, err)
	}
	first, err := reader.FindPage(ctx, conversationID, nil, 3)
	require.NoError(t, err)
	require.Len(t, first, 3)
	require.Equal(t, "Tie A", first[1].Content)
	require.Equal(t, "Tie B", first[2].Content)
	position := &conversation.MessagePosition{ID: first[2].ID, CreatedOn: first[2].CreatedOn}
	second, err := reader.FindPage(ctx, conversationID, position, 3)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, "Tie C", second[0].Content)
	require.Greater(t, second[0].ID, position.ID)
	exhausted, err := reader.FindPage(ctx, conversationID, &conversation.MessagePosition{ID: second[0].ID, CreatedOn: second[0].CreatedOn}, 3)
	require.NoError(t, err)
	require.Empty(t, exhausted)
	require.NotNil(t, exhausted)
}
