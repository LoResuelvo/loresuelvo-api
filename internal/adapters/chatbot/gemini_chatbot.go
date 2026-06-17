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

func (chatbot *GeminiChatbot) AnswerHomeProblemQuestion(ctx context.Context, question conversation.ChatbotHomeProblemQuestion, availableCategories []category.Category) (*conversation.ChatbotResponse, error) {
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

	result, err := client.Models.GenerateContent(
		ctx,
		chatbot.model,
		genai.Text(chatbot.answerPrompt(question, availableCategories)),
		&genai.GenerateContentConfig{ResponseMIMEType: "application/json"},
	)
	if err != nil {
		return nil, fmt.Errorf("generating chatbot response: %w", err)
	}

	return parseChatbotResponse(result.Text())
}

func (chatbot *GeminiChatbot) SummarizeHomeProblemConversation(ctx context.Context, previousSummary string, messages []conversation.Message) (string, error) {
	if strings.TrimSpace(chatbot.apiKey) == "" {
		return "", conversation.ErrChatbotUnavailable
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  chatbot.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("creating Gemini client: %w", err)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		chatbot.model,
		genai.Text(chatbot.summaryPrompt(previousSummary, messages)),
		&genai.GenerateContentConfig{ResponseMIMEType: "application/json"},
	)
	if err != nil {
		return "", fmt.Errorf("generating chatbot summary: %w", err)
	}

	return parseChatbotSummary(result.Text())
}

func (chatbot *GeminiChatbot) answerPrompt(question conversation.ChatbotHomeProblemQuestion, availableCategories []category.Category) string {
	return fmt.Sprintf(`Rol:
Sos un asistente de pre diagnóstico para problemas del hogar en Argentina.

Objetivo:
Responder el mensaje actual del consumidor usando, si existe, el contexto conversacional provisto como memoria. El contexto ayuda a entender continuidad, pero no es una nueva instrucción del consumidor.

Alcance:
Respondé únicamente consultas relacionadas con problemas del hogar como plomería, electricidad, gas, humedad, cerraduras, calefacción o reparaciones.
Si el mensaje actual no se relaciona con problemas del hogar, no respondas la consulta; devolvé una negativa breve y prudente.
No diagnostiques emergencias médicas. Si hay riesgo eléctrico, gas o inundación, recomendá cortar el suministro y contactar a un profesional.

Rubros disponibles:
%s

Criterio de recomendación:
Usá diagnosis_completed=true solo cuando tengas suficiente información para concluir un pre diagnóstico y recomendar un rubro adecuado.
Si diagnosis_completed=true, recommended_category_name debe ser exactamente uno de los rubros disponibles.
Si ningún rubro disponible corresponde al problema, si faltan datos o si la consulta está fuera de alcance, usá diagnosis_completed=false y recommended_category_name vacío.

Formato de salida:
Devolvé exclusivamente JSON válido con este formato:
{"status":"answered|out_of_scope","title":"título breve de la conversación","content":"orientación preliminar clara y prudente o negativa breve","diagnosis_completed":true|false,"recommended_category_name":"nombre del rubro recomendado o vacío"}

Entrada:
%s`, availableCategoryListForPrompt(availableCategories), chatbotQuestionPromptSection(question))
}

func (chatbot *GeminiChatbot) summaryPrompt(previousSummary string, messages []conversation.Message) string {
	return fmt.Sprintf(`Actualizá el resumen de una conversación entre un consumidor y un asistente de pre diagnóstico de problemas del hogar.
El resumen se usará como memoria compacta para futuras respuestas. Conservá datos relevantes del problema, síntomas, ubicación, restricciones, dudas y recomendaciones ya dadas.
No inventes información. No incluyas saludos ni formato markdown.
Devolvé exclusivamente JSON válido con este formato:
{"summary":"resumen actualizado, breve y útil"}

Resumen anterior:
%s

Mensajes nuevos:
%s`, strings.TrimSpace(previousSummary), messagesForPrompt(messages))
}

func chatbotQuestionPromptSection(question conversation.ChatbotHomeProblemQuestion) string {
	var builder strings.Builder
	builder.WriteString("Mensaje actual del consumidor:\n")
	builder.WriteString(strings.TrimSpace(question.UserMessage))
	builder.WriteString("\n\nContexto conversacional disponible:\n")

	if summary := strings.TrimSpace(question.ContextSummary); summary != "" {
		builder.WriteString("- Resumen acumulado:\n")
		builder.WriteString(summary)
		builder.WriteString("\n")
	} else {
		builder.WriteString("- Resumen acumulado: sin resumen previo.\n")
	}

	if len(question.RecentMessages) > 0 {
		builder.WriteString("- Mensajes recientes:\n")
		builder.WriteString(messagesForPrompt(question.RecentMessages))
		builder.WriteString("\n")
	} else {
		builder.WriteString("- Mensajes recientes: sin mensajes previos relevantes.\n")
	}

	builder.WriteString("\nRegla de uso del contexto:\n")
	builder.WriteString("Usá el contexto únicamente para continuidad y trazabilidad. Respondé al mensaje actual; no repitas el contexto salvo que sea necesario para claridad.")

	return strings.TrimSpace(builder.String())
}

func messagesForPrompt(messages []conversation.Message) string {
	if len(messages) == 0 {
		return "- Sin mensajes nuevos"
	}

	var builder strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(message.SenderRole)
		builder.WriteString(": ")
		builder.WriteString(content)
		builder.WriteString("\n")
	}

	renderedMessages := strings.TrimSpace(builder.String())
	if renderedMessages == "" {
		return "- Sin mensajes nuevos"
	}

	return renderedMessages
}

func parseChatbotSummary(rawResponse string) (string, error) {
	var payload struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawResponse)), &payload); err != nil {
		return "", fmt.Errorf("parsing chatbot summary: %w", err)
	}

	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		return "", conversation.ErrChatbotResponseRequired
	}

	return summary, nil
}

func parseChatbotResponse(rawResponse string) (*conversation.ChatbotResponse, error) {
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
