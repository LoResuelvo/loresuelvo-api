package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetWorkOrderReturnsScheduledContractWithoutCompletionReport(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := user.RehydrateBaseUser(10, "auth0|consumer", "consumer@example.com", "Ana", "Perez", consumer.Role, nil)
	env := setupWorkOrderDetailServiceTest(order, actor)

	detail, err := env.service.GetWorkOrder(t.Context(), actor.AuthID(), order.ID())

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, order.ID(), detail.ID)
	assert.Equal(t, order.ServiceProposalID(), detail.ServiceProposalID)
	assert.Equal(t, order.ConsumerID(), detail.ConsumerID)
	assert.Equal(t, order.ProviderID(), detail.ProviderID)
	assert.Equal(t, int64(0), detail.Amount)
	assert.Equal(t, string(workorder.StatusScheduled), detail.Status)
	assert.Nil(t, detail.CompletionReport)
	env.reader.AssertExpectations(t)
	env.users.AssertExpectations(t)
}

func TestGetWorkOrderResolvesCompletionImagesInPersistedOrder(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	report, err := workorder.NewCompletionReport(
		"Trabajo finalizado y verificado.",
		[]string{"file-1", "file-2"},
		now,
	)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(order.ProviderID(), report))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	fileService := new(fileServiceMock)
	fileService.On(
		"ResolveWorkOrderCompletionImages",
		mock.Anything,
		[]filedomain.Image{{FileID: "file-1"}, {FileID: "file-2"}},
	).Return([]filedomain.Image{
		{FileID: "file-1", OriginalName: "trabajo.jpg", URL: "https://private/file-1"},
		{FileID: "file-2", OriginalName: "detalle.png", URL: "https://private/file-2"},
	}, nil).Once()
	env := setupWorkOrderDetailServiceTest(order, actor)
	env.service = workorder.NewService(env.reader, env.users, fileService, nil, nil, nil, nil)

	detail, err := env.service.GetWorkOrder(t.Context(), actor.AuthID(), order.ID())

	require.NoError(t, err)
	require.NotNil(t, detail.CompletionReport)
	assert.Equal(t, string(workorder.StatusAwaitingPayment), detail.Status)
	assert.Equal(t, report.Description(), detail.CompletionReport.Description)
	assert.Equal(t, report.ReportedOn(), detail.CompletionReport.ReportedOn)
	assert.Equal(t, []filedomain.Image{
		{FileID: "file-1", OriginalName: "trabajo.jpg", URL: "https://private/file-1"},
		{FileID: "file-2", OriginalName: "detalle.png", URL: "https://private/file-2"},
	}, detail.CompletionReport.Images)
	fileService.AssertExpectations(t)
}

func TestGetWorkOrderAllowsAssignedProvider(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	env := setupWorkOrderDetailServiceTest(order, actor)

	detail, err := env.service.GetWorkOrder(t.Context(), actor.AuthID(), order.ID())

	require.NoError(t, err)
	assert.Equal(t, order.ID(), detail.ID)
}

func TestGetWorkOrderRejectsNonParticipant(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := user.RehydrateBaseUser(99, "auth0|other", "other@example.com", "Other", "User", consumer.Role, nil)
	env := setupWorkOrderDetailServiceTest(order, actor)

	_, err := env.service.GetWorkOrder(t.Context(), actor.AuthID(), order.ID())

	assert.ErrorIs(t, err, workorder.ErrOnlyWorkOrderParticipantCanView)
}

func TestGetWorkOrderReturnsNotFoundForMissingOrder(t *testing.T) {
	actor := user.RehydrateBaseUser(10, "auth0|consumer", "consumer@example.com", "Ana", "Perez", consumer.Role, nil)
	reader := new(readerMock)
	reader.On("FindByID", mock.Anything, 999999).Return(nil, workorder.ErrDoesNotExist).Once()
	users := new(userRepositoryMock)
	service := workorder.NewService(reader, users, nil, nil, nil, nil, nil)

	_, err := service.GetWorkOrder(t.Context(), actor.AuthID(), 999999)

	assert.ErrorIs(t, err, workorder.ErrDoesNotExist)
	users.AssertNotCalled(t, "FindByAuthID", mock.Anything)
}

func TestGetWorkOrderMapsFileResolutionFailure(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	report, err := workorder.NewCompletionReport("Trabajo finalizado", []string{"file-1"}, now)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(order.ProviderID(), report))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	fileService := new(fileServiceMock)
	resolutionErr := errors.New("storage unavailable")
	fileService.On("ResolveWorkOrderCompletionImages", mock.Anything, mock.Anything).Return(nil, resolutionErr).Once()
	env := setupWorkOrderDetailServiceTest(order, actor)
	env.service = workorder.NewService(env.reader, env.users, fileService, nil, nil, nil, nil)

	_, err = env.service.GetWorkOrder(t.Context(), actor.AuthID(), order.ID())

	assert.ErrorIs(t, err, resolutionErr)
	fileService.AssertExpectations(t)
}

type workOrderDetailServiceTestEnv struct {
	reader  *readerMock
	users   *userRepositoryMock
	service *workorder.Service
}

func setupWorkOrderDetailServiceTest(order *workorder.WorkOrder, actor user.User) *workOrderDetailServiceTestEnv {
	reader := new(readerMock)
	reader.On("FindByID", mock.Anything, order.ID()).Return(order, nil).Once()
	users := new(userRepositoryMock)
	users.On("FindByAuthID", actor.AuthID()).Return(actor, nil).Once()
	return &workOrderDetailServiceTestEnv{
		reader:  reader,
		users:   users,
		service: workorder.NewService(reader, users, nil, nil, nil, nil, nil),
	}
}
