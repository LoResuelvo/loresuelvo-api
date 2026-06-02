package jobrequest_test

import (
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jobRequestRepositoryMock struct {
	savedJobRequest   []jobrequest.JobRequest
	savedConversation conversation.Conversation
	saveCalled        bool
	err               error
}

func (r *jobRequestRepositoryMock) SaveWithConversation(jobRequest jobrequest.JobRequest, pendingConversation conversation.Conversation) (*jobrequest.JobRequest, error) {
	r.saveCalled = true
	r.savedJobRequest = append(r.savedJobRequest, jobRequest)
	r.savedConversation = pendingConversation
	if r.err != nil {
		return nil, r.err
	}

	jobRequest.ID = 1
	jobRequest.ConversationID = 2
	return &jobRequest, nil
}

func (r *jobRequestRepositoryMock) FindByUserAuthID(userAuthID string) ([]jobrequest.JobRequest, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.savedJobRequest, nil
}

type consumerRepo struct {
	consumerID int
	err        error
}

func (m *consumerRepo) FindIDByAuthID(authID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.consumerID, nil
}

type providerRepo struct {
	exists bool
	err    error
}

func (m *providerRepo) ExistsByID(id int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.exists, nil
}

type conversationRepo struct {
	exists bool
	err    error
}

func (m *conversationRepo) ExistsBetween(consumerID, providerID int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.exists, nil
}

func TestCreateJobRequestSavesRequestWithPendingConversation(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	createdRequest, err := service.Create("auth0|consumer", 20, "  Reparación de fuga  ", "  Necesito ayuda esta semana  ")

	firstSavedRequest := repo.savedJobRequest[0]
	require.NoError(t, err)
	require.NotNil(t, createdRequest)
	assert.True(t, repo.saveCalled)
	assert.Equal(t, 10, firstSavedRequest.ConsumerID)
	assert.Equal(t, 20, firstSavedRequest.ProviderID)
	assert.Equal(t, "Reparación de fuga", firstSavedRequest.Title)
	assert.Equal(t, "Necesito ayuda esta semana", firstSavedRequest.Description)
	assert.Equal(t, conversation.StatusPending, repo.savedConversation.Status)
	assert.Equal(t, 10, repo.savedConversation.ConsumerID)
	assert.Equal(t, 20, repo.savedConversation.ProviderID)
	assert.Equal(t, 1, createdRequest.ID)
	assert.Equal(t, 2, createdRequest.ConversationID)
}

func TestCreateJobRequestAllowsEmptyDescription(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	createdRequest, err := service.Create("auth0|consumer", 20, "Reparación de fuga", "   ")

	require.NoError(t, err)
	require.NotNil(t, createdRequest)
	assert.Equal(t, "", repo.savedJobRequest[0].Description)
}

func TestCreateJobRequestRejectsMissingTitle(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	createdRequest, err := service.Create("auth0|consumer", 20, "   ", "Necesito ayuda")

	assert.ErrorIs(t, err, jobrequest.ErrTitleRequired)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestRejectsNonConsumer(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{err: errors.New("consumer not found")},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	createdRequest, err := service.Create("auth0|provider", 20, "Reparación de fuga", "")

	assert.ErrorIs(t, err, jobrequest.ErrOnlyConsumerCanCreateJobRequest)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestRejectsNonExistingProvider(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: false},
		&conversationRepo{},
	)

	createdRequest, err := service.Create("auth0|consumer", 20, "Reparación de fuga", "")

	assert.ErrorIs(t, err, jobrequest.ErrProviderDoesNotExist)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestShouldGetNoJobRequests(t *testing.T) {
	service := jobrequest.NewService(
		&jobRequestRepositoryMock{},
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	jobRequests, err := service.GetJobRequests("auth0|consumer")

	require.NoError(t, err)
	assert.Empty(t, jobRequests)
}

func TestSHouldGetListOfJobRequests(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		&consumerRepo{consumerID: 10},
		&providerRepo{exists: true},
		&conversationRepo{},
	)

	repo.savedJobRequest = []jobrequest.JobRequest{
		{ID: 1, ConsumerID: 10, ProviderID: 20, Title: "Reparación de fuga", Description: "Necesito ayuda esta semana"},
		{ID: 2, ConsumerID: 10, ProviderID: 30, Title: "Instalación de grifo", Description: "¿Alguien disponible?"},
	}
	expectedJobRequests := repo.savedJobRequest

	jobRequests, err := service.GetJobRequests("auth0|consumer")

	require.NoError(t, err)
	assert.Equal(t, expectedJobRequests, jobRequests)
}
