package handler

import (
	"encoding/json"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatbotConversationResponseReusesChatbotDetailPayload(t *testing.T) {
	recommendedCategory := &category.Category{ID: 3, Name: "Plomería", NormalizedName: "plomería"}
	chatbotConversation := &conversation.ChatBotConversation{
		BaseConversation: &conversation.BaseConversation{ID: 7, Status: conversation.StatusActive},
		Title:            "Pérdida de agua en la cocina",
	}
	result := conversation.ChatbotConversationResult{
		Conversation:        chatbotConversation,
		ResponseStatus:      conversation.ChatbotResponseAnswered,
		DiagnosisCompleted:  true,
		RecommendedCategory: recommendedCategory,
		RecommendedProviders: []providerreadmodel.ProviderSummary{{
			ID:           20,
			Name:         "Juan",
			Surname:      "Gómez",
			CategoryName: "Plomería",
		}},
	}

	response := chatbotConversationResponseFromDomain(result)

	assert.Equal(t, "Pérdida de agua en la cocina", response.Title)
	assert.Equal(t, string(conversation.ChatbotResponseAnswered), response.ResponseStatus)
	assert.True(t, response.DiagnosisCompleted)
	require.NotNil(t, response.RecommendedCategory)
	assert.Equal(t, 3, response.RecommendedCategory.ID)
	assert.Equal(t, "Plomería", response.RecommendedCategory.Name)
	require.Len(t, response.RecommendedProviders, 1)

	body, err := json.Marshal(response)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	assert.Contains(t, fields, "recommended_category")
	assert.NotContains(t, fields, "recommended_category_name")
}
