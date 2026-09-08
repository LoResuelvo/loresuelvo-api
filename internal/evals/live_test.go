package evals

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGeminiExecutorRequiresOptInAndCredentials(t *testing.T) {
	for _, tc := range []struct {
		name  string
		allow bool
		key   string
	}{{"no opt in", false, "configured"}, {"no key", true, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &roundTripperMock{}
			_, err := newGeminiExecutor(&Dataset{}, executionTestPlan("case"), executionTestLimits(), tc.allow, tc.key, transport)
			require.Error(t, err)
			transport.AssertNotCalled(t, "RoundTrip", mock.Anything)
		})
	}
}
func TestGeminiExecutorRejectsCaseOutsidePlanBeforeNetwork(t *testing.T) {
	transport := &roundTripperMock{}
	executor, err := newGeminiExecutor(&Dataset{}, executionTestPlan("allowed"), executionTestLimits(), true, "fake-key", transport)
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "other")
	var classified *ExecutionError
	require.ErrorAs(t, err, &classified)
	require.Equal(t, "unauthorized_case", classified.Kind)
	require.Zero(t, output.RequestCount)
	transport.AssertNotCalled(t, "RoundTrip", mock.Anything)
}
func TestLimitedTransportBlocksImplicitRetry(t *testing.T) {
	upstream := &roundTripperMock{}
	executor := &GeminiExecutor{plan: executionTestPlan("case"), transport: upstream}
	transport := &limitedTransport{executor: executor}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)
	upstream.On("RoundTrip", request).Return(&http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(""))}, nil).Once()
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	_, err = transport.RoundTrip(request)
	require.ErrorContains(t, err, "implicit SDK retry or redirect blocked")
	require.Equal(t, 1, transport.count)
	upstream.AssertExpectations(t)
}
func TestLimitedTransportCapsRequestsAcrossAttempts(t *testing.T) {
	upstream := &roundTripperMock{}
	executor := &GeminiExecutor{plan: executionTestPlan("case"), transport: upstream}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)
	upstream.On("RoundTrip", request).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil).Once()
	first := &limitedTransport{executor: executor}
	response, err := first.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	second := &limitedTransport{executor: executor}
	_, err = second.RoundTrip(request)
	require.ErrorIs(t, err, ErrRequestBudget)
	require.Zero(t, second.count)
	upstream.AssertExpectations(t)
}
func TestGeminiExecutorBlocksRedirectBeforeSecondRequest(t *testing.T) {
	upstream := &roundTripperMock{}
	dataset := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: PDInput{UserMessage: "Question"}}}}
	upstream.On("RoundTrip", mock.Anything).Return(&http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": []string{"https://example.test/redirected"}}, Body: io.NopCloser(strings.NewReader(""))}, nil).Once()
	executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), executionTestLimits(), true, "fake-key", upstream)
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "case")
	require.Error(t, err)
	require.Equal(t, 1, output.RequestCount)
	upstream.AssertExpectations(t)
}
func TestGeminiExecutorMissingImageNeverCallsTransport(t *testing.T) {
	upstream := &roundTripperMock{}
	image := ImageInput{AssetID: "missing", FileID: "missing", Path: "missing.png"}
	dataset := &Dataset{Root: t.TempDir(), assets: map[string]ImageInput{"missing": image}, PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: PDInput{Images: []ImageInput{image}}}}}
	executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), executionTestLimits(), true, "fake-key", upstream)
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "case")
	var classified *ExecutionError
	require.ErrorAs(t, err, &classified)
	require.Equal(t, "asset_error", classified.Kind)
	require.Zero(t, output.RequestCount)
	upstream.AssertNotCalled(t, "RoundTrip", mock.Anything)
}
