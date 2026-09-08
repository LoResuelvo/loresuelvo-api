package chatbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/genai"
)

var (
	ErrInvalidGeminiOptions = errors.New("invalid Gemini options")
	errEmptyGeminiResponse  = errors.New("empty Gemini response")
)

// GeminiOptions enables bounded, observable generation without changing domain ports.
// An observer runs synchronously before parsing and must support concurrent calls
// if the chatbot is shared. It must not mutate or retain caller-owned state.
type GeminiOptions struct {
	HTTPClient      *http.Client
	MaxOutputTokens int32
	Observer        func(context.Context, GenerationTrace)
}

// GenerationTrace contains SDK inputs and output, never credentials or HTTP headers.
// Contents includes media bytes; callers must store traces with appropriate access.
// Response is SDK-decoded JSON, not a byte-for-byte HTTP response. Missing provider
// metadata remains absent. Config records requested settings, not observed defaults.
type GenerationTrace struct {
	Operation string          `json:"operation"`
	Model     string          `json:"model"`
	Contents  json.RawMessage `json:"contents"`
	Config    json.RawMessage `json:"config"`
	RawText   string          `json:"raw_text"`
	Response  json.RawMessage `json:"response"`
	Error     string          `json:"error,omitempty"`
}

// NewGeminiChatbotWithOptions preserves defaults when options are zero-valued.
func NewGeminiChatbotWithOptions(model, apiKey string, options GeminiOptions) (*GeminiChatbot, error) {
	if options.MaxOutputTokens < 0 {
		return nil, ErrInvalidGeminiOptions
	}
	chatbot := NewGeminiChatbot(model, apiKey)
	chatbot.options = options
	if options.HTTPClient != nil {
		chatbot.client = options.HTTPClient
	}
	return chatbot, nil
}

func (chatbot *GeminiChatbot) generationConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{ResponseMIMEType: "application/json", MaxOutputTokens: chatbot.options.MaxOutputTokens}
}

func (chatbot *GeminiChatbot) answerGenerationConfig() *genai.GenerateContentConfig {
	maxSelectedImages := int64(3)
	config := chatbot.generationConfig()
	config.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"status": {
				Type: genai.TypeString,
				Enum: []string{"answered", "out_of_scope"},
			},
			"title":   {Type: genai.TypeString},
			"content": {Type: genai.TypeString},
			"image_descriptions": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"image_ref":   {Type: genai.TypeString},
						"description": {Type: genai.TypeString},
					},
					Required: []string{"image_ref", "description"},
				},
			},
			"assessment": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"action": {
						Type: genai.TypeString,
						Enum: []string{"unchanged", "replace"},
					},
					"outcome": {
						Type: genai.TypeString,
						Enum: []string{"", "collecting_information", "self_service", "professional_required"},
					},
					"problem_title":         {Type: genai.TypeString},
					"problem_description":   {Type: genai.TypeString},
					"problem_category_name": {Type: genai.TypeString},
					"selected_image_refs": {
						Type:     genai.TypeArray,
						Items:    &genai.Schema{Type: genai.TypeString},
						MaxItems: &maxSelectedImages,
					},
				},
				Required: []string{"action", "outcome", "problem_title", "problem_description", "problem_category_name", "selected_image_refs"},
			},
		},
		Required: []string{"status", "title", "content", "image_descriptions", "assessment"},
	}
	return config
}

func (chatbot *GeminiChatbot) providerRankingGenerationConfig(maxResults int) *genai.GenerateContentConfig {
	config := chatbot.generationConfig()
	recommendations := &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"reference": {Type: genai.TypeString},
				"reason":    {Type: genai.TypeString},
			},
			Required: []string{"reference", "reason"},
		},
	}
	if maxResults > 0 {
		limit := int64(maxResults)
		recommendations.MaxItems = &limit
	}
	config.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"recommendations": recommendations,
		},
		Required: []string{"recommendations"},
	}
	return config
}

func (chatbot *GeminiChatbot) generateContent(ctx context.Context, client *genai.Client, operation string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	// Avoid serialization overhead entirely in the production default path.
	if chatbot.options.Observer == nil {
		return client.Models.GenerateContent(ctx, chatbot.model, contents, config)
	}
	input, err := json.Marshal(contents)
	if err != nil {
		return nil, fmt.Errorf("encoding generation input: %w", err)
	}
	requestedConfig, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encoding generation config: %w", err)
	}
	trace := GenerationTrace{Operation: operation, Model: chatbot.model, Contents: input, Config: requestedConfig}
	result, generationErr := client.Models.GenerateContent(ctx, chatbot.model, contents, config)
	if generationErr == nil && result == nil {
		generationErr = errEmptyGeminiResponse
	}
	if generationErr != nil {
		// Provider errors can contain request URLs or echoed credentials. Keep only a
		// stable status in the persisted trace; the caller receives the original error.
		trace.Error = "generation_failed"
	} else if result != nil {
		trace.RawText = result.Text()
		response := *result
		response.SDKHTTPResponse = nil
		trace.Response, err = json.Marshal(&response)
		if err != nil {
			chatbot.options.Observer(ctx, trace)
			return nil, fmt.Errorf("encoding generation response: %w", err)
		}
	}
	chatbot.options.Observer(ctx, trace)
	return result, generationErr
}
