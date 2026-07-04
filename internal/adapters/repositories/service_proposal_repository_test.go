package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serviceProposalRepositoryTestContext struct {
	database                  *sql.DB
	consumerRepository        *repositories.ConsumerRepository
	providerRepository        *repositories.ProviderRepository
	categoryRepository        *repositories.CategoryRepository
	conversationRepository    *repositories.ConversationRepository
	serviceProposalRepository *repositories.ServiceProposalRepository
}

func newServiceProposalRepositoryTest(t *testing.T) serviceProposalRepositoryTestContext {
	t.Helper()

	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanServiceProposalRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanServiceProposalRepositoryTestDatabase(t, database)

	userRepository := repositories.NewUserRepository(database)
	messageRepository := repositories.NewMessageRepository(database, repositories.NewMessageImageRepository(database))

	return serviceProposalRepositoryTestContext{
		database:                  database,
		consumerRepository:        repositories.NewConsumerRepository(database, userRepository),
		providerRepository:        repositories.NewProviderRepository(database, userRepository),
		categoryRepository:        repositories.NewCategoryRepository(database),
		conversationRepository:    repositories.NewConversationRepository(database, messageRepository),
		serviceProposalRepository: repositories.NewServiceProposalRepository(database),
	}
}

func cleanServiceProposalRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM service_proposals")
	require.NoError(t, err, "could not clean service proposals")

	_, err = database.Exec("DELETE FROM conversations")
	require.NoError(t, err, "could not clean conversations")

	_, err = database.Exec("DELETE FROM providers")
	require.NoError(t, err, "could not clean providers")

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM files")
	require.NoError(t, err, "could not clean files")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

func TestServiceProposalRepositoryCanSave(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		consumerRepository: testContext.consumerRepository,
	}, "auth0|service-proposal-consumer", "service.proposal.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		providerRepository: testContext.providerRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|service-proposal-provider", "service.proposal.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)
	scheduledOn := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	proposalToSave, err := serviceproposal.NewServiceProposal(
		&provider.Provider{ID: providerID},
		&consumer.Consumer{ID: consumerID},
		activeConversation,
		1500050,
		scheduledOn,
		"Reparacion de perdida de agua en cocina con materiales incluidos.",
		clockadapter.NewSystemClock(),
	)
	require.NoError(t, err)

	savedProposal, err := testContext.serviceProposalRepository.Save(proposalToSave)

	require.NoError(t, err)
	require.NotNil(t, savedProposal)
	assert.NotZero(t, savedProposal.ID)
	assert.Equal(t, consumerID, savedProposal.Consumer.ID)
	assert.Equal(t, providerID, savedProposal.Provider.ID)
	assert.Equal(t, activeConversation.Base().ID, savedProposal.Conversation.Base().ID)
	assert.Equal(t, proposalToSave.Amount, savedProposal.Amount)
	assert.Equal(t, proposalToSave.ScheduledOn, savedProposal.ScheduledOn)
	assert.Equal(t, proposalToSave.Description, savedProposal.Description)
	assert.Equal(t, serviceproposal.StatusPending, savedProposal.Status)

	var storedConsumerID, storedProviderID, storedConversationID int
	var storedAmount int64
	var storedScheduledOn time.Time
	var storedDescription string
	var storedStatus serviceproposal.Status
	err = testContext.database.QueryRow(
		`SELECT consumer_id, provider_id, conversation_id, amount_cents, scheduled_on, description, status
		FROM service_proposals
		WHERE id = $1`,
		savedProposal.ID,
	).Scan(
		&storedConsumerID,
		&storedProviderID,
		&storedConversationID,
		&storedAmount,
		&storedScheduledOn,
		&storedDescription,
		&storedStatus,
	)
	require.NoError(t, err)
	assert.Equal(t, consumerID, storedConsumerID)
	assert.Equal(t, providerID, storedProviderID)
	assert.Equal(t, activeConversation.Base().ID, storedConversationID)
	assert.Equal(t, proposalToSave.Amount, storedAmount)
	assert.Equal(t, proposalToSave.ScheduledOn, storedScheduledOn.UTC())
	assert.Equal(t, proposalToSave.Description, storedDescription)
	assert.Equal(t, proposalToSave.Status, storedStatus)
}
