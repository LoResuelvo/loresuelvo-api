package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jobRequestRepositoryTestContext struct {
	database               *sql.DB
	userRepository         *repositories.UserRepository
	categoryRepository     *repositories.CategoryRepository
	conversationRepository *repositories.ConversationRepository
	jobRequestRepository   *repositories.JobRequestRepository
}

func cleanJobRequestRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM job_requests")
	require.NoError(t, err, "could not clean job requests")

	_, err = database.Exec("DELETE FROM messages")
	require.NoError(t, err, "could not clean messages")

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
	messageRepository := repositories.NewMessageRepository(database, repositories.NewMessageImageRepository(database))

	return jobRequestRepositoryTestContext{
		database:               database,
		userRepository:         userRepository,
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

	consumerToSave, err := consumer.NewConsumer(authID, email, name, surname, nil)
	require.NoError(t, err)
	_, err = testContext.userRepository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)

	consumerID, err := testContext.userRepository.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)

	return consumerID
}

func savedProviderIDForJobRequest(t *testing.T, testContext jobRequestRepositoryTestContext) int {
	t.Helper()

	return savedProviderIDWithData(t, testContext, "auth0|job-request-provider", "job.request.provider@example.com", "Juan", "Gomez", "Plomeria")
}

func savedProviderIDWithData(t *testing.T, testContext jobRequestRepositoryTestContext, authID, email, name, surname, categoryName string) int {
	t.Helper()

	providerToSave := validProviderWithData(t, testContext.categoryRepository, testContext.database, authID, email, name, surname, categoryName)
	_, err := testContext.userRepository.Save(context.Background(), providerUser(providerToSave))
	require.NoError(t, err)

	providerID, err := testContext.userRepository.FindIDByEmail(providerToSave.Email())
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

	return pendingConversation
}

func validJobRequest(t *testing.T, consumerID, providerID int) jobrequest.JobRequest {
	t.Helper()

	requestToSave, err := jobrequest.New(consumerID, providerID, "Reparacion de fuga", "Necesito ayuda esta semana", nil)
	require.NoError(t, err)

	return *requestToSave
}

func savedJobRequestImage(t *testing.T, testContext jobRequestRepositoryTestContext, fileID, originalName, uploaderAuthID string) filedomain.Image {
	t.Helper()

	_, err := testContext.database.Exec(
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, 'private', $3, 'image/jpeg', 1024, 'confirmed', 'private', 'job_request_image', $4, NOW(), NOW())`,
		fileID,
		"files/2026/06/job_request_image/"+fileID+"/"+originalName,
		originalName,
		uploaderAuthID,
	)
	require.NoError(t, err)

	return filedomain.Image{FileID: fileID, OriginalName: originalName}
}

func TestJobRequestRepositoryCanSaveRequestWithConversation(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	requestToSave.Images = []filedomain.Image{
		savedJobRequestImage(t, testContext, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "perdida-bajo-mesada.jpg", "auth0|job-request-consumer"),
	}
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
	assert.Equal(t, jobrequest.StatusPending, savedJobRequest.Status)
	assert.Equal(t, requestToSave.Images, savedJobRequest.Images)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedJobRequest.ConversationID)
	require.NoError(t, err)
	foundWorkConversation := foundConversation.(*conversation.WorkConversation)
	assert.Equal(t, consumerID, foundWorkConversation.ConsumerID)
	assert.Equal(t, providerID, foundWorkConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, foundWorkConversation.Status())

	foundJobRequest, err := testContext.jobRequestRepository.FindByConversationID(savedJobRequest.ConversationID)
	require.NoError(t, err)
	assert.Equal(t, requestToSave.Images, foundJobRequest.Images)
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

func TestJobRequestRepositoryCanDetectRequestBetweenConsumerAndProviderWithAnyStatus(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	_, err := testContext.jobRequestRepository.SaveWithConversation(validJobRequest(t, consumerID, providerID), conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)

	exists, err := testContext.jobRequestRepository.ExistsBetweenWithAnyStatus(consumerID, providerID, jobrequest.OpenStatuses())

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestJobRequestRepositoryReturnsFalseWhenNoRequestExistsBetweenConsumerAndProviderWithAnyStatus(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)

	exists, err := testContext.jobRequestRepository.ExistsBetweenWithAnyStatus(consumerID, providerID, jobrequest.OpenStatuses())

	require.NoError(t, err)
	assert.False(t, exists)
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
	assert.Equal(t, jobrequest.StatusPending, foundJobRequest.Status)
}

func TestJobRequestRepositoryCanFindByID(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	requestToSave := validJobRequest(t, consumerID, providerID)
	pendingConversation := conversationForJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, pendingConversation)
	require.NoError(t, err)

	foundJobRequest, err := testContext.jobRequestRepository.FindByID(savedJobRequest.ID)

	require.NoError(t, err)
	require.NotNil(t, foundJobRequest)
	assert.Equal(t, savedJobRequest.ID, foundJobRequest.ID)
	assert.Equal(t, consumerID, foundJobRequest.ConsumerID)
	assert.Equal(t, providerID, foundJobRequest.ProviderID)
	assert.Equal(t, savedJobRequest.ConversationID, foundJobRequest.ConversationID)
	assert.Equal(t, requestToSave.Title, foundJobRequest.Title)
	assert.Equal(t, requestToSave.Description, foundJobRequest.Description)
	assert.Equal(t, jobrequest.StatusPending, foundJobRequest.Status)
}

func TestJobRequestRepositoryFindByIDReturnsNotFoundIfRequestDoesNotExist(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)

	foundJobRequest, err := testContext.jobRequestRepository.FindByID(999999999)

	assert.ErrorIs(t, err, jobrequest.ErrJobRequestNotFound)
	assert.Nil(t, foundJobRequest)
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
	assert.Equal(t, string(jobrequest.StatusPending), foundJobRequests[0].Status)
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
	assert.Equal(t, string(jobrequest.StatusPending), foundJobRequests[0].Status)
}

func TestJobRequestRepositoryFindByUserAuthIDReturnsOnlyPendingRequests(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, testContext, "auth0|job-request-active-consumer", "job.request.active.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, testContext, "auth0|job-request-active-provider", "job.request.active.provider@example.com", "Juan", "Gomez", "Plomeria")
	requestToSave := validJobRequest(t, consumerID, providerID)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(requestToSave, conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	savedJobRequest.Status = jobrequest.StatusAccepted
	require.NoError(t, testContext.jobRequestRepository.SaveStatus(context.Background(), *savedJobRequest))

	foundJobRequests, err := testContext.jobRequestRepository.FindByUserAuthID("auth0|job-request-active-consumer")

	require.NoError(t, err)
	assert.Empty(t, foundJobRequests)
}

func TestJobRequestRepositoryCanSaveStatus(t *testing.T) {
	testContext := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, testContext)
	savedJobRequest, err := testContext.jobRequestRepository.SaveWithConversation(validJobRequest(t, consumerID, providerID), conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	savedJobRequest.Status = jobrequest.StatusAccepted

	err = testContext.jobRequestRepository.SaveStatus(context.Background(), *savedJobRequest)

	require.NoError(t, err)
	foundJobRequest, err := testContext.jobRequestRepository.FindByID(savedJobRequest.ID)
	require.NoError(t, err)
	assert.Equal(t, jobrequest.StatusAccepted, foundJobRequest.Status)
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
