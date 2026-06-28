package conversation

import (
	"context"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type Chatbot interface {
	AnswerHomeProblemQuestion(ctx context.Context, question ChatbotHomeProblemQuestion, availableCategories []category.Category) (*ChatbotResponse, error)
	SummarizeHomeProblemConversation(ctx context.Context, previousSummary string, messages []Message) (string, error)
}

type ChatbotHomeProblemQuestion struct {
	UserMessage       string
	ContextSummary    string
	RecentMessages    []Message
	Images            []filedomain.MessageImageContent
	IsNewConversation bool
}

type ChatbotResponseStatus string

const (
	ChatbotResponseAnswered   ChatbotResponseStatus = "answered"
	ChatbotResponseOutOfScope ChatbotResponseStatus = "out_of_scope"
)

type ChatbotResponse struct {
	Status     ChatbotResponseStatus
	Title      string
	Content    string
	Assessment ChatbotAssessmentResponse
}

type ChatbotAssessmentAction string

const (
	ChatbotAssessmentUnchanged ChatbotAssessmentAction = "unchanged"
	ChatbotAssessmentReplace   ChatbotAssessmentAction = "replace"
)

type ChatbotAssessmentResponse struct {
	Action              ChatbotAssessmentAction
	Outcome             ProblemAssessmentOutcome
	ProblemTitle        string
	ProblemDescription  string
	ProblemCategoryName string
}

type chatbotAnswer struct {
	response             *ChatbotResponse
	message              *Message
	problemCategory      *category.Category
	recommendedProviders []providerreadmodel.ProviderSummary
}

func ParseChatbotAssessmentAction(value string) (ChatbotAssessmentAction, error) {
	switch action := ChatbotAssessmentAction(strings.ToLower(strings.TrimSpace(value))); action {
	case ChatbotAssessmentUnchanged, ChatbotAssessmentReplace:
		return action, nil
	default:
		return "", ErrChatbotResponseRequired
	}
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
