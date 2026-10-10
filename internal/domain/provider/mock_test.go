package provider_test

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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

func (reader *providerSearchReaderMock) FindByCategoryAndCoverageZoneID(ctx context.Context, categoryID, coverageZoneID int) ([]readmodel.ProviderSearchResult, error) {
	args := reader.Called(ctx, categoryID, coverageZoneID)
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

type conversionReaderMock struct{ mock.Mock }

func (m *conversionReaderMock) Read(ctx context.Context, providerID int, query provider.ConversionQuery) (*readmodel.ConversionSnapshot, error) {
	args := m.Called(ctx, providerID, query)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.ConversionSnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

type registrationUnitMock struct{ mock.Mock }

func (unit *registrationUnitMock) Execute(ctx context.Context, operation func(provider.RegistrationStore) error) error {
	args := unit.Called(ctx, operation)
	if run, ok := args.Get(0).(func(context.Context, func(provider.RegistrationStore) error) error); ok {
		return run(ctx, operation)
	}
	return args.Error(0)
}

type registrationStoreMock struct{ mock.Mock }

func (store *registrationStoreMock) FindCategory(ctx context.Context, id int) (*category.Category, error) {
	args := store.Called(ctx, id)
	if run, ok := args.Get(0).(func(context.Context, int) (*category.Category, error)); ok {
		return run(ctx, id)
	}
	found, _ := args.Get(0).(*category.Category)
	return found, args.Error(1)
}
func (store *registrationStoreMock) SaveUser(ctx context.Context, toSave user.User) (user.User, error) {
	args := store.Called(ctx, toSave)
	if run, ok := args.Get(0).(func(context.Context, user.User) (user.User, error)); ok {
		return run(ctx, toSave)
	}
	saved, _ := args.Get(0).(user.User)
	return saved, args.Error(1)
}
func newProviderServiceForTest(search provider.ProviderSearchReader, repository provider.UserRepository, finder provider.CategoryFinder, files provider.FileService, profiles provider.ProviderProfileReader, zones provider.CoverageZoneFinder, identities ...provider.IdentityApprovalReader) *provider.Service {
	unit := new(registrationUnitMock)
	store := new(registrationStoreMock)
	unit.On("Execute", mock.Anything, mock.Anything).Return(func(ctx context.Context, operation func(provider.RegistrationStore) error) error {
		return operation(store)
	})
	store.On("FindCategory", mock.Anything, mock.Anything).Return(func(ctx context.Context, id int) (*category.Category, error) { return finder.FindByID(ctx, id) })
	store.On("SaveUser", mock.Anything, mock.Anything).Return(func(ctx context.Context, toSave user.User) (user.User, error) { return repository.Save(ctx, toSave) })
	return provider.NewService(unit, search, repository, finder, files, profiles, zones, nil, identities...)
}

type consumerFinderMock struct{ mock.Mock }

func (finder *consumerFinderMock) FindConsumerByAuthID(ctx context.Context, authID string) (*consumer.Consumer, error) {
	args := finder.Called(ctx, authID)
	found, _ := args.Get(0).(*consumer.Consumer)
	return found, args.Error(1)
}
func searchConsumerFinder(ctx context.Context, authID string, zoneID int) *consumerFinderMock {
	finder := new(consumerFinderMock)
	found := consumer.RehydrateConsumer(nil, consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{ID: zoneID})
	finder.On("FindConsumerByAuthID", ctx, authID).Return(found, nil).Once()
	return finder
}
