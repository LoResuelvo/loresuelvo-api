package chatbot

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"google.golang.org/genai"
)

// CountAnswerInputTokens asks Gemini's CountTokens endpoint for the exact
// answer prompt, including every image part. It does not generate a response.
func (chatbot *GeminiChatbot) CountAnswerInputTokens(ctx context.Context, question conversation.ChatbotHomeProblemQuestion, availableCategories []category.Category) (int64, error) {
	if chatbot == nil || chatbot.apiKey == "" {
		return 0, fmt.Errorf("chatbot API key is required for token counting")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: chatbot.apiKey, Backend: genai.BackendGeminiAPI, HTTPClient: chatbot.client})
	if err != nil {
		return 0, fmt.Errorf("creating Gemini client for token counting failed")
	}
	response, err := client.Models.CountTokens(ctx, chatbot.model, chatbot.answerContent(question, availableCategories), nil)
	if err != nil {
		return 0, fmt.Errorf("counting Gemini answer input tokens failed")
	}
	if response == nil || response.TotalTokens <= 0 {
		return 0, fmt.Errorf("Gemini token counter returned no usable total")
	}
	return int64(response.TotalTokens), nil
}

// CountRankingInputTokens asks Gemini's CountTokens endpoint for the exact
// ranking prompt. It does not generate a response.
func (chatbot *GeminiChatbot) CountRankingInputTokens(ctx context.Context, request conversation.ProviderRankingRequest) (int64, error) {
	if chatbot == nil || chatbot.apiKey == "" {
		return 0, fmt.Errorf("chatbot API key is required for token counting")
	}
	prompt, err := chatbot.providerRankingPrompt(request)
	if err != nil {
		return 0, fmt.Errorf("building ranking prompt for token counting failed")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: chatbot.apiKey, Backend: genai.BackendGeminiAPI, HTTPClient: chatbot.client})
	if err != nil {
		return 0, fmt.Errorf("creating Gemini client for token counting failed")
	}
	response, err := client.Models.CountTokens(ctx, chatbot.model, genai.Text(prompt), nil)
	if err != nil {
		return 0, fmt.Errorf("counting Gemini ranking input tokens failed")
	}
	if response == nil || response.TotalTokens <= 0 {
		return 0, fmt.Errorf("Gemini token counter returned no usable total")
	}
	return int64(response.TotalTokens), nil
}
