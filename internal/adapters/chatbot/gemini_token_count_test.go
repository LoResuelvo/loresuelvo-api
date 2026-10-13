package chatbot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSummaryCountingUsesExactGenerationContents(t *testing.T) {
	transport := &generationTransportMock{}
	var generated, counted json.RawMessage
	transport.On("RoundTrip", mock.MatchedBy(func(request *http.Request) bool { return strings.Contains(request.URL.Path, ":generateContent") })).Run(func(args mock.Arguments) {
		var envelope struct {
			Contents json.RawMessage `json:"contents"`
		}
		require.NoError(t, json.NewDecoder(args.Get(0).(*http.Request).Body).Decode(&envelope))
		generated = envelope.Contents
	}).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"updated\"}"}]}}]}`))}, nil).Once()
	transport.On("RoundTrip", mock.MatchedBy(func(request *http.Request) bool { return strings.Contains(request.URL.Path, ":countTokens") })).Run(func(args mock.Arguments) {
		var envelope struct {
			Contents json.RawMessage `json:"contents"`
		}
		require.NoError(t, json.NewDecoder(args.Get(0).(*http.Request).Body).Decode(&envelope))
		counted = envelope.Contents
	}).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"totalTokens":17}`))}, nil).Once()
	bot, err := NewGeminiChatbotWithOptions("test-model", "fake-key", GeminiOptions{HTTPClient: &http.Client{Transport: transport}})
	require.NoError(t, err)
	messages := []conversation.Message{{SenderRole: conversation.SenderConsumer, Content: "new fact"}}
	_, err = bot.SummarizeHomeProblemConversation(context.Background(), "previous memory", messages)
	require.NoError(t, err)
	tokens, err := bot.CountSummaryInputTokens(context.Background(), "previous memory", messages)
	require.NoError(t, err)
	require.EqualValues(t, 17, tokens)
	require.JSONEq(t, string(generated), string(counted))
	transport.AssertExpectations(t)
}
