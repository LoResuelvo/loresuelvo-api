package jobrequest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jobRequestRepositoryMock struct {
	savedJobRequest                  []jobrequest.JobRequest
	foundJobRequests                 []readmodel.JobRequestSummary
	foundJobRequest                  *jobrequest.JobRequest
	savedConversation                conversation.Conversation
	saveCalled                       bool
	existsBetweenWithAnyStatusCalled bool
	openExists                       bool
	queriedStatuses                  []jobrequest.Status
	findByIDCalled                   bool
	saveStatusCalled                 bool
	savedStatus                      jobrequest.Status
	err                              error
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

func (r *jobRequestRepositoryMock) ExistsBetweenWithAnyStatus(consumerID, providerID int, statuses []jobrequest.Status) (bool, error) {
	r.existsBetweenWithAnyStatusCalled = true
	r.queriedStatuses = statuses
	if r.err != nil {
		return false, r.err
	}
	return r.openExists, nil
}

func (r *jobRequestRepositoryMock) FindByUserAuthID(userAuthID string) ([]readmodel.JobRequestSummary, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.foundJobRequests, nil
}

func (r *jobRequestRepositoryMock) FindByID(id int) (*jobrequest.JobRequest, error) {
	r.findByIDCalled = true
	if r.err != nil {
		return nil, r.err
	}
	if r.foundJobRequest != nil {
		return r.foundJobRequest, nil
	}
	return nil, jobrequest.ErrJobRequestNotFound
}

func (r *jobRequestRepositoryMock) SaveStatus(ctx context.Context, jobRequest jobrequest.JobRequest) error {
	r.saveStatusCalled = true
	r.savedStatus = jobRequest.Status
	if r.err != nil {
		return r.err
	}
	return nil
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
	exists     bool
	categoryID int
	providerID int
	err        error
}

type userRepositoryMock struct {
	consumer *consumerRepo
	provider *providerRepo
}

func newUserRepositoryMock(consumer *consumerRepo, provider *providerRepo) *userRepositoryMock {
	return &userRepositoryMock{consumer: consumer, provider: provider}
}

func (m *userRepositoryMock) FindIDByAuthID(authID string) (int, error) {
	if m.provider != nil && m.provider.providerID != 0 {
		return m.provider.FindIDByAuthID(authID)
	}
	return m.consumer.FindIDByAuthID(authID)
}

func (m *userRepositoryMock) ExistsProviderByID(id int) (bool, error) {
	return m.provider.ExistsByID(id)
}

func (m *userRepositoryMock) FindProviderByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	return m.provider.FindByID(ctx, providerID)
}

func (m *providerRepo) FindByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	if m.err != nil {
		return nil, m.err
	}
	if !m.exists {
		return nil, provider.ErrDoesNotExist
	}
	return &provider.Provider{BaseUser: &user.BaseUser{ID: providerID, Role: provider.Role}, Category: &category.Category{ID: m.categoryID}}, nil
}

func TestCreateFromChatbotAssessmentCopiesCurrentAssessment(t *testing.T) {
	categoryID := 3
	assessment := &conversation.ProblemAssessment{
		ID: 9, ChatbotConversationID: 7, Version: 2,
		Outcome: conversation.AssessmentProfessionalRequired, ProblemCategoryID: &categoryID,
		ProblemTitle: "Pérdida en el sifón", ProblemDescription: "La pérdida continúa después de ajustar la conexión.", BasedOnMessageID: 20,
	}
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true, categoryID: categoryID}),
		&conversationRepo{foundConversation: &conversation.ChatBotConversation{
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, "", time.Time{}, nil),
			ConsumerID:       10, CurrentAssessment: assessment,
		}},
		fileServiceForJobRequestTest(),
	)

	created, err := service.CreateFromChatbotAssessment(context.Background(), "auth0|consumer", 7, 20)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, repo.savedJobRequest, 1)
	assert.Equal(t, assessment.ProblemTitle, repo.savedJobRequest[0].Title)
	assert.Equal(t, assessment.ProblemDescription, repo.savedJobRequest[0].Description)
	require.NotNil(t, repo.savedJobRequest[0].SourceAssessmentID)
	assert.Equal(t, assessment.ID, *repo.savedJobRequest[0].SourceAssessmentID)
}

