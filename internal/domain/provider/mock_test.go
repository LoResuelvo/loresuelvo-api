package provider_test

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ratingStatsReaderMock struct {
	stats provider.RatingStats
	err   error
}

func (reader *ratingStatsReaderMock) FindRatingStatsByProviderID(_ context.Context, _ int) (provider.RatingStats, error) {
	return reader.stats, reader.err
}

type paidWorkHistoryReaderMock struct {
	workOrders []readmodel.WorkOrder
	err        error
}

func (reader *paidWorkHistoryReaderMock) FindPaidWorkHistoryByProviderID(_ context.Context, _ int) ([]readmodel.WorkOrder, error) {
	return reader.workOrders, reader.err
}
