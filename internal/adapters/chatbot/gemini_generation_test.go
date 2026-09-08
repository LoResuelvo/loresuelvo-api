package chatbot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGeminiGenerationOptionsRejectNegativeTokenLimit(t *testing.T) {
	_, err := NewGeminiChatbotWithOptions("model", "key", GeminiOptions{MaxOutputTokens: -1})
	require.ErrorIs(t, err, ErrInvalidGeminiOptions)
}

func TestGeminiGenerationTracePreservesInvalidOutputAndMetadata(t *testing.T) {
	transport := &generationTransportMock{}
	transport.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		request := args.Get(0).(*http.Request)
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		config := body["generationConfig"].(map[string]any)
		require.Equal(t, float64(128), config["maxOutputTokens"])
		require.Equal(t, "application/json", config["responseMimeType"])
	}).Return(&http.Response{StatusCode: 200, Header: http.Header{"Secret": {"credential"}}, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"invalid JSON"}],"role":"model"}}],"modelVersion":"observed-model","responseId":"response-1","usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3,"totalTokenCount":15}}`))}, nil).Once()
	var traces []GenerationTrace
	bot, err := NewGeminiChatbotWithOptions("requested-model", "credential", GeminiOptions{HTTPClient: &http.Client{Transport: transport}, MaxOutputTokens: 128, Observer: func(_ context.Context, trace GenerationTrace) { traces = append(traces, trace) }})
	require.NoError(t, err)
	_, err = bot.RankProviders(context.Background(), conversation.ProviderRankingRequest{ProblemTitle: "test problem"})
	require.Error(t, err)
	require.Len(t, traces, 1)
	trace := traces[0]
	require.Equal(t, "invalid JSON", trace.RawText)
	require.Equal(t, "rank_chatbot_providers", trace.Operation)
	require.Contains(t, string(trace.Contents), "test problem")
	require.Contains(t, string(trace.Response), `"modelVersion":"observed-model"`)
	require.Contains(t, string(trace.Response), `"totalTokenCount":15`)
	encoded, err := json.Marshal(trace)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "credential")
	require.NotContains(t, string(encoded), "sdkHttpResponse")
	transport.AssertExpectations(t)
}

func TestGeminiGenerationTraceDoesNotPersistTransportErrorSecrets(t *testing.T) {
	transport := &generationTransportMock{}
	transport.On("RoundTrip", mock.Anything).Return((*http.Response)(nil), errors.New("credential in URL")).Once()
	var trace GenerationTrace
	bot, err := NewGeminiChatbotWithOptions("model", "credential", GeminiOptions{HTTPClient: &http.Client{Transport: transport}, Observer: func(_ context.Context, observed GenerationTrace) { trace = observed }})
	require.NoError(t, err)
	_, err = bot.SummarizeHomeProblemConversation(context.Background(), "prior summary", nil)
	require.Error(t, err)
	require.Equal(t, "generation_failed", trace.Error)
	require.Nil(t, trace.Response)
	require.Contains(t, string(trace.Contents), "prior summary")
	transport.AssertExpectations(t)
}

func TestRankProvidersSendsBoundedObjectResponseSchema(t *testing.T) {
	transport := &generationTransportMock{}
	transport.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		request := args.Get(0).(*http.Request)
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		config := body["generationConfig"].(map[string]any)
		schema := config["responseSchema"].(map[string]any)
		require.Equal(t, "OBJECT", schema["type"])
		require.Equal(t, []any{"recommendations"}, schema["required"])
		recommendations := schema["properties"].(map[string]any)["recommendations"].(map[string]any)
		require.Equal(t, "ARRAY", recommendations["type"])
		require.Equal(t, float64(2), recommendations["maxItems"])
		item := recommendations["items"].(map[string]any)
		require.Equal(t, "OBJECT", item["type"])
		require.ElementsMatch(t, []any{"reference", "reason"}, item["required"])
	}).Return(&http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[]}"}]}}]}`))}, nil).Once()
	bot, err := NewGeminiChatbotWithOptions("model", "key", GeminiOptions{HTTPClient: &http.Client{Transport: transport}})
	require.NoError(t, err)

	_, err = bot.RankProviders(context.Background(), conversation.ProviderRankingRequest{MaxResults: 2})

	require.NoError(t, err)
	transport.AssertExpectations(t)
}

func TestGeminiGenerationDefaultConfigIsUnchanged(t *testing.T) {
	bot := NewGeminiChatbot("model", "key")
	encoded, err := json.Marshal(bot.generationConfig())
	require.NoError(t, err)
	require.JSONEq(t, `{"responseMimeType":"application/json"}`, string(encoded))
	require.Nil(t, bot.options.Observer)
}

func TestGeminiGenerationObserverPreservesRequestForEachOperation(t *testing.T) {
	operations := map[string]func(*GeminiChatbot) error{
		"answer": func(bot *GeminiChatbot) error {
			_, err := bot.AnswerHomeProblemQuestion(context.Background(), conversation.ChatbotHomeProblemQuestion{UserMessage: "problem"}, nil)
			return err
		},
		"summary": func(bot *GeminiChatbot) error {
			_, err := bot.SummarizeHomeProblemConversation(context.Background(), "summary", nil)
			return err
		},
		"ranking": func(bot *GeminiChatbot) error {
			_, err := bot.RankProviders(context.Background(), conversation.ProviderRankingRequest{ProblemTitle: "problem"})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			var requests []string
			transport := &generationTransportMock{}
			for range 2 {
				transport.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					body, err := io.ReadAll(args.Get(0).(*http.Request).Body)
					require.NoError(t, err)
					requests = append(requests, string(body))
				}).Return(&http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"invalid JSON"}]}}]}`))}, nil).Once()
			}
			original := NewGeminiChatbot("model", "key")
			original.client = &http.Client{Transport: transport}
			observed, err := NewGeminiChatbotWithOptions("model", "key", GeminiOptions{HTTPClient: original.client, Observer: func(_ context.Context, trace GenerationTrace) { require.Equal(t, "invalid JSON", trace.RawText) }})
			require.NoError(t, err)
			require.Error(t, operation(original))
			require.Error(t, operation(observed))
			require.Len(t, requests, 2)
			require.JSONEq(t, requests[0], requests[1])
			transport.AssertExpectations(t)
		})
	}
}
