package provider_handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConversionHandlerRejectsInvalidQueriesBeforeService(t *testing.T) {
	for _, query := range []string{"from=%GG", "from=a", "from=", "from=2026-09-01T00:00:00", "from=2026-09-01T00:00:00Z&from=2026-09-02T00:00:00Z", "provider_id=2", "granularity=day", "compare_previous=true", "to=2026-09-01T00:00:00Z;bad=true"} {
		t.Run(query, func(t *testing.T) {
			service := &conversionServiceMock{}
			response := conversionRequest(t, service, query, true, context.Background())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
			require.Equal(t, 400, response.Code)
		})
	}
}
func TestConversionHandlerRejectsNonRFC3339SyntaxBeforeService(t *testing.T) {
	for _, timestamp := range []string{
		"2026-09-01T1:00:00Z",
		"2026-09-01T00:00:00,123Z",
		"2026-09-01T00:00:00%2B24:00",
		"2026-09-01T00:00:00%2B03:60",
	} {
		t.Run(timestamp, func(t *testing.T) {
			service := &conversionServiceMock{}
			service.On("Query", mock.Anything, mock.Anything, mock.Anything).Return((*readmodel.Conversion)(nil), nil).Maybe()
			response := conversionRequest(t, service, "from="+timestamp+"&to=2026-09-02T00:00:00Z", true, context.Background())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
			require.Equal(t, 400, response.Code)
		})
	}
}
func TestConversionHandlerRejectsPrecisionBeyondNanosecondsBeforeService(t *testing.T) {
	for _, query := range []string{
		"from=2026-09-01T00:00:00.0000000001Z&to=2026-09-02T00:00:00Z",
		"from=2026-09-01T00:00:00Z&to=2026-09-02T00:00:00.0000000001Z",
		"from=2026-09-01T00:00:00.123456789012345Z&to=2026-09-02T00:00:00Z",
	} {
		t.Run(query, func(t *testing.T) {
			service := &conversionServiceMock{}
			service.On("Query", mock.Anything, mock.Anything, mock.Anything).Return((*readmodel.Conversion)(nil), nil).Maybe()
			response := conversionRequest(t, service, query, true, context.Background())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
			require.Equal(t, 400, response.Code)
		})
	}
}
func TestConversionHandlerAcceptsRFC3339Syntax(t *testing.T) {
	for _, timestamp := range []string{
		"2026-09-01T00:00:00Z",
		"2026-09-01T00:00:00.1Z",
		"2026-09-01T00:00:00.123456789Z",
		"2026-09-01T00:00:00-03:00",
		"2026-09-01T00:00:00%2B03:30",
	} {
		t.Run(timestamp, func(t *testing.T) {
			service := &conversionServiceMock{}
			service.On("Query", mock.Anything, "auth0|provider", mock.Anything).Return(&readmodel.Conversion{}, nil).Once()
			response := conversionRequest(t, service, "from="+timestamp+"&to=2026-09-02T00:00:00Z", true, context.Background())
			require.Equal(t, 200, response.Code)
			service.AssertExpectations(t)
		})
	}
}
func TestConversionHandlerMapsServiceErrors(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{provider.ErrInvalidConversionQuery, 400}, {provider.ErrConversionForbidden, 403}, {provider.ErrConversionProviderNotFound, 404}, {errors.New("private database details"), 500}, {nil, 500},
	} {
		service := &conversionServiceMock{}
		service.On("Query", mock.Anything, "auth0|provider", mock.Anything).Return((*readmodel.Conversion)(nil), test.err).Once()
		response := conversionRequest(t, service, "", true, context.Background())
		require.Equal(t, test.status, response.Code)
		require.NotContains(t, response.Body.String(), "private database details")
		service.AssertExpectations(t)
	}
	require.Equal(t, 500, conversionRequest(t, nil, "", true, context.Background()).Code)
	require.Equal(t, 401, conversionRequest(t, nil, "", false, context.Background()).Code)
}
func TestConversionHandlerPropagatesIdentityContextAndTimestamps(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	from := time.Date(2026, 9, 1, 0, 0, 0, 123456789, time.FixedZone("offset", -3*3600))
	to := from.Add(time.Hour)
	percent := float64(50)
	result := &readmodel.Conversion{
		Period: readmodel.ConversionPeriod{From: from, To: to}, TimeZone: "America/Argentina/Buenos_Aires", ObservedAt: to,
		Proposals: readmodel.ConversionProposals{
			Stages: readmodel.ConversionStages{Issued: 4, Contracted: 2, Reported: 1, Paid: 1}, Uncontracted: 2,
			Rates: readmodel.ConversionRates{Contracted: readmodel.ConversionStageRates{Cohort: readmodel.ConversionRatio{Numerator: 2, Denominator: 4, Percentage: &percent}}},
		},
		Requests: readmodel.ConversionRequests{ConversionRequestCounts: readmodel.ConversionRequestCounts{Received: 3, Accepted: 2, Pending: 1}},
	}
	service := &conversionServiceMock{}
	service.On("Query", mock.Anything, "auth0|provider", mock.Anything).Return(result, nil).Once().Run(func(args mock.Arguments) {
		got := args.Get(0).(context.Context)
		input := args.Get(2).(provider.ConversionQueryInput)
		require.Equal(t, "request", got.Value(contextKey{}))
		require.True(t, input.From.Equal(from))
		require.True(t, input.To.Equal(to))
	})
	response := conversionRequest(t, service, "from=2026-09-01T00:00:00.123456789-03:00&to=2026-09-01T01:00:00.123456789-03:00", true, ctx)
	service.AssertExpectations(t)
	require.Equal(t, 200, response.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Len(t, payload, 4)
	period := payload["period"].(map[string]any)
	require.Len(t, period, 3)
	require.Equal(t, "2026-09-01T03:00:00.123456789Z", period["from"])
	require.Equal(t, "2026-09-01T04:00:00.123456789Z", payload["observed_at"])
	proposals := payload["proposals"].(map[string]any)
	require.Len(t, proposals, 3)
	require.Equal(t, map[string]any{"issued": float64(4), "contracted": float64(2), "reported": float64(1), "paid": float64(1)}, proposals["stages"])
	require.Equal(t, float64(2), proposals["uncontracted"])
	rates := proposals["rates"].(map[string]any)
	require.Len(t, rates, 3)
	for _, stage := range []string{"contracted", "reported", "paid"} {
		entries := rates[stage].(map[string]any)
		require.Len(t, entries, 2)
		for _, basis := range []string{"cohort", "previous_stage"} {
			ratio := entries[basis].(map[string]any)
			require.Len(t, ratio, 3)
			require.Contains(t, ratio, "numerator")
			require.Contains(t, ratio, "denominator")
			require.Contains(t, ratio, "percentage")
			if stage == "contracted" && basis == "cohort" {
				require.Equal(t, float64(50), ratio["percentage"])
			} else {
				require.Nil(t, ratio["percentage"])
			}
		}
	}
	requests := payload["requests"].(map[string]any)
	require.Len(t, requests, 4)
	require.Equal(t, float64(3), requests["received"])
	require.Equal(t, float64(2), requests["accepted"])
	require.Equal(t, float64(1), requests["pending"])
	require.Equal(t, map[string]any{"numerator": float64(0), "denominator": float64(0), "percentage": nil}, requests["acceptance_rate"])
}
