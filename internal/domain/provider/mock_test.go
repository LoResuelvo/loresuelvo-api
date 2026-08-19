package provider_test

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type providerProfileReaderMock struct {
	statsByProviderID map[int]provider.RatingStats
	providerIDs       []int
	stats             provider.RatingStats
	workOrders        []readmodel.WorkOrder
	ratingStatsErr    error
	batchStatsErr     error
	workHistoryErr    error
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
