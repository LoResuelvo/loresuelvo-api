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
		Conversation:    chatbotConversation,
		ResponseStatus:  conversation.ChatbotResponseAnswered,
		ProblemCategory: recommendedCategory,
		Assessment: &conversation.ProblemAssessment{
			ID: 1, Version: 1, Outcome: conversation.AssessmentProfessionalRequired,
			ProblemCategoryID: &recommendedCategory.ID, ProblemTitle: "Pérdida",
			ProblemDescription: "Pierde agua.", BasedOnMessageID: 2,
		},
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
	require.NotNil(t, response.Assessment)
	assert.Equal(t, string(conversation.AssessmentProfessionalRequired), response.Assessment.Outcome)
	require.NotNil(t, response.Assessment.ProblemCategory)
	assert.Equal(t, 3, response.Assessment.ProblemCategory.ID)
	assert.Equal(t, "Plomería", response.Assessment.ProblemCategory.Name)
	require.Len(t, response.RecommendedProviders, 1)

	body, err := json.Marshal(response)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	assert.Contains(t, fields, "assessment")
	assert.NotContains(t, fields, "diagnosis_completed")
	assert.NotContains(t, fields, "recommended_category")
}
