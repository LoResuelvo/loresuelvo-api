package conversation

import (
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

const MaxProblemAssessmentImages = 3

type ProblemAssessmentOutcome string

const (
	AssessmentCollectingInformation ProblemAssessmentOutcome = "collecting_information"
	AssessmentSelfService           ProblemAssessmentOutcome = "self_service"
	AssessmentProfessionalRequired  ProblemAssessmentOutcome = "professional_required"
)

type ProblemAssessment struct {
	ID                    int
	ChatbotConversationID int
	Version               int
	Outcome               ProblemAssessmentOutcome
	ProblemCategoryID     *int
	ProblemTitle          string
	ProblemDescription    string
	BasedOnMessageID      int
	Images                []filedomain.MessageImage
}

func NewProblemAssessment(
	chatbotConversationID int,
	version int,
	outcome ProblemAssessmentOutcome,
	problemCategoryID *int,
	title string,
	description string,
	images ...filedomain.MessageImage,
) (*ProblemAssessment, error) {
	assessment := &ProblemAssessment{
		ChatbotConversationID: chatbotConversationID,
		Version:               version,
		Outcome:               outcome,
		ProblemCategoryID:     copyOptionalInt(problemCategoryID),
		ProblemTitle:          strings.TrimSpace(title),
		ProblemDescription:    strings.TrimSpace(description),
		Images:                append([]filedomain.MessageImage(nil), images...),
	}
	if err := assessment.Validate(); err != nil {
		return nil, err
	}
	return assessment, nil
}

func (assessment ProblemAssessment) Validate() error {
	if assessment.Version <= 0 || assessment.BasedOnMessageID < 0 {
		return ErrProblemAssessmentInvalid
	}
	if assessment.ProblemCategoryID != nil && *assessment.ProblemCategoryID <= 0 {
		return ErrProblemAssessmentInvalid
	}
	if len(assessment.Images) > MaxProblemAssessmentImages {
		return ErrProblemAssessmentInvalid
	}
	seenImages := make(map[string]struct{}, len(assessment.Images))
	for index := range assessment.Images {
		assessment.Images[index].FileID = strings.TrimSpace(assessment.Images[index].FileID)
		assessment.Images[index].Description = strings.TrimSpace(assessment.Images[index].Description)
		if assessment.Images[index].FileID == "" || assessment.Images[index].Description == "" {
			return ErrProblemAssessmentInvalid
		}
		if _, exists := seenImages[assessment.Images[index].FileID]; exists {
			return ErrProblemAssessmentInvalid
		}
		seenImages[assessment.Images[index].FileID] = struct{}{}
	}

	switch assessment.Outcome {
	case AssessmentCollectingInformation:
		if assessment.ProblemCategoryID != nil || assessment.ProblemTitle != "" || assessment.ProblemDescription != "" {
			return ErrProblemAssessmentInvalid
		}
	case AssessmentSelfService:
		if assessment.ProblemTitle == "" || assessment.ProblemDescription == "" {
			return ErrProblemAssessmentInvalid
		}
	case AssessmentProfessionalRequired:
		if assessment.ProblemCategoryID == nil || assessment.ProblemTitle == "" || assessment.ProblemDescription == "" {
			return ErrProblemAssessmentInvalid
		}
	default:
		return ErrProblemAssessmentInvalid
	}

	return nil
}

func (assessment ProblemAssessment) RequiresProfessional() bool {
	return assessment.Outcome == AssessmentProfessionalRequired
}

func (assessment ProblemAssessment) IsComplete() bool {
	return assessment.Outcome == AssessmentSelfService || assessment.Outcome == AssessmentProfessionalRequired
}

func ParseProblemAssessmentOutcome(value string) (ProblemAssessmentOutcome, error) {
	switch outcome := ProblemAssessmentOutcome(strings.ToLower(strings.TrimSpace(value))); outcome {
	case AssessmentCollectingInformation, AssessmentSelfService, AssessmentProfessionalRequired:
		return outcome, nil
	default:
		return "", ErrProblemAssessmentInvalid
	}
}
