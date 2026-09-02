package chatbot

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

const (
	defaultChatbotTitle   = "Pérdida de agua en la cocina"
	defaultChatbotContent = "Revisá la zona afectada y contactá a un profesional si el problema continúa."
)

// FakeChatbot is a deterministic chatbot adapter for acceptance tests.
type FakeChatbot struct {
	mu                          sync.Mutex
	response                    conversation.ChatbotResponse
	summary                     string
	requestCount                int
	summaryRequestCount         int
	lastQuestion                conversation.ChatbotHomeProblemQuestion
	lastSummaryMessages         []conversation.Message
	lastPreviousSummary         string
	lastAvailableCategories     []category.Category
	lastProviderRankingRequest  conversation.ProviderRankingRequest
	providerRankingRequestCount int
	providerRankingProviderIDs  []int
	providerRankingError        error
	imageDescriptions           map[string]string
	selectedImageNames          []string
	descriptionMode             string
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
	response.ImageDescriptions = make([]conversation.ChatbotImageDescription, 0, len(question.Images))
	for index, image := range question.Images {
		if chatbot.descriptionMode == "omit_after_first" && index > 0 {
			continue
		}
		ref := conversation.ChatbotImageRef(image.FileID)
		if chatbot.descriptionMode == "unknown_ref" {
			ref = conversation.ChatbotImageRef("00000000-0000-0000-0000-000000000000")
		}
		description := chatbot.imageDescriptions[image.OriginalName]
		if strings.TrimSpace(description) == "" {
			description = "Se observa evidencia visual relevante del problema en " + image.OriginalName + "."
		}
		response.ImageDescriptions = append(response.ImageDescriptions, conversation.ChatbotImageDescription{ImageRef: ref, Description: description})
	}
	if response.Assessment.Action == conversation.ChatbotAssessmentReplace {
		for _, selectedName := range chatbot.selectedImageNames {
			for _, image := range chatbotQuestionImages(question) {
				if image.OriginalName == selectedName {
					response.Assessment.SelectedImageRefs = append(response.Assessment.SelectedImageRefs, conversation.ChatbotImageRef(image.FileID))
					break
				}
			}
		}
	}

	return &response, nil
}

func chatbotQuestionImages(question conversation.ChatbotHomeProblemQuestion) []filedomain.MessageImageContent {
	result := append([]filedomain.MessageImageContent(nil), question.Images...)
	for _, message := range question.RecentMessages {
		for _, image := range message.Images {
			result = append(result, filedomain.MessageImageContent{MessageImage: image})
		}
	}
	return result
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

func (chatbot *FakeChatbot) RankProviders(ctx context.Context, request conversation.ProviderRankingRequest) (*conversation.ProviderRankingResponse, error) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.providerRankingRequestCount++
	chatbot.lastProviderRankingRequest = copyProviderRankingRequest(request)
	if chatbot.providerRankingError != nil {
		return nil, chatbot.providerRankingError
	}

	recommendations := make([]conversation.ProviderRankingRecommendation, 0, len(request.Candidates))
	if len(chatbot.providerRankingProviderIDs) == 0 {
		for _, candidate := range request.Candidates {
			recommendations = append(recommendations, conversation.ProviderRankingRecommendation{
				Reference: candidate.Reference,
				Reason:    "La evidencia disponible coincide con el problema diagnosticado.",
			})
		}
		return &conversation.ProviderRankingResponse{Recommendations: recommendations}, nil
	}

	byProviderID := make(map[int]conversation.ProviderRecommendationCandidate, len(request.Candidates))
	for _, candidate := range request.Candidates {
		byProviderID[candidate.ProviderID] = candidate
	}
	for _, providerID := range chatbot.providerRankingProviderIDs {
		candidate, exists := byProviderID[providerID]
		if !exists {
			continue
		}
		recommendations = append(recommendations, conversation.ProviderRankingRecommendation{
			Reference: candidate.Reference,
			Reason:    "La evidencia disponible respalda su experiencia para este problema.",
		})
	}
	return &conversation.ProviderRankingResponse{Recommendations: recommendations}, nil
}

func (chatbot *FakeChatbot) SetProviderRankingByProviderIDs(providerIDs ...int) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.providerRankingProviderIDs = append([]int(nil), providerIDs...)
}

func (chatbot *FakeChatbot) SetProviderRankingError(err error) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.providerRankingError = err
}

func (chatbot *FakeChatbot) LastProviderRankingRequest() conversation.ProviderRankingRequest {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	return copyProviderRankingRequest(chatbot.lastProviderRankingRequest)
}

// LastProviderRankingAIInputJSON exposes the same sanitized JSON contract that
// the Gemini adapter sends to the external ranker. It is intentionally limited
// to the fake adapter so acceptance tests can verify the adapter boundary
// without exposing wire DTOs through the domain package.
func (chatbot *FakeChatbot) LastProviderRankingAIInputJSON() ([]byte, error) {
	chatbot.mu.Lock()
	request := copyProviderRankingRequest(chatbot.lastProviderRankingRequest)
	chatbot.mu.Unlock()

	return json.Marshal(providerRankingRequestPayloadFromDomain(request))
}

func (chatbot *FakeChatbot) SetResponse(title, content string) {
	chatbot.SetResponseWithStatus(conversation.ChatbotResponseAnswered, title, content)
}

func (chatbot *FakeChatbot) SetSummary(summary string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.summary = strings.TrimSpace(summary)
}

func (chatbot *FakeChatbot) SetImageDescription(originalName, description string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.imageDescriptions[originalName] = strings.TrimSpace(description)
}

func (chatbot *FakeChatbot) SetSelectedImageNames(names ...string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.selectedImageNames = append([]string(nil), names...)
}

func (chatbot *FakeChatbot) SetImageDescriptionMode(mode string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()
	chatbot.descriptionMode = mode
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

func (chatbot *FakeChatbot) SetUnchangedResponse(title, content string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   title,
		Content: content,
		Assessment: conversation.ChatbotAssessmentResponse{
			Action: conversation.ChatbotAssessmentUnchanged,
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
	chatbot.lastProviderRankingRequest = conversation.ProviderRankingRequest{}
	chatbot.providerRankingRequestCount = 0
	chatbot.providerRankingProviderIDs = nil
	chatbot.providerRankingError = nil
	chatbot.imageDescriptions = map[string]string{}
	chatbot.selectedImageNames = nil
	chatbot.descriptionMode = ""
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

func (chatbot *FakeChatbot) ProviderRankingRequestCount() int {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.providerRankingRequestCount
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

func copyProviderRankingRequest(request conversation.ProviderRankingRequest) conversation.ProviderRankingRequest {
	copied := request
	copied.Candidates = make([]conversation.ProviderRecommendationCandidate, len(request.Candidates))
	copy(copied.Candidates, request.Candidates)
	for index := range copied.Candidates {
		copied.Candidates[index].Evidence.WorkHistory = append([]providerreadmodel.WorkOrder(nil), request.Candidates[index].Evidence.WorkHistory...)
	}
	return copied
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
		if content == "" && len(message.Images) == 0 {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(message.SenderRole)
		builder.WriteString(": ")
		builder.WriteString(content)
		for _, image := range message.Images {
			builder.WriteString(" [")
			builder.WriteString(conversation.ChatbotImageRef(image.FileID))
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(image.Description))
			builder.WriteString("]")
		}
	}

	return strings.TrimSpace(builder.String())
}
