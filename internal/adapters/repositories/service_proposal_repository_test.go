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
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serviceProposalRepositoryTestContext struct {
	database                  *sql.DB
	userRepository            *repositories.UserRepository
	categoryRepository        *repositories.CategoryRepository
	conversationRepository    *repositories.ConversationRepository
	serviceProposalRepository *repositories.ServiceProposalRepository
	workOrderRepository       *repositories.WorkOrderRepository
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
	serviceProposalRepository := repositories.NewServiceProposalRepository(database)
	workOrderRepository := repositories.NewWorkOrderRepository(database, serviceProposalRepository)

	return serviceProposalRepositoryTestContext{
		database:                  database,
		userRepository:            userRepository,
		categoryRepository:        repositories.NewCategoryRepository(database),
		conversationRepository:    repositories.NewConversationRepository(database, messageRepository),
		serviceProposalRepository: serviceProposalRepository,
		workOrderRepository:       workOrderRepository,
	}
}

func cleanServiceProposalRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM work_orders")
	require.NoError(t, err, "could not clean work orders")

	_, err = database.Exec("DELETE FROM service_proposals")
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

func TestServiceProposalRepositorySavesAcceptanceWithWorkOrderAtomically(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|acceptance-consumer", "acceptance.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|acceptance-provider", "acceptance.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)

	proposal, err := serviceproposal.NewServiceProposal(
		&provider.Provider{BaseUser: &user.BaseUser{ID: providerID}},
		&consumer.Consumer{BaseUser: &user.BaseUser{ID: consumerID}},
		activeConversation,
		1500050,
		time.Now().Add(24*time.Hour).UTC().Truncate(time.Microsecond),
		"Reparacion de perdida de agua.",
		clockadapter.NewSystemClock(),
	)
	require.NoError(t, err)
	proposal, err = testContext.serviceProposalRepository.Save(proposal)
	require.NoError(t, err)

	acceptedOn := time.Now().UTC().Truncate(time.Microsecond)
	order, err := workorder.New(proposal, acceptedOn)
	require.NoError(t, err)
	proposal.Status = serviceproposal.StatusAccepted

	savedOrder, err := testContext.workOrderRepository.Save(context.Background(), order)

	require.NoError(t, err)
	require.NotNil(t, savedOrder)
	assert.NotZero(t, savedOrder.ID)
	assert.Equal(t, workorder.StatusScheduled, savedOrder.Status)

	var storedProposalStatus serviceproposal.Status
	require.NoError(t, testContext.database.QueryRow(
		"SELECT status FROM service_proposals WHERE id = $1",
		proposal.ID,
	).Scan(&storedProposalStatus))
	assert.Equal(t, serviceproposal.StatusAccepted, storedProposalStatus)

	var storedProposalID int
	var storedStatus workorder.Status
	var storedAcceptedOn time.Time
	require.NoError(t, testContext.database.QueryRow(
		`SELECT service_proposal_id, status, accepted_on
		FROM work_orders
		WHERE id = $1`,
		savedOrder.ID,
	).Scan(&storedProposalID, &storedStatus, &storedAcceptedOn))
	assert.Equal(t, proposal.ID, storedProposalID)
	assert.Equal(t, workorder.StatusScheduled, storedStatus)
	assert.Equal(t, acceptedOn, storedAcceptedOn.UTC())

	foundOrder, err := testContext.workOrderRepository.FindByServiceProposalID(context.Background(), proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, savedOrder.ID, foundOrder.ID)
	assert.Equal(t, savedOrder.ServiceProposal.ServiceProposalID(), foundOrder.ServiceProposal.ServiceProposalID())
	assert.Equal(t, savedOrder.Status, foundOrder.Status)
	assert.Equal(t, savedOrder.AcceptedOn.UTC(), foundOrder.AcceptedOn.UTC())
}

func TestWorkOrderRepositoryReturnsNotFoundForMissingServiceProposal(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)

	foundOrder, err := testContext.workOrderRepository.FindByServiceProposalID(context.Background(), 999999)

	assert.Nil(t, foundOrder)
	assert.ErrorIs(t, err, workorder.ErrDoesNotExist)
}

func TestServiceProposalRepositoryCanSave(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|service-proposal-consumer", "service.proposal.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|service-proposal-provider", "service.proposal.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)
	scheduledOn := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	proposalToSave, err := serviceproposal.NewServiceProposal(
		&provider.Provider{BaseUser: &user.BaseUser{ID: providerID}},
		&consumer.Consumer{BaseUser: &user.BaseUser{ID: consumerID}},
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
	assert.Equal(t, activeConversation.ID(), savedProposal.Conversation.ID())
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
	assert.Equal(t, activeConversation.ID(), storedConversationID)
	assert.Equal(t, proposalToSave.Amount, storedAmount)
	assert.Equal(t, proposalToSave.ScheduledOn, storedScheduledOn.UTC())
	assert.Equal(t, proposalToSave.Description, storedDescription)
	assert.Equal(t, proposalToSave.Status, storedStatus)
}

func TestServiceProposalRepositoryFindsPendingProposalForConsumer(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|proposal-list-consumer", "proposal.list.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|proposal-list-provider", "proposal.list.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)

	expected, err := serviceproposal.NewServiceProposal(
		&provider.Provider{BaseUser: &user.BaseUser{ID: providerID}},
		&consumer.Consumer{BaseUser: &user.BaseUser{ID: consumerID}},
		activeConversation,
		1500050,
		time.Now().Add(24*time.Hour).UTC().Truncate(time.Microsecond),
		"Reparacion de perdida de agua en cocina con materiales incluidos.",
		clockadapter.NewSystemClock(),
	)
	require.NoError(t, err)
	expected, err = testContext.serviceProposalRepository.Save(expected)
	require.NoError(t, err)

	found, err := testContext.serviceProposalRepository.FindByUserID(context.Background(), consumerID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, expected.ID, found[0].ID)
	assert.Equal(t, consumerID, found[0].Consumer.ID)
	assert.Equal(t, providerID, found[0].Provider.ID)
	assert.Equal(t, "Juan", found[0].Provider.Base().Name)
	assert.Equal(t, "Gomez", found[0].Provider.Base().Surname)
	assert.Equal(t, "Plomeria", found[0].Provider.Category.Name)
	assert.Equal(t, activeConversation.ID(), found[0].Conversation.ID())
	assert.Equal(t, serviceproposal.StatusPending, found[0].Status)
	assert.Equal(t, expected.Amount, found[0].Amount)
	assert.Equal(t, expected.ScheduledOn, found[0].ScheduledOn.UTC())
	assert.Equal(t, expected.Description, found[0].Description)
}
