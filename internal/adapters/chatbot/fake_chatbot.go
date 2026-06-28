package chatbot

import (
	"context"
	"strings"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

const (
	defaultChatbotTitle   = "Pérdida de agua en la cocina"
	defaultChatbotContent = "Revisá la zona afectada y contactá a un profesional si el problema continúa."
)

// FakeChatbot is a deterministic chatbot adapter for acceptance tests.
type FakeChatbot struct {
	mu                      sync.Mutex
	response                conversation.ChatbotResponse
	summary                 string
	requestCount            int
	summaryRequestCount     int
	lastQuestion            conversation.ChatbotHomeProblemQuestion
	lastSummaryMessages     []conversation.Message
	lastPreviousSummary     string
	lastAvailableCategories []category.Category
}

func NewFakeChatbot() *FakeChatbot {
	chatbot := &FakeChatbot{}
	chatbot.Reset()
	return chatbot
}

func (chatbot *FakeChatbot) AnswerHomeProblemQuestion(ctx context.Context, question conversation.ChatbotHomeProblemQuestion, availableCategories []category.Category) (*conversation.ChatbotResponse, error) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.requestCount++
	chatbot.lastQuestion = copyQuestion(question)
	chatbot.lastAvailableCategories = availableCategories
	response := chatbot.response

	return &response, nil
}

func (chatbot *FakeChatbot) SummarizeHomeProblemConversation(ctx context.Context, previousSummary string, messages []conversation.Message) (string, error) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.summaryRequestCount++
	chatbot.lastPreviousSummary = strings.TrimSpace(previousSummary)
	chatbot.lastSummaryMessages = copyMessages(messages)
	if strings.TrimSpace(chatbot.summary) != "" {
		return chatbot.summary, nil
	}

	return fallbackSummary(previousSummary, messages), nil
}

func (chatbot *FakeChatbot) SetResponse(title, content string) {
	chatbot.SetResponseWithStatus(conversation.ChatbotResponseAnswered, title, content)
}

func (chatbot *FakeChatbot) SetSummary(summary string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.summary = strings.TrimSpace(summary)
}

func (chatbot *FakeChatbot) SetOutOfScopeResponse(title, content string) {
	chatbot.SetResponseWithStatus(conversation.ChatbotResponseOutOfScope, title, content)
}

func (chatbot *FakeChatbot) SetConcludedDiagnosisResponse(title, content, recommendedCategoryName string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   title,
		Content: content,
		Assessment: conversation.ChatbotAssessmentResponse{
			Action:              conversation.ChatbotAssessmentReplace,
			Outcome:             conversation.AssessmentProfessionalRequired,
			ProblemTitle:        title,
			ProblemDescription:  content,
			ProblemCategoryName: recommendedCategoryName,
		},
	}
}

func (chatbot *FakeChatbot) SetSelfServiceResponse(title, content, categoryName string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.response = conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   title,
		Content: content,
		Assessment: conversation.ChatbotAssessmentResponse{
			Action:              conversation.ChatbotAssessmentReplace,
			Outcome:             conversation.AssessmentSelfService,
			ProblemTitle:        title,
			ProblemDescription:  content,
			ProblemCategoryName: categoryName,
		},
	}
}

func (chatbot *FakeChatbot) SetResponseWithStatus(status conversation.ChatbotResponseStatus, title, content string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  status,
		Title:   title,
		Content: content,
		Assessment: conversation.ChatbotAssessmentResponse{
			Action:  conversation.ChatbotAssessmentReplace,
			Outcome: conversation.AssessmentCollectingInformation,
		},
	}
	if status == conversation.ChatbotResponseOutOfScope {
		chatbot.response.Assessment = conversation.ChatbotAssessmentResponse{Action: conversation.ChatbotAssessmentUnchanged}
	}
}

func (chatbot *FakeChatbot) Reset() {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   defaultChatbotTitle,
		Content: defaultChatbotContent,
		Assessment: conversation.ChatbotAssessmentResponse{
			Action:  conversation.ChatbotAssessmentReplace,
			Outcome: conversation.AssessmentCollectingInformation,
		},
	}
	chatbot.summary = ""
	chatbot.requestCount = 0
	chatbot.summaryRequestCount = 0
	chatbot.lastQuestion = conversation.ChatbotHomeProblemQuestion{}
	chatbot.lastSummaryMessages = nil
	chatbot.lastPreviousSummary = ""
	chatbot.lastAvailableCategories = nil
}

func (chatbot *FakeChatbot) RequestCount() int {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.requestCount
}

func (chatbot *FakeChatbot) SummaryRequestCount() int {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.summaryRequestCount
}

func (chatbot *FakeChatbot) LastQuestion() conversation.ChatbotHomeProblemQuestion {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return copyQuestion(chatbot.lastQuestion)
}

func (chatbot *FakeChatbot) LastPreviousSummary() string {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.lastPreviousSummary
}

func (chatbot *FakeChatbot) LastSummaryMessages() []conversation.Message {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return copyMessages(chatbot.lastSummaryMessages)
}

func (chatbot *FakeChatbot) LastAvailableCategories() []category.Category {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.lastAvailableCategories
}

func copyQuestion(question conversation.ChatbotHomeProblemQuestion) conversation.ChatbotHomeProblemQuestion {
	return conversation.ChatbotHomeProblemQuestion{
		UserMessage:       question.UserMessage,
		ContextSummary:    question.ContextSummary,
		RecentMessages:    copyMessages(question.RecentMessages),
		Images:            copyMessageImageContents(question.Images),
		IsNewConversation: question.IsNewConversation,
	}
}

func copyMessageImageContents(images []filedomain.MessageImageContent) []filedomain.MessageImageContent {
	copiedImages := make([]filedomain.MessageImageContent, len(images))
	for index, image := range images {
		copiedImages[index] = image
		if image.Data != nil {
			copiedImages[index].Data = append([]byte(nil), image.Data...)
		}
	}
	return copiedImages
}

func copyMessages(messages []conversation.Message) []conversation.Message {
	copiedMessages := make([]conversation.Message, len(messages))
	copy(copiedMessages, messages)
	return copiedMessages
}

func fallbackSummary(previousSummary string, messages []conversation.Message) string {
	var builder strings.Builder
	if summary := strings.TrimSpace(previousSummary); summary != "" {
		builder.WriteString(summary)
	}
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(message.SenderRole)
		builder.WriteString(": ")
		builder.WriteString(content)
	}

	return strings.TrimSpace(builder.String())
}
