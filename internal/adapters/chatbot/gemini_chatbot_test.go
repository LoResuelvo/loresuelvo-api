package chatbot

import (
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChatbotResponseParsesProfessionalAssessment(t *testing.T) {
	response, err := parseChatbotResponse(`{
		"status":"answered",
		"title":"Pérdida debajo de la pileta",
		"content":"El problema requiere un plomero.",
		"assessment":{
			"action":"replace",
			"outcome":"professional_required",
			"problem_title":"Pérdida en el sifón",
			"problem_description":"La pérdida continúa después de ajustar la conexión.",
			"problem_category_name":"Plomería"
		}
	}`, true)

	require.NoError(t, err)
	assert.Equal(t, conversation.ChatbotAssessmentReplace, response.Assessment.Action)
	assert.Equal(t, conversation.AssessmentProfessionalRequired, response.Assessment.Outcome)
	assert.Equal(t, "Plomería", response.Assessment.ProblemCategoryName)
}

func TestParseChatbotResponseAllowsUnchangedOutOfScopeAssessment(t *testing.T) {
	response, err := parseChatbotResponse(`{
		"status":"out_of_scope",
		"title":"",
		"content":"Solo puedo ayudarte con problemas del hogar.",
		"assessment":{
			"action":"unchanged",
			"outcome":"",
			"problem_title":"",
			"problem_description":"",
			"problem_category_name":""
		}
	}`, false)

	require.NoError(t, err)
	assert.Equal(t, conversation.ChatbotResponseOutOfScope, response.Status)
	assert.Equal(t, conversation.ChatbotAssessmentUnchanged, response.Assessment.Action)
}

func TestParseChatbotResponseRejectsOutOfScopeAssessmentReplacement(t *testing.T) {
	response, err := parseChatbotResponse(`{
		"status":"out_of_scope",
		"title":"",
		"content":"Solo puedo ayudarte con problemas del hogar.",
		"assessment":{
			"action":"replace",
			"outcome":"collecting_information",
			"problem_title":"",
			"problem_description":"",
			"problem_category_name":""
		}
	}`, false)

	assert.ErrorIs(t, err, conversation.ErrProblemAssessmentInvalid)
	assert.Nil(t, response)
}

func TestAnswerPromptRequiresStructuredProfessionalDiagnosis(t *testing.T) {
	prompt := (&GeminiChatbot{}).answerPrompt(
		conversation.ChatbotHomeProblemQuestion{UserMessage: "Pierde agua debajo de la pileta."},
		[]category.Category{{Name: "Plomería"}},
	)

	for _, section := range []string{
		"Situación observada:",
		"Evidencia disponible:",
		"Diagnóstico preliminar:",
		"Posibles causas:",
		"Urgencia y riesgos:",
		"Recomendaciones para la visita:",
	} {
		assert.Contains(t, prompt, section)
	}
	assert.Contains(t, prompt, "Separá hechos observados de hipótesis")
	assert.Contains(t, prompt, "nunca presentes una causa como confirmada")
}

func TestAnswerPromptBoundsInformationCollection(t *testing.T) {
	prompt := (&GeminiChatbot{}).answerPrompt(
		conversation.ChatbotHomeProblemQuestion{UserMessage: "Tengo un problema eléctrico."},
		nil,
	)

	assert.GreaterOrEqual(t, strings.Count(prompt, "como máximo 2 preguntas"), 2)
	assert.Contains(t, prompt, "cuya respuesta pueda cambiar materialmente")
	assert.Contains(t, prompt, "no repitas preguntas ya respondidas")
	assert.Contains(t, prompt, "avanzá declarando la incertidumbre restante")
}

func TestAnswerPromptRequiresActionableSelfServiceGuide(t *testing.T) {
	prompt := (&GeminiChatbot{}).answerPrompt(
		conversation.ChatbotHomeProblemQuestion{UserMessage: "La canilla tiene el aireador tapado."},
		nil,
	)

	for _, section := range []string{
		"Qué parece estar ocurriendo:",
		"Antes de empezar:",
		"Pasos:",
		"Cómo comprobarlo:",
		"Detenete y contactá a un profesional si:",
	} {
		assert.Contains(t, prompt, section)
	}
	assert.Contains(t, prompt, "herramientas especiales")
	assert.Contains(t, prompt, "el resultado no debe ser self_service")
}

func TestProviderRankingPromptMapsDomainDataToGeminiWireContract(t *testing.T) {
	recentWork := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	prompt, err := (&GeminiChatbot{}).providerRankingPrompt(conversation.ProviderRankingRequest{
		ProblemTitle:       "Pérdida debajo de la pileta",
		ProblemDescription: "La conexión pierde agua.",
		MaxResults:         3,
		Candidates: []conversation.ProviderRecommendationCandidate{{
			Reference:  "candidate-secret",
			ProviderID: 42,
			Evidence: conversation.ProviderRecommendationEvidence{
				RatingAverage:      4.5,
				RatingCount:        2,
				RatingDistribution: provider.RatingDistribution{0, 0, 0, 1, 1},
				PaidWorkCount:      2,
				MostRecentPaidWork: recentWork,
			},
		}},
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, `"problem_title":"Pérdida debajo de la pileta"`)
	assert.Contains(t, prompt, `"reference":"candidate-secret"`)
	assert.Contains(t, prompt, `"rating_average":4.5`)
	assert.Contains(t, prompt, `"most_recent_paid_work":"2026-08-20T12:00:00Z"`)
	assert.NotContains(t, prompt, `"provider_id"`)
	assert.NotContains(t, prompt, `"ProviderID"`)
}

func TestParseProviderRankingResponseMapsGeminiWireContract(t *testing.T) {
	response, err := parseProviderRankingResponse(`{"recommendations":[{"reference":" candidate-1 ","reason":" experiencia comprobable "}]}`)

	require.NoError(t, err)
	require.Len(t, response.Recommendations, 1)
	assert.Equal(t, "candidate-1", response.Recommendations[0].Reference)
	assert.Equal(t, "experiencia comprobable", response.Recommendations[0].Reason)
}
