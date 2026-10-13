package evals

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestGeminiExecutorPreservesWireUsageAndParsedRanking(t *testing.T) {
	for _, usage := range []string{`{"promptTokenCount":12,"candidatesTokenCount":0,"thoughtsTokenCount":0,"cachedContentTokenCount":4,"totalTokenCount":12}`, `{}`} {
		t.Run(usage, func(t *testing.T) {
			upstream := &roundTripperMock{}
			upstream.On("RoundTrip", mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Request-Id": {"http-id"}}, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[]}"}]}}],"usageMetadata":` + usage + `}`))}, nil).Once()
			dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: RKInput{MaxResults: 3}}}}
			executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), executionTestLimits(), true, "fake-key", upstream)
			require.NoError(t, err)
			output, err := executor.Execute(context.Background(), "case")
			require.NoError(t, err)
			require.JSONEq(t, usage, string(output.ProviderUsage))
			require.Equal(t, "http_usage_metadata", output.UsageSource)
			require.Equal(t, "gemini", output.Provider)
			require.Equal(t, http.StatusOK, *output.HTTPStatus)
			require.Equal(t, "http-id", *output.RequestID)
			require.JSONEq(t, `{"recommendations":[]}`, string(output.ParsedOutput))
			upstream.AssertExpectations(t)
		})
	}
}

func TestGeminiExecutorSummaryUsesPreviousMemoryAndWireObject(t *testing.T) {
	upstream := &roundTripperMock{}
	upstream.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		body, err := io.ReadAll(args.Get(0).(*http.Request).Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "previous memory")
		require.Contains(t, string(body), "new fact")
	}).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"updated summary\"}"}]}}]}`))}, nil).Once()
	dataset := &Dataset{SM: []SMCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: SMInput{PreviousSummary: "previous memory", Messages: []MessageInput{{SenderRole: "consumer", Content: "new fact"}}}}}}
	executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), executionTestLimits(), true, "fake-key", upstream)
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "case")
	require.NoError(t, err)
	require.JSONEq(t, `{"summary":"updated summary"}`, string(output.ParsedOutput))
	upstream.AssertExpectations(t)
}

func TestGeminiPoolBindsCredentialToLaneAndRejectsOversubscription(t *testing.T) {
	upstream := &roundTripperMock{}
	limits := executionTestLimits()
	limits.Concurrency = 2
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: RKInput{MaxResults: 3}}}}
	_, err := newGeminiExecutorWithKeys(dataset, executionTestPlan("case"), limits, true, []string{"one-key"}, upstream)
	require.Error(t, err)
	upstream.On("RoundTrip", mock.MatchedBy(func(request *http.Request) bool { return request.Header.Get("x-goog-api-key") == "second-key" })).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[]}"}]}}]}`))}, nil).Once()
	executor, err := newGeminiExecutorWithKeys(dataset, executionTestPlan("case"), limits, true, []string{"first-key", "second-key"}, upstream)
	require.NoError(t, err)
	_, err = executor.ExecuteLane(t.Context(), "case", 1)
	require.NoError(t, err)
	upstream.AssertExpectations(t)
}

func TestGeminiPoolSharesPacingAndRateLimitAcrossPhases(t *testing.T) {
	limits := executionTestLimits()
	limits.MinInterval = 20 * time.Millisecond
	upstream := &roundTripperMock{}
	first, err := newGeminiExecutorWithKeys(&Dataset{}, executionTestPlan("case"), limits, true, []string{"same-key"}, upstream)
	require.NoError(t, err)
	second, err := newGeminiExecutorWithKeys(&Dataset{}, executionTestPlan("case"), limits, true, []string{"same-key"}, upstream)
	require.NoError(t, err)
	ShareGeminiProjectLanes([]*GeminiExecutor{first, second})
	require.Same(t, first.lanes[0], second.lanes[0])
	require.NoError(t, first.lanes[0].wait(t.Context()))
	started := time.Now()
	require.NoError(t, second.lanes[0].wait(t.Context()))
	require.GreaterOrEqual(t, time.Since(started), limits.MinInterval-time.Millisecond)
	first.lanes[0].stop()
	require.ErrorIs(t, second.lanes[0].wait(t.Context()), ErrProjectLaneStopped)
	upstream.AssertNotCalled(t, "RoundTrip", mock.Anything)
}

