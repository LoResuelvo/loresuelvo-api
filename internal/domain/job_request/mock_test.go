package jobrequest_test

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/stretchr/testify/mock"
)

type creationUnitMock struct{ mock.Mock }

func (unit *creationUnitMock) Execute(ctx context.Context, operation func(jobrequest.CreationStore) error) error {
	args := unit.Called(ctx, operation)
	if run, ok := args.Get(0).(func(context.Context, func(jobrequest.CreationStore) error) error); ok {
		return run(ctx, operation)
	}
	return args.Error(0)
}

type creationStoreMock struct{ mock.Mock }

func (store *creationStoreMock) FindProviderCategory(ctx context.Context, id int) (*category.Category, error) {
	args := store.Called(ctx, id)
	found, _ := args.Get(0).(*category.Category)
	return found, args.Error(1)
}
func (store *creationStoreMock) SaveWithConversation(ctx context.Context, request jobrequest.JobRequest, pending conversation.Conversation) (*jobrequest.JobRequest, error) {
	args := store.Called(ctx, request, pending)
	if run, ok := args.Get(0).(func(jobrequest.JobRequest, conversation.Conversation) (*jobrequest.JobRequest, error)); ok {
		return run(request, pending)
	}
	found, _ := args.Get(0).(*jobrequest.JobRequest)
	return found, args.Error(1)
}
func newCreationUnitForTest(repository *jobRequestRepositoryMock) *creationUnitMock {
	unit := new(creationUnitMock)
	store := new(creationStoreMock)
	unit.On("Execute", mock.Anything, mock.Anything).Return(func(_ context.Context, operation func(jobrequest.CreationStore) error) error { return operation(store) })
	store.On("FindProviderCategory", mock.Anything, mock.Anything).Return(&category.Category{ID: 1, Name: "Plomería", NormalizedName: "plomería", Enabled: true, Version: 1}, nil)
	store.On("SaveWithConversation", mock.Anything, mock.Anything, mock.Anything).Return(func(request jobrequest.JobRequest, pending conversation.Conversation) (*jobrequest.JobRequest, error) {
		return repository.SaveWithConversation(request, pending)
	})
	return unit
}
