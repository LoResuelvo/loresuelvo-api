package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"google.golang.org/genai"
)

const defaultGeminiModel = "gemini-2.5-flash"

type GeminiChatbot struct {
	apiKey string
	model  string
}

func NewGeminiChatbotFromEnv() *GeminiChatbot {
	return &GeminiChatbot{
		apiKey: strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		model:  envOrDefault("GEMINI_MODEL", defaultGeminiModel),
	}
}

func (chatbot *GeminiChatbot) GetResponse(ctx context.Context, prompt string) (*conversation.ChatbotResponse, error) {
	if strings.TrimSpace(chatbot.apiKey) == "" {
		return nil, conversation.ErrChatbotUnavailable
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  chatbot.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	result, err := client.Models.GenerateContent(ctx, chatbot.model, genai.Text(chatbot.prompt(prompt)), nil)
	if err != nil {
		return nil, fmt.Errorf("generating chatbot response: %w", err)
	}

	return parseChatbotResponse(prompt, result.Text())
}

func (chatbot *GeminiChatbot) prompt(userPrompt string) string {
	return fmt.Sprintf(`Sos un asistente de pre diagnóstico para problemas del hogar en Argentina.
Respondé únicamente si la consulta se relaciona con problemas del hogar como plomería, electricidad, gas, humedad, cerraduras, calefacción o reparaciones.
Devolvé exclusivamente JSON válido con este formato:
{"title":"título breve de la conversación","content":"orientación preliminar clara y prudente"}
No diagnostiques emergencias médicas. Si hay riesgo eléctrico, gas o inundación, recomendá cortar el suministro y contactar a un profesional.

Consulta del consumidor:
%s`, userPrompt)
}

func parseChatbotResponse(_, rawResponse string) (*conversation.ChatbotResponse, error) {
	var payload struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawResponse)), &payload); err != nil {
		return nil, fmt.Errorf("parsing chatbot response: %w", err)
	}

	payload.Title = strings.TrimSpace(payload.Title)
	payload.Content = strings.TrimSpace(payload.Content)

	if payload.Content == "" || payload.Title == "" {
		return nil, conversation.ErrChatbotResponseRequired
	}

	return &conversation.ChatbotResponse{
		Title:   payload.Title,
		Content: payload.Content,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
