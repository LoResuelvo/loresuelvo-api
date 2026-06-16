package conversation

import (
	"context"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

type Chatbot interface {
	AnswerHomeProblemQuestion(ctx context.Context, question string, availableCategories []category.Category) (*ChatbotResponse, error)
}

type ChatbotResponseStatus string

const (
	ChatbotResponseAnswered   ChatbotResponseStatus = "answered"
	ChatbotResponseOutOfScope ChatbotResponseStatus = "out_of_scope"
)

type ChatbotResponse struct {
	Status                  ChatbotResponseStatus
	Title                   string
	Content                 string
	DiagnosisCompleted      bool
	RecommendedCategoryName string
}

func ParseChatbotResponseStatus(value string) (ChatbotResponseStatus, error) {
	switch ChatbotResponseStatus(strings.ToLower(strings.TrimSpace(value))) {
	case ChatbotResponseAnswered:
		return ChatbotResponseAnswered, nil
	case ChatbotResponseOutOfScope:
		return ChatbotResponseOutOfScope, nil
	default:
		return "", ErrChatbotResponseRequired
	}
}