func TestCreateFromChatbotAssessmentRejectsSelfServiceOutcome(t *testing.T) {
	categoryID := 3
	service := jobrequest.NewService(
		&jobRequestRepositoryMock{},
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{foundConversation: &conversation.ChatBotConversation{
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, "", time.Time{}, nil),
			ConsumerID:       10,
			CurrentAssessment: &conversation.ProblemAssessment{
				ID: 9, Version: 1, Outcome: conversation.AssessmentSelfService,
				ProblemCategoryID: &categoryID, ProblemTitle: "Problema simple",
				ProblemDescription: "Puede resolverse sin prestador.", BasedOnMessageID: 2,
			},
		}},
		fileServiceForJobRequestTest(),
	)

	created, err := service.CreateFromChatbotAssessment(context.Background(), "auth0|consumer", 7, 20)

	assert.ErrorIs(t, err, jobrequest.ErrAssessmentNotContactable)
	assert.Nil(t, created)
}

func TestCreateFromChatbotAssessmentRejectsDifferentOwner(t *testing.T) {
	service := jobrequest.NewService(
		&jobRequestRepositoryMock{},
		newUserRepositoryMock(&consumerRepo{consumerID: 11}, &providerRepo{exists: true}),
		&conversationRepo{foundConversation: &conversation.ChatBotConversation{
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, "", time.Time{}, nil),
			ConsumerID:       10,
		}},
		fileServiceForJobRequestTest(),
	)

	created, err := service.CreateFromChatbotAssessment(context.Background(), "auth0|other", 7, 20)

	assert.ErrorIs(t, err, jobrequest.ErrChatbotConversationAccessDenied)
	assert.Nil(t, created)
}

func (m *providerRepo) ExistsByID(id int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.exists, nil
}

func (m *providerRepo) FindIDByAuthID(authID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.providerID, nil
}

type conversationRepo struct {
	exists            bool
	foundConversation conversation.Conversation
	savedConversation conversation.Conversation
	findByIDCalled    bool
	saveStatusCalled  bool
	err               error
}

func (m *conversationRepo) ExistsBetween(consumerID, providerID int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.exists, nil
}

func (m *conversationRepo) FindByID(ctx context.Context, conversationID int) (conversation.Conversation, error) {
	m.findByIDCalled = true
	if m.err != nil {
		return nil, m.err
	}
	if m.foundConversation != nil {
		return m.foundConversation, nil
	}
	return nil, conversation.ErrConversationDoesNotExist
}

func (m *conversationRepo) SaveStatus(ctx context.Context, conversation conversation.Conversation) error {
	m.saveStatusCalled = true
	m.savedConversation = conversation
	if m.err != nil {
		return m.err
	}
	return nil
}

type jobRequestImageValidatorMock struct {
	called  bool
	authID  string
	fileIDs []string
	images  []filedomain.Image
	err     error
}

func (m *jobRequestImageValidatorMock) PrepareJobRequestImages(_ context.Context, authID string, fileIDs []string) ([]filedomain.Image, error) {
	m.called = true
	m.authID = authID
	m.fileIDs = append([]string(nil), fileIDs...)
	if m.err != nil {
		return nil, m.err
	}
	return m.images, nil
}

func (m *jobRequestImageValidatorMock) ResolveJobRequestImages(_ context.Context, images []filedomain.Image) ([]filedomain.Image, error) {
	if m.err != nil {
		return nil, m.err
	}
	return images, nil
}

func fileServiceForJobRequestTest() jobrequest.FileService {
	return &jobRequestImageValidatorMock{}
}

func TestCreateJobRequestSavesRequestWithPendingConversation(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "  Reparación de fuga  ", "  Necesito ayuda esta semana  ", []string{})

	firstSavedRequest := repo.savedJobRequest[0]
	require.NoError(t, err)
	require.NotNil(t, createdRequest)
	assert.True(t, repo.existsBetweenWithAnyStatusCalled)
	assert.Equal(t, jobrequest.OpenStatuses(), repo.queriedStatuses)
	assert.True(t, repo.saveCalled)
	assert.Equal(t, 10, firstSavedRequest.ConsumerID)
	assert.Equal(t, 20, firstSavedRequest.ProviderID)
	assert.Equal(t, "Reparación de fuga", firstSavedRequest.Title)
	assert.Equal(t, "Necesito ayuda esta semana", firstSavedRequest.Description)
	assert.Equal(t, jobrequest.StatusPending, firstSavedRequest.Status)
	savedConversation := repo.savedConversation.(*conversation.WorkConversation)
	assert.Equal(t, conversation.StatusPending, savedConversation.Status())
	assert.Equal(t, 10, savedConversation.ConsumerID)
	assert.Equal(t, 20, savedConversation.ProviderID)
	assert.Equal(t, 1, createdRequest.ID)
	assert.Equal(t, 2, createdRequest.ConversationID)
}

