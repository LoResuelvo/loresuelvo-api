package chatbot

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
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
