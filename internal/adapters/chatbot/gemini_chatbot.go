package chatbot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"google.golang.org/genai"
)

const defaultGeminiModel = "gemini-2.5-flash"

type GeminiChatbot struct {
	apiKey string
	model  string
}

func NewGeminiChatbot(model, apiKey string) *GeminiChatbot {
	return &GeminiChatbot{
		apiKey: strings.TrimSpace(apiKey),
		model:  model,
	}
}

func (chatbot *GeminiChatbot) AnswerHomeProblemQuestion(ctx context.Context, prompt string, availableCategories []category.Category) (*conversation.ChatbotResponse, error) {
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

	result, err := client.Models.GenerateContent(ctx, chatbot.model, genai.Text(chatbot.prompt(prompt, availableCategories)), nil)
	if err != nil {
		return nil, fmt.Errorf("generating chatbot response: %w", err)
	}

	return parseChatbotResponse(prompt, result.Text())
}

func (chatbot *GeminiChatbot) prompt(userPrompt string, availableCategories []category.Category) string {
	return fmt.Sprintf(`Sos un asistente de pre diagnóstico para problemas del hogar en Argentina.
Respondé únicamente si la consulta se relaciona con problemas del hogar como plomería, electricidad, gas, humedad, cerraduras, calefacción o reparaciones.
Si la consulta no se relaciona con problemas del hogar, no respondas la consulta; devolvé una negativa breve y prudente.
Devolvé exclusivamente JSON válido con este formato:
{"status":"answered|out_of_scope","title":"título breve de la conversación","content":"orientación preliminar clara y prudente o negativa breve","diagnosis_completed":true|false,"recommended_category_name":"nombre del rubro recomendado o vacío"}
Usá diagnosis_completed=true solo cuando tengas suficiente información para concluir un pre diagnóstico y recomendar un rubro adecuado.
Si diagnosis_completed=true, recommended_category_name debe ser exactamente uno de los rubros disponibles listados abajo.
Si ningún rubro disponible corresponde al problema, si faltan datos o si la consulta está fuera de alcance, usá diagnosis_completed=false y recommended_category_name vacío.

Rubros disponibles:
%s
No diagnostiques emergencias médicas. Si hay riesgo eléctrico, gas o inundación, recomendá cortar el suministro y contactar a un profesional.

Consulta del consumidor:
%s`, availableCategoryListForPrompt(availableCategories), userPrompt)
}

func parseChatbotResponse(_, rawResponse string) (*conversation.ChatbotResponse, error) {
	var payload struct {
		Status                  string `json:"status"`
		Title                   string `json:"title"`
		Content                 string `json:"content"`
		DiagnosisCompleted      bool   `json:"diagnosis_completed"`
		RecommendedCategoryName string `json:"recommended_category_name"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawResponse)), &payload); err != nil {
		return nil, fmt.Errorf("parsing chatbot response: %w", err)
	}

	payload.Title = strings.TrimSpace(payload.Title)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.RecommendedCategoryName = strings.TrimSpace(payload.RecommendedCategoryName)
	status, err := conversation.ParseChatbotResponseStatus(payload.Status)
	if err != nil {
		return nil, err
	}

	if payload.Content == "" || payload.Title == "" {
		return nil, conversation.ErrChatbotResponseRequired
	}

	return &conversation.ChatbotResponse{
		Status:                  status,
		Title:                   payload.Title,
		Content:                 payload.Content,
		DiagnosisCompleted:      payload.DiagnosisCompleted,
		RecommendedCategoryName: payload.RecommendedCategoryName,
	}, nil
}

func availableCategoryListForPrompt(availableCategories []category.Category) string {
	if len(availableCategories) == 0 {
		return "- No hay rubros disponibles"
	}

	var builder strings.Builder
	for _, category := range availableCategories {
		name := strings.TrimSpace(category.Name)
		if name == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}

	listedCategories := strings.TrimSpace(builder.String())
	if listedCategories == "" {
		return "- No hay rubros disponibles"
	}

	return listedCategories
}