func TestCreateJobRequestValidatesImagesWithFileService(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	imageValidator := &jobRequestImageValidatorMock{
		images: []filedomain.Image{{FileID: "file-1", OriginalName: "problema.jpg", URL: "https://files/file-1"}},
	}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		imageValidator,
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "Reparación de fuga", "Necesito ayuda", []string{"file-1"})

	require.NoError(t, err)
	require.NotNil(t, createdRequest)
	assert.True(t, imageValidator.called)
	assert.Equal(t, "auth0|consumer", imageValidator.authID)
	assert.Equal(t, []string{"file-1"}, imageValidator.fileIDs)
	require.Len(t, repo.savedJobRequest, 1)
	assert.Equal(t, []filedomain.Image{{FileID: "file-1", OriginalName: "problema.jpg", URL: "https://files/file-1"}}, repo.savedJobRequest[0].Images)
}

func TestCreateJobRequestAllowsEmptyDescription(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "Reparación de fuga", "   ", []string{})

	require.NoError(t, err)
	require.NotNil(t, createdRequest)
	assert.Equal(t, "", repo.savedJobRequest[0].Description)
}

func TestCreateJobRequestRejectsMissingTitle(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "   ", "Necesito ayuda", []string{})

	assert.ErrorIs(t, err, jobrequest.ErrTitleRequired)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestRejectsNonConsumer(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{err: errors.New("consumer not found")}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|provider", 20, "Reparación de fuga", "", []string{})

	assert.ErrorIs(t, err, jobrequest.ErrOnlyConsumerCanCreateJobRequest)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestRejectsNonExistingProvider(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: false}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "Reparación de fuga", "", []string{})

	assert.ErrorIs(t, err, jobrequest.ErrProviderDoesNotExist)
	assert.Nil(t, createdRequest)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestRejectsExistingOpenRequestBetweenConsumerAndProvider(t *testing.T) {
	repo := &jobRequestRepositoryMock{openExists: true}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "Reparación de fuga", "", []string{})

	assert.ErrorIs(t, err, jobrequest.ErrAlreadyExists)
	assert.Nil(t, createdRequest)
	assert.True(t, repo.existsBetweenWithAnyStatusCalled)
	assert.Equal(t, jobrequest.OpenStatuses(), repo.queriedStatuses)
	assert.False(t, repo.saveCalled)
}

func TestCreateJobRequestPropagatesOpenRequestLookupError(t *testing.T) {
	repo := &jobRequestRepositoryMock{err: errors.New("lookup failed")}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	createdRequest, err := service.Create(context.Background(), "auth0|consumer", 20, "Reparación de fuga", "", []string{})

	assert.ErrorContains(t, err, "lookup failed")
	assert.Nil(t, createdRequest)
	assert.True(t, repo.existsBetweenWithAnyStatusCalled)
	assert.Equal(t, jobrequest.OpenStatuses(), repo.queriedStatuses)
	assert.False(t, repo.saveCalled)
}

