package repositories_test

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/mock"
)

// administrationRecordReadBarrier observes a real unit of work and coordinates
// its first two missing-record reads, forcing the cross-claim uniqueness race.
type administrationRecordReadBarrier struct {
	unit      claim.AdministrationUnitOfWork
	arrived   atomic.Int32
	conflicts atomic.Int32
	ready     chan struct{}
}

func (barrier *administrationRecordReadBarrier) Execute(ctx context.Context, operation func(claim.AdministrationStore) error) error {
	err := barrier.unit.Execute(ctx, func(store claim.AdministrationStore) error {
		return operation(&administrationRecordReadBarrierStore{AdministrationStore: store, barrier: barrier})
	})
	if errors.Is(err, claim.ErrAdministrationRecordConflict) {
		barrier.conflicts.Add(1)
	}
	return err
}

type administrationRecordReadBarrierStore struct {
	claim.AdministrationStore
	barrier *administrationRecordReadBarrier
}

func (store *administrationRecordReadBarrierStore) FindRecord(ctx context.Context, operator int, key string) (*claim.AdministrationRecord, error) {
	record, err := store.AdministrationStore.FindRecord(ctx, operator, key)
	if err != nil || record != nil {
		return record, err
	}
	if store.barrier.arrived.Add(1) == 2 {
		close(store.barrier.ready)
	}
	select {
	case <-store.barrier.ready:
		return record, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// administrationCommitBarrier holds only the first successful callback before
// its real PostgreSQL commit, so a replay must wait on the same claim row.
type administrationCommitBarrier struct {
	unit     claim.AdministrationUnitOfWork
	calls    atomic.Int32
	prepared chan struct{}
	release  chan struct{}
}

func (barrier *administrationCommitBarrier) Execute(ctx context.Context, operation func(claim.AdministrationStore) error) error {
	first := barrier.calls.Add(1) == 1
	return barrier.unit.Execute(ctx, func(store claim.AdministrationStore) error {
		if err := operation(store); err != nil {
			return err
		}
		if !first {
			return nil
		}
		close(barrier.prepared)
		select {
		case <-barrier.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// conversationAccessReadBarrier pauses after the real projection and before authorization/detail.
type conversationAccessReadBarrier struct {
	conversation.Reader
	mock.Mock
}

func (m *conversationAccessReadBarrier) FindAccessByID(ctx context.Context, id int) (*readmodel.ConversationAccess, error) {
	access, err := m.Reader.FindAccessByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Called(ctx, id)
	return access, nil
}

type conversationEmptyFileServiceMock struct {
	conversation.FileService
	mock.Mock
}

func (m *conversationEmptyFileServiceMock) PrepareMessageImages(ctx context.Context, authID string, ids []string) ([]filedomain.MessageImage, error) {
	args := m.Called(ctx, authID, ids)
	return args.Get(0).([]filedomain.MessageImage), args.Error(1)
}
func (m *conversationEmptyFileServiceMock) ResolveMessageImages(ctx context.Context, ids []string) (map[string]filedomain.MessageImage, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[string]filedomain.MessageImage), args.Error(1)
}
func (m *conversationEmptyFileServiceMock) ResolveMessageAudios(ctx context.Context, ids []string) (map[string]filedomain.MessageAudio, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[string]filedomain.MessageAudio), args.Error(1)
}
func (m *conversationEmptyFileServiceMock) ResolveMessageVideos(ctx context.Context, ids []string) (map[string]filedomain.MessageVideo, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[string]filedomain.MessageVideo), args.Error(1)
}

type conversationMessagePublisherMock struct{ mock.Mock }

func (m *conversationMessagePublisherMock) PublishMessage(ctx context.Context, c conversation.Conversation, authID string, message conversation.Message) {
	m.Called(ctx, c, authID, message)
}

func (m *conversationEmptyFileServiceMock) ResolvePublicURL(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
