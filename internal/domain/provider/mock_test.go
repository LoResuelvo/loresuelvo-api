package provider_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/mock"
)

type activityReaderMock struct{ mock.Mock }

type collectionReaderMock struct{ mock.Mock }

func (m *collectionReaderMock) ReadSummary(ctx context.Context, providerID int, query provider.ActivityQuery) (*readmodel.CollectionSnapshot, error) {
	args := m.Called(ctx, providerID, query)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.CollectionSnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *collectionReaderMock) ReadDetail(ctx context.Context, providerID int, query provider.CollectionDetailQuery) (*readmodel.CollectionDetailSnapshot, error) {
	args := m.Called(ctx, providerID, query)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.CollectionDetailSnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *activityReaderMock) Read(ctx context.Context, providerID int, query provider.ActivityQuery) (*readmodel.ActivitySnapshot, error) {
	args := m.Called(ctx, providerID, query)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.ActivitySnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

type providerActorFinderMock struct{ mock.Mock }

func (m *providerActorFinderMock) FindByAuthID(ctx context.Context, authID string) (int, string, error) {
	args := m.Called(ctx, authID)
	return args.Int(0), args.String(1), args.Error(2)
}

type activityClockMock struct{ mock.Mock }

func (m *activityClockMock) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

type providerProfileReaderMock struct {
	statsByProviderID map[int]provider.RatingStats
	providerIDs       []int
	stats             provider.RatingStats
	workOrders        []readmodel.WorkOrder
	ratingStatsErr    error
	batchStatsErr     error
	workHistoryErr    error
}

type identityApprovalReaderMock struct {
	approvedByProviderID map[int]bool
	providerIDs          []int
	err                  error
}

func (reader *identityApprovalReaderMock) FindApprovedByProviderIDs(_ context.Context, providerIDs []int) (map[int]bool, error) {
	reader.providerIDs = append([]int(nil), providerIDs...)
	return reader.approvedByProviderID, reader.err
}

func (reader *providerProfileReaderMock) FindRatingStatsByProviderID(_ context.Context, _ int) (provider.RatingStats, error) {
	return reader.stats, reader.ratingStatsErr
}

func (reader *providerProfileReaderMock) FindRatingStatsByProviderIDs(_ context.Context, providerIDs []int) (map[int]provider.RatingStats, error) {
	reader.providerIDs = append([]int(nil), providerIDs...)
	return reader.statsByProviderID, reader.batchStatsErr
}

func (reader *providerProfileReaderMock) FindPaidWorkHistoryByProviderID(_ context.Context, _ int) ([]readmodel.WorkOrder, error) {
	return reader.workOrders, reader.workHistoryErr
}

type providerSearchReaderMock struct{ mock.Mock }

func (reader *providerSearchReaderMock) FindByCategoryID(ctx context.Context, categoryID int) ([]readmodel.ProviderSearchResult, error) {
	args := reader.Called(ctx, categoryID)
	return args.Get(0).([]readmodel.ProviderSearchResult), args.Error(1)
}

type reputationReaderMock struct{ mock.Mock }

func (m *reputationReaderMock) Read(ctx context.Context, providerID int, query provider.ReputationQuery) (*readmodel.ReputationSnapshot, error) {
	args := m.Called(ctx, providerID, query)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.ReputationSnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}
