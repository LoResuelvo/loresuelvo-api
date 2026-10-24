package chatbot

import (
	"encoding/json"
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
		"image_descriptions":[],
		"assessment":{
			"action":"replace",
			"outcome":"professional_required",
			"problem_title":"Pérdida en el sifón",
			"problem_description":"La pérdida continúa después de ajustar la conexión.",
			"problem_category_name":"Plomería",
			"selected_image_refs":[]
		}
	}`, true, 0, []category.Category{{Name: "Plomería"}})

	require.NoError(t, err)
	assert.Equal(t, conversation.ChatbotAssessmentReplace, response.Assessment.Action)
	assert.Equal(t, conversation.AssessmentProfessionalRequired, response.Assessment.Outcome)
	assert.Equal(t, "Plomería", response.Assessment.ProblemCategoryName)
}

func TestParseChatbotResponsePreservesQualifiedProfessionalHypothesis(t *testing.T) {
	description := "Situación observada: no enciende. Evidencia disponible: relato del consumidor. Diagnóstico preliminar: falla no determinada. Posibles causas: el profesional podría verificar un componente interno, sin asumir que esté averiado. Urgencia y riesgos: no informados. Recomendaciones para la visita: inspección profesional."
	raw, err := json.Marshal(map[string]any{
		"status":             "answered",
		"title":              "Artefacto que no enciende",
		"content":            "Conviene que lo revise un profesional.",
		"image_descriptions": []any{},
		"assessment": map[string]any{
			"action":                "replace",
			"outcome":               "professional_required",
			"problem_title":         "Artefacto que no enciende",
			"problem_description":   description,
			"problem_category_name": "Electricidad",
			"selected_image_refs":   []any{},
		},
	})
	require.NoError(t, err)

	response, err := parseChatbotResponse(string(raw), true, 0, []category.Category{{Name: "Electricidad"}})

	require.NoError(t, err)
	assert.Equal(t, description, response.Assessment.ProblemDescription)
}

func TestParseChatbotResponseAllowsUnchangedOutOfScopeAssessment(t *testing.T) {
	response, err := parseChatbotResponse(`{
		"status":"out_of_scope",
		"title":"",
		"content":"Solo puedo ayudarte con problemas del hogar.",
		"image_descriptions":[],
		"assessment":{
			"action":"unchanged",
			"outcome":"",
			"problem_title":"",
			"problem_description":"",
			"problem_category_name":"",
			"selected_image_refs":[]
		}
	}`, false, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, conversation.ChatbotResponseOutOfScope, response.Status)
	assert.Equal(t, conversation.ChatbotAssessmentUnchanged, response.Assessment.Action)
}

func TestParseChatbotResponseRejectsOutOfScopeAssessmentReplacement(t *testing.T) {
	response, err := parseChatbotResponse(`{
		"status":"out_of_scope",
		"title":"",
		"content":"Solo puedo ayudarte con problemas del hogar.",
		"image_descriptions":[],
		"assessment":{
			"action":"replace",
			"outcome":"collecting_information",
			"problem_title":"",
			"problem_description":"",
			"problem_category_name":"",
			"selected_image_refs":[]
		}
	}`, false, 0, nil)

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

	assert.Contains(t, prompt, "como máximo 2 preguntas")
	assert.Contains(t, prompt, "cuya respuesta pueda cambiar materialmente")
	assert.Contains(t, prompt, "no repitas preguntas ya respondidas")
	assert.Contains(t, prompt, "avanzá declarando la incertidumbre restante")
}

func TestAnswerPromptUsesOnlyDomainOutcomesInItsOutcomeInstructions(t *testing.T) {
	prompt := (&GeminiChatbot{}).answerPrompt(conversation.ChatbotHomeProblemQuestion{}, nil)
	_, section, found := strings.Cut(prompt, "Resultados de evaluación:\n")
	require.True(t, found)
	section, _, found = strings.Cut(section, "\n\nPuerta de suficiencia:")
	require.True(t, found)
	var outcomes []string
	for line := range strings.SplitSeq(section, "\n") {
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		outcome, _, found := strings.Cut(strings.TrimPrefix(line, "- "), ":")
		require.True(t, found)
		outcomes = append(outcomes, outcome)
	}
	assert.ElementsMatch(t, []string{
		string(conversation.AssessmentCollectingInformation),
		string(conversation.AssessmentSelfService),
		string(conversation.AssessmentProfessionalRequired),
	}, outcomes)
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

func TestProviderRankingPromptPreservesInitialColdStartPolicy(t *testing.T) {
	prompt, err := (&GeminiChatbot{}).providerRankingPrompt(conversation.ProviderRankingRequest{
		ProblemTitle: "Problema",
		MaxResults:   3,
		Candidates: []conversation.ProviderRecommendationCandidate{
			{Reference: "candidate-a"},
			{Reference: "candidate-b"},
		},
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "Si la evidencia está vacía, el candidato sigue siendo elegible")
	assert.Contains(t, prompt, "no debe ser penalizado por una reputación inexistente")
	assert.Contains(t, prompt, "Las razones deben ser breves, específicas y basadas únicamente en la evidencia recibida")
}

func TestParseProviderRankingResponseMapsGeminiWireContract(t *testing.T) {
	response, err := parseProviderRankingResponse(`{"recommendations":[{"reference":" candidate-1 ","reason":" experiencia comprobable "}]}`, 3)

	require.NoError(t, err)
	require.Len(t, response.Recommendations, 1)
	assert.Equal(t, "candidate-1", response.Recommendations[0].Reference)
	assert.Equal(t, "experiencia comprobable", response.Recommendations[0].Reason)
}

func TestParsersRejectUnknownMissingDuplicateAndTrailingFields(t *testing.T) {
	validAnswer := `{"status":"answered","title":"","content":"Necesito un dato.","image_descriptions":[],"assessment":{"action":"replace","outcome":"collecting_information","problem_title":"","problem_description":"","problem_category_name":"","selected_image_refs":[]}}`
	parseAnswer := func(raw string) error {
		_, err := parseChatbotResponse(raw, false, 0, nil)
		return err
	}
	parseRanking := func(raw string) error {
		_, err := parseProviderRankingResponse(raw, 3)
		return err
	}

	tests := map[string]struct {
		parse func(string) error
		raw   string
	}{
		"answer unknown field":        {parse: parseAnswer, raw: strings.Replace(validAnswer, `"content":`, `"unexpected":true,"content":`, 1)},
		"answer case variant field":   {parse: parseAnswer, raw: strings.Replace(validAnswer, `"status":`, `"Status":`, 1)},
		"answer nested case variant":  {parse: parseAnswer, raw: strings.Replace(validAnswer, `"action":`, `"Action":`, 1)},
		"answer missing field":        {parse: parseAnswer, raw: strings.Replace(validAnswer, `"image_descriptions":[],`, "", 1)},
		"answer duplicate field":      {parse: parseAnswer, raw: strings.Replace(validAnswer, `"status":"answered"`, `"status":"answered","status":"out_of_scope"`, 1)},
		"answer trailing JSON":        {parse: parseAnswer, raw: validAnswer + `{}`},
		"answer null selected refs":   {parse: parseAnswer, raw: strings.Replace(validAnswer, `"selected_image_refs":[]`, `"selected_image_refs":null`, 1)},
		"answer null selected ref":    {parse: parseAnswer, raw: strings.Replace(validAnswer, `"selected_image_refs":[]`, `"selected_image_refs":[null]`, 1)},
		"summary unknown field":       {parse: func(raw string) error { _, err := parseChatbotSummary(raw); return err }, raw: `{"summary":"ok","extra":true}`},
		"summary case variant field":  {parse: func(raw string) error { _, err := parseChatbotSummary(raw); return err }, raw: `{"Summary":"ok"}`},
		"summary missing field":       {parse: func(raw string) error { _, err := parseChatbotSummary(raw); return err }, raw: `{}`},
		"ranking is array":            {parse: parseRanking, raw: `[{"reference":"candidate-a","reason":"sin evidencia"}]`},
		"ranking item missing reason": {parse: parseRanking, raw: `{"recommendations":[{"reference":"candidate-a"}]}`},
		"ranking item blank reason":   {parse: parseRanking, raw: `{"recommendations":[{"reference":"candidate-a","reason":"  "}]}`},
	}
	tests["answer duplicate selected ref"] = struct {
		parse func(string) error
		raw   string
	}{parse: parseAnswer, raw: strings.Replace(strings.Replace(strings.Replace(strings.Replace(validAnswer, `"outcome":"collecting_information"`, `"outcome":"self_service"`, 1), `"problem_title":""`, `"problem_title":"Título"`, 1), `"problem_description":""`, `"problem_description":"Descripción"`, 1), `"selected_image_refs":[]`, `"selected_image_refs":["image:a","image:a"]`, 1)}
	tests["answer too many selected refs"] = struct {
		parse func(string) error
		raw   string
	}{parse: parseAnswer, raw: strings.Replace(strings.Replace(strings.Replace(strings.Replace(validAnswer, `"outcome":"collecting_information"`, `"outcome":"self_service"`, 1), `"problem_title":""`, `"problem_title":"Título"`, 1), `"problem_description":""`, `"problem_description":"Descripción"`, 1), `"selected_image_refs":[]`, `"selected_image_refs":["image:a","image:b","image:c","image:d"]`, 1)}
	tests["answer blank image description"] = struct {
		parse func(string) error
		raw   string
	}{parse: func(raw string) error { _, err := parseChatbotResponse(raw, false, 1, nil); return err }, raw: strings.Replace(validAnswer, `"image_descriptions":[]`, `"image_descriptions":[{"image_ref":"image:a","description":" "}]`, 1)}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, test.parse(test.raw))
		})
	}
}
