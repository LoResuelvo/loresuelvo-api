package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jobRequestRepositoryTestContext struct {
	database               *sql.DB
	consumerRepository     *repositories.ConsumerRepository
	providerRepository     *repositories.ProviderRepository
	categoryRepository     *repositories.CategoryRepository
	conversationRepository *repositories.ConversationRepository
	jobRequestRepository   *repositories.JobRequestRepository
}

func cleanJobRequestRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

func newJobRequestRepositoryTest(t *testing.T) jobRequestRepositoryTestContext {
	t.Helper()

	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanJobRequestRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanJobRequestRepositoryTestDatabase(t, database)

	userRepository := repositories.NewUserRepository(database)
	messageRepository := repositories.NewMessageRepository(database)

	return jobRequestRepositoryTestContext{
		database:               database,
		consumerRepository:     repositories.NewConsumerRepository(database, userRepository),
		providerRepository:     repositories.NewProviderRepository(database, userRepository),
		categoryRepository:     repositories.NewCategoryRepository(database),
		conversationRepository: repositories.NewConversationRepository(database, messageRepository),
		jobRequestRepository:   repositories.NewJobRequestRepository(database),
	}
}

func savedConsumerIDForJobRequest(t *testing.T, testContext jobRequestRepositoryTestContext) int {
	t.Helper()

	return savedConsumerIDWithData(t, testContext, "auth0|job-request-consumer", "job.request.consumer@example.com", "Ana", "Perez")
}

func savedConsumerIDWithData(t *testing.T, testContext jobRequestRepositoryTestContext, authID, email, name, surname string) int {
	t.Helper()

	consumerToSave, err := consumer.NewConsumer(authID, email, name, surname)
	require.NoError(t, err)
	require.NoError(t, testContext.consumerRepository.Save(*consumerToSave))

	consumerID, err := testContext.consumerRepository.FindIDByEmail(consumerToSave.User.Email)
	require.NoError(t, err)

	return consumerID
}

func savedProviderIDForJobRequest(t *testing.T, testContext jobRequestRepositoryTestContext) int {
	t.Helper()

	return savedProviderIDWithData(t, testContext, "auth0|job-request-provider", "job.request.provider@example.com", "Juan", "Gomez", "Plomeria")
}

func savedProviderIDWithData(t *testing.T, testContext jobRequestRepositoryTestContext, authID, email, name, surname, categoryName string) int {
	t.Helper()

	providerToSave := validProviderWithData(t, testContext.categoryRepository, authID, email, name, surname, categoryName)
	require.NoError(t, testContext.providerRepository.Save(*providerToSave))

	providerID, err := testContext.providerRepository.FindIDByEmail(providerToSave.User.Email)
	require.NoError(t, err)

	return providerID
}

func savedJobRequestParticipants(t *testing.T, testContext jobRequestRepositoryTestContext) (int, int) {
	t.Helper()

	return savedConsumerIDForJobRequest(t, testContext), savedProviderIDForJobRequest(t, testContext)
}

func conversationForJobRequest(t *testing.T, consumerID, providerID int) conversation.Conversation {
	t.Helper()

	pendingConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)

	return *pendingConversation
}

func validJobRequest(t *testing.T, consumerID, providerID int) jobrequest.JobRequest {
	t.Helper()

	requestToSave, err := jobrequest.New(consumerID, providerID, "Reparacion de fuga", "Necesito ayuda esta semana")
	require.NoError(t, err)

	return *requestToSave
}

func TestJobRequestRepositoryCanSaveRequestWithConversation(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	pendingConversation := conversationForJobRequest(t, consumerID, providerID)

	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)

	require.NoError(t, err)
	require.NotNil(t, savedJobRequest)
	assert.NotZero(t, savedJobRequest.ID)
	assert.Equal(t, consumerID, savedJobRequest.ConsumerID)
	assert.Equal(t, providerID, savedJobRequest.ProviderID)
	assert.NotZero(t, savedJobRequest.ConversationID)
	assert.Equal(t, requestToSave.Title, savedJobRequest.Title)
	assert.Equal(t, requestToSave.Description, savedJobRequest.Description)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedJobRequest.ConversationID)
	require.NoError(t, err)
	assert.Equal(t, consumerID, foundConversation.ConsumerID)
	assert.Equal(t, providerID, foundConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, foundConversation.Status)
}

func TestJobRequestRepositoryRejectsDuplicateRequestBetweenSameConsumerAndProvider(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	pendingConversation := conversationForJobRequest(t, consumerID, providerID)

	_, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)
	require.NoError(t, err)

	duplicateJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)

	assert.ErrorIs(t, err, jobrequest.ErrAlreadyExists)
	assert.Nil(t, duplicateJobRequest)
}

