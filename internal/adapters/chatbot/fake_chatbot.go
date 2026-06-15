package chatbot

import (
	"context"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

const (
	defaultChatbotTitle   = "Pérdida de agua en la cocina"
	defaultChatbotContent = "Revisá la zona afectada y contactá a un profesional si el problema continúa."
)

// FakeChatbot is a deterministic chatbot adapter for acceptance tests.
type FakeChatbot struct {
	mu           sync.Mutex
	response     conversation.ChatbotResponse
	requestCount int
	lastQuestion string
}

func NewFakeChatbot() *FakeChatbot {
	chatbot := &FakeChatbot{}
	chatbot.Reset()
	return chatbot
}

func (chatbot *FakeChatbot) AnswerHomeProblemQuestion(ctx context.Context, question string) (*conversation.ChatbotResponse, error) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.requestCount++
	chatbot.lastQuestion = question
	response := chatbot.response

	return &response, nil
}

func (chatbot *FakeChatbot) SetResponse(title, content string) {
	chatbot.SetResponseWithStatus(conversation.ChatbotResponseAnswered, title, content)
}

func (chatbot *FakeChatbot) SetOutOfScopeResponse(title, content string) {
	chatbot.SetResponseWithStatus(conversation.ChatbotResponseOutOfScope, title, content)
}

func (chatbot *FakeChatbot) SetResponseWithStatus(status conversation.ChatbotResponseStatus, title, content string) {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  status,
		Title:   title,
		Content: content,
	}
}

func (chatbot *FakeChatbot) Reset() {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	chatbot.response = conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   defaultChatbotTitle,
		Content: defaultChatbotContent,
	}
	chatbot.requestCount = 0
	chatbot.lastQuestion = ""
}

func (chatbot *FakeChatbot) RequestCount() int {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.requestCount
}

func (chatbot *FakeChatbot) LastQuestion() string {
	chatbot.mu.Lock()
	defer chatbot.mu.Unlock()

	return chatbot.lastQuestion
}
