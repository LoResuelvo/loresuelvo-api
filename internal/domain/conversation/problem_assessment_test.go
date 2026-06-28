package conversation_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProblemAssessmentRequiresCategoryOnlyForProfessionalOutcome(t *testing.T) {
	categoryID := 3
	assessment, err := conversation.NewProblemAssessment(
		10, 1, conversation.AssessmentProfessionalRequired, &categoryID,
		"Pérdida en el sifón", "La pérdida continúa después de ajustar la conexión.",
	)

	require.NoError(t, err)
	assert.True(t, assessment.RequiresProfessional())
	assert.True(t, assessment.IsComplete())
}

func TestProblemAssessmentRejectsProfessionalOutcomeWithoutCategory(t *testing.T) {
	assessment, err := conversation.NewProblemAssessment(
		10, 1, conversation.AssessmentProfessionalRequired, nil,
		"Pérdida en el sifón", "La pérdida continúa.",
	)

	assert.ErrorIs(t, err, conversation.ErrProblemAssessmentInvalid)
	assert.Nil(t, assessment)
}

func TestProblemAssessmentAllowsClassifiedSelfServiceOutcome(t *testing.T) {
	categoryID := 3
	assessment, err := conversation.NewProblemAssessment(
		10, 1, conversation.AssessmentSelfService, &categoryID,
		"Sifón flojo", "La conexión estaba floja y dejó de perder después de ajustarla.",
	)

	require.NoError(t, err)
	assert.False(t, assessment.RequiresProfessional())
	assert.True(t, assessment.IsComplete())
	assert.Equal(t, categoryID, *assessment.ProblemCategoryID)
}

func TestCollectingAssessmentRejectsPrematureProblemDetails(t *testing.T) {
	assessment, err := conversation.NewProblemAssessment(
		10, 1, conversation.AssessmentCollectingInformation, nil,
		"Posible pérdida", "",
	)

	assert.ErrorIs(t, err, conversation.ErrProblemAssessmentInvalid)
	assert.Nil(t, assessment)
}