func TestJobRequestRepositoryCanFindByConversationID(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	pendingConversation := conversationForJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)
	require.NoError(t, err)

	foundJobRequest, err := testContext.jobRequestRepository.FindByConversationID(savedJobRequest.ConversationID)

	require.NoError(t, err)
	require.NotNil(t, foundJobRequest)
	assert.Equal(t, savedJobRequest.ID, foundJobRequest.ID)
	assert.Equal(t, consumerID, foundJobRequest.ConsumerID)
	assert.Equal(t, providerID, foundJobRequest.ProviderID)
	assert.Equal(t, savedJobRequest.ConversationID, foundJobRequest.ConversationID)
	assert.Equal(t, requestToSave.Title, foundJobRequest.Title)
	assert.Equal(t, requestToSave.Description, foundJobRequest.Description)
}

func TestJobRequestRepositoryCanFindPendingRequestsByConsumerAuthID(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-consumer-owner", "job.request.consumer.owner@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, testContext, "auth0|job-request-provider-owner", "job.request.provider.owner@example.com", "Juan", "Gomez", "Plomeria")
	unrelatedConsumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-other-consumer", "job.request.other.consumer@example.com", "Carla", "Lopez")
	unrelatedProviderID := savedProviderIDWithData(t, testContext, "auth0|job-request-other-provider", "job.request.other.provider@example.com", "Pedro", "Diaz", "Electricidad")

	requestToFind := validJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToFind, conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	_, err = testContext.jobRequestRepository.SaveWithConversation(validJobRequest(t, unrelatedConsumerID, unrelatedProviderID), conversationForJobRequest(t, unrelatedConsumerID, unrelatedProviderID))
	require.NoError(t, err)

	foundJobRequests, err := testContext.jobRequestRepository.FindByUserAuthID("auth0|job-request-consumer-owner")

	require.NoError(t, err)
	require.Len(t, foundJobRequests, 1)
	assert.Equal(t, savedJobRequest.ID, foundJobRequests[0].ID)
	assert.Equal(t, savedJobRequest.ConversationID, foundJobRequests[0].ConversationID)
	assert.Equal(t, "Ana", foundJobRequests[0].Requester.Name)
	assert.Equal(t, "Perez", foundJobRequests[0].Requester.Surname)
}

func TestJobRequestRepositoryCanFindPendingRequestsByProviderAuthID(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-provider-view-consumer", "job.request.provider.view.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, testContext, "auth0|job-request-provider-view-owner", "job.request.provider.view.owner@example.com", "Juan", "Gomez", "Plomeria")
	unrelatedConsumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-provider-view-other-consumer", "job.request.provider.view.other.consumer@example.com", "Carla", "Lopez")
	unrelatedProviderID := savedProviderIDWithData(t, testContext, "auth0|job-request-provider-view-other-provider", "job.request.provider.view.other.provider@example.com", "Pedro", "Diaz", "Electricidad")

	requestToFind := validJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToFind, conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	_, err = testContext.jobRequestRepository.SaveWithConversation(validJobRequest(t, unrelatedConsumerID, unrelatedProviderID), conversationForJobRequest(t, unrelatedConsumerID, unrelatedProviderID))
	require.NoError(t, err)

	foundJobRequests, err := testContext.jobRequestRepository.FindByUserAuthID("auth0|job-request-provider-view-owner")

	require.NoError(t, err)
	require.Len(t, foundJobRequests, 1)
	assert.Equal(t, savedJobRequest.ID, foundJobRequests[0].ID)
	assert.Equal(t, savedJobRequest.ConversationID, foundJobRequests[0].ConversationID)
	assert.Equal(t, "Ana", foundJobRequests[0].Requester.Name)
	assert.Equal(t, "Perez", foundJobRequests[0].Requester.Surname)
}

func TestJobRequestRepositoryFindByUserAuthIDReturnsOnlyPendingRequests(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-active-consumer", "job.request.active.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, testContext, "auth0|job-request-active-provider", "job.request.active.provider@example.com", "Juan", "Gomez", "Plomeria")
	requestToSave := validJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	_, err = testContext.database.Exec(`UPDATE conversations SET status = 'active' WHERE id = $1`, savedJobRequest.ConversationID)
	require.NoError(t, err)

	foundJobRequests, err := testContext.jobRequestRepository.FindByUserAuthID("auth0|job-request-active-consumer")

	require.NoError(t, err)
	assert.Empty(t, foundJobRequests)
}

func TestJobRequestRepositoryCanDeleteAllRequests(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	pendingConversation := conversationForJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)
	require.NoError(t, err)

	err = testContext.jobRequestRepository.DeleteAll()

	require.NoError(t, err)
	foundJobRequest, err := testContext.jobRequestRepository.FindByConversationID(savedJobRequest.ConversationID)
	assert.Error(t, err)
	assert.Nil(t, foundJobRequest)
}
