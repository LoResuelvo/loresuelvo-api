package conversation

import (
	"context"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Chatbot interface {
	AnswerHomeProblemQuestion(ctx context.Context, question ChatbotHomeProblemQuestion, availableCategories []category.Category) (*ChatbotResponse, error)
	SummarizeHomeProblemConversation(ctx context.Context, previousSummary string, messages []Message) (string, error)
	RankProviders(ctx context.Context, request ProviderRankingRequest) (*ProviderRankingResponse, error)
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
	Status            ChatbotResponseStatus
	Title             string
	Content           string
	ImageDescriptions []ChatbotImageDescription
	Assessment        ChatbotAssessmentResponse
}

type ChatbotImageDescription struct {
	ImageRef    string
	Description string
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
	SelectedImageRefs   []string
}

func ChatbotImageRef(fileID string) string {
	return "image:" + strings.TrimSpace(fileID)
}

type chatbotAnswer struct {
	response              *ChatbotResponse
	message               *Message
	problemCategory       *category.Category
	recommendedProviders  []provider.Provider
	recommendationReasons map[int]string
	currentRecommendation *CurrentProviderRecommendation
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