func TestShouldGetNoJobRequests(t *testing.T) {
	service := jobrequest.NewService(
		&jobRequestRepositoryMock{},
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	jobRequests, err := service.GetJobRequests(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	assert.Empty(t, jobRequests)
}

func TestSHouldGetListOfJobRequests(t *testing.T) {
	repo := &jobRequestRepositoryMock{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{consumerID: 10}, &providerRepo{exists: true}),
		&conversationRepo{},
		fileServiceForJobRequestTest(),
	)

	repo.foundJobRequests = []readmodel.JobRequestSummary{
		{ID: 1, ConversationID: 11, Title: "Reparación de fuga", Description: "Necesito ayuda esta semana", Status: string(jobrequest.StatusPending), Requester: readmodel.JobRequestRequester{Name: "Ana", Surname: "Perez"}},
		{ID: 2, ConversationID: 12, Title: "Instalación de grifo", Description: "¿Alguien disponible?", Status: string(jobrequest.StatusPending), Requester: readmodel.JobRequestRequester{Name: "Ana", Surname: "Perez"}},
	}
	expectedJobRequests := repo.foundJobRequests

	jobRequests, err := service.GetJobRequests(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	assert.Equal(t, expectedJobRequests, jobRequests)
}

func TestAcceptJobRequestActivatesLinkedConversationForAssignedProvider(t *testing.T) {
	repo := &jobRequestRepositoryMock{
		foundJobRequest: &jobrequest.JobRequest{
			ID:             1,
			ConsumerID:     10,
			ProviderID:     20,
			ConversationID: 30,
			Title:          "Reparación de fuga",
			Description:    "Necesito ayuda esta semana",
			Status:         jobrequest.StatusPending,
		},
	}
	conversationRepo := &conversationRepo{
		foundConversation: &conversation.WorkConversation{
			BaseConversation: conversation.RehydrateBaseConversation(30, conversation.TypeWork, conversation.StatusPending, time.Time{}, nil),
			ConsumerID:       10,
			ProviderID:       20,
		},
	}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{}, &providerRepo{providerID: 20}),
		conversationRepo,
		fileServiceForJobRequestTest(),
	)

	acceptedJobRequest, err := service.Accept(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, acceptedJobRequest)
	assert.Equal(t, 1, acceptedJobRequest.ID)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, conversationRepo.findByIDCalled)
	assert.True(t, conversationRepo.saveStatusCalled)
	assert.True(t, repo.saveStatusCalled)
	assert.Equal(t, jobrequest.StatusAccepted, acceptedJobRequest.Status)
	assert.Equal(t, jobrequest.StatusAccepted, repo.savedStatus)
	assert.Equal(t, conversation.StatusActive, conversationRepo.savedConversation.Status())
}

func TestAcceptJobRequestRejectsProviderThatIsNotAssigned(t *testing.T) {
	repo := &jobRequestRepositoryMock{
		foundJobRequest: &jobrequest.JobRequest{
			ID:             1,
			ConsumerID:     10,
			ProviderID:     20,
			ConversationID: 30,
			Title:          "Reparación de fuga",
			Status:         jobrequest.StatusPending,
		},
	}
	conversationRepo := &conversationRepo{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{}, &providerRepo{providerID: 99}),
		conversationRepo,
		fileServiceForJobRequestTest(),
	)

	acceptedJobRequest, err := service.Accept(context.Background(), "auth0|other-provider", 1)

	assert.ErrorIs(t, err, jobrequest.ErrOnlyAssignedProviderCanAcceptJobRequest)
	assert.Nil(t, acceptedJobRequest)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, conversationRepo.findByIDCalled)
	assert.False(t, conversationRepo.saveStatusCalled)
	assert.False(t, repo.saveStatusCalled)
}

func TestAcceptJobRequestReturnsNotFoundWhenRequestDoesNotExist(t *testing.T) {
	repo := &jobRequestRepositoryMock{err: jobrequest.ErrJobRequestNotFound}
	conversationRepo := &conversationRepo{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{}, &providerRepo{providerID: 20}),
		conversationRepo,
		fileServiceForJobRequestTest(),
	)

	acceptedJobRequest, err := service.Accept(context.Background(), "auth0|provider", 999)

	assert.ErrorIs(t, err, jobrequest.ErrJobRequestNotFound)
	assert.Nil(t, acceptedJobRequest)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, conversationRepo.findByIDCalled)
	assert.False(t, conversationRepo.saveStatusCalled)
	assert.False(t, repo.saveStatusCalled)
}

func TestAcceptJobRequestRejectsAlreadyAcceptedRequest(t *testing.T) {
	repo := &jobRequestRepositoryMock{
		foundJobRequest: &jobrequest.JobRequest{
			ID:             1,
			ConsumerID:     10,
			ProviderID:     20,
			ConversationID: 30,
			Title:          "Reparación de fuga",
			Status:         jobrequest.StatusAccepted,
		},
	}
	conversationRepo := &conversationRepo{}
	service := jobrequest.NewService(
		repo,
		newUserRepositoryMock(&consumerRepo{}, &providerRepo{providerID: 20}),
		conversationRepo,
		fileServiceForJobRequestTest(),
	)

	acceptedJobRequest, err := service.Accept(context.Background(), "auth0|provider", 1)

	assert.ErrorIs(t, err, jobrequest.ErrOnlyPendingJobRequestCanBeAccepted)
	assert.Nil(t, acceptedJobRequest)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, conversationRepo.findByIDCalled)
	assert.False(t, conversationRepo.saveStatusCalled)
	assert.False(t, repo.saveStatusCalled)
}