func TestGeminiParserFailureRetainsRawAndWireUsageWithoutParsedSuccess(t *testing.T) {
	upstream := &roundTripperMock{}
	upstream.On("RoundTrip", mock.Anything).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[],\"extra\":true}"}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":0,"thoughtsTokenCount":0}}`))}, nil).Once()
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: RKInput{MaxResults: 3}}}}
	executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), executionTestLimits(), true, "fake-key", upstream)
	require.NoError(t, err)
	output, err := executor.Execute(t.Context(), "case")
	require.Error(t, err)
	require.Equal(t, `{"recommendations":[],"extra":true}`, output.RawOutput)
	require.Nil(t, output.ParsedOutput)
	require.JSONEq(t, `{"promptTokenCount":1,"candidatesTokenCount":0,"thoughtsTokenCount":0}`, string(output.ProviderUsage))
}

func TestGeminiCountInputTokensBoundsHungPreflightRequest(t *testing.T) {
	upstream := &roundTripperMock{}
	upstream.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		<-args.Get(0).(*http.Request).Context().Done()
	}).Return((*http.Response)(nil), context.DeadlineExceeded).Once()
	limits := executionTestLimits()
	limits.AttemptTimeout = 20 * time.Millisecond
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "case"}, Input: RKInput{MaxResults: 3}}}}
	executor, err := newGeminiExecutor(dataset, executionTestPlan("case"), limits, true, "fake-key", upstream)
	require.NoError(t, err)
	started := time.Now()
	_, err = executor.CountInputTokens(context.Background(), "case")
	require.Error(t, err)
	require.Less(t, time.Since(started), time.Second)
	upstream.AssertExpectations(t)
}

func TestGeminiCountInputTokensBoundsPreflightPacingBeforeNetwork(t *testing.T) {
	upstream := &roundTripperMock{}
	limits := executionTestLimits()
	limits.AttemptTimeout = 20 * time.Millisecond
	executor, err := newGeminiExecutorWithKeys(&Dataset{}, executionTestPlan("case"), limits, true, []string{"fake-key"}, upstream)
	require.NoError(t, err)
	executor.lanes[0].next = time.Now().Add(time.Hour)
	_, err = executor.CountInputTokens(context.Background(), "case")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	upstream.AssertNotCalled(t, "RoundTrip", mock.Anything)
}

func TestGeminiStoppedProjectAcrossRunsDoesNotLoseUnsentSlots(t *testing.T) {
	for _, concurrency := range []int{1, 2} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			upstream := &roundTripperMock{}
			limits := executionTestLimits()
			limits.Concurrency = 2
			dataset := &Dataset{}
			for _, id := range []string{"first", "second", "third"} {
				dataset.RK = append(dataset.RK, RKCase{CaseMetadata: CaseMetadata{ID: id}, Input: RKInput{MaxResults: 3}})
			}
			upstream.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool { return r.Header.Get("x-goog-api-key") == "first-key" })).Return(&http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED"}}`))}, nil).Once()
			for range 2 {
				upstream.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool { return r.Header.Get("x-goog-api-key") == "second-key" })).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[]}"}]}}]}`))}, nil).Once()
			}
			firstPlan := executionTestPlan("first")
			first, err := newGeminiExecutorWithKeys(dataset, firstPlan, limits, true, []string{"first-key", "second-key"}, upstream)
			require.NoError(t, err)
			secondPlan := executionTestPlan("second", "third")
			secondLimits := limits
			secondLimits.Concurrency = concurrency
			second, err := newGeminiExecutorWithKeys(dataset, secondPlan, secondLimits, true, []string{"first-key", "second-key"}, upstream)
			require.NoError(t, err)
			ShareGeminiProjectLanes([]*GeminiExecutor{first, second})
			journal, record, _ := executionTestJournal(t, firstPlan)
			_, err = Run(t.Context(), dataset, firstPlan, limits, first, journal, record, CredentialsRedactor(nil))
			require.NoError(t, err)
			journal, record, directory := executionTestJournal(t, secondPlan)
			actual, err := Run(t.Context(), dataset, secondPlan, secondLimits, second, journal, record, CredentialsRedactor(nil))
			require.NoError(t, err)
			require.Equal(t, "completed", actual.Status)
			_, attempts, err := ReadRun(directory)
			require.NoError(t, err)
			require.Len(t, attempts, 2)
			for _, attempt := range attempts {
				require.Equal(t, "executed", attempt.Status)
				require.Equal(t, 1, attempt.RequestCount)
			}
			upstream.AssertExpectations(t)
		})
	}
}
