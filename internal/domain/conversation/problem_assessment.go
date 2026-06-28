package conversation

import "strings"

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
}

func NewProblemAssessment(
	chatbotConversationID int,
	version int,
	outcome ProblemAssessmentOutcome,
	problemCategoryID *int,
	title string,
	description string,
) (*ProblemAssessment, error) {
	assessment := &ProblemAssessment{
		ChatbotConversationID: chatbotConversationID,
		Version:               version,
		Outcome:               outcome,
		ProblemCategoryID:     copyOptionalInt(problemCategoryID),
		ProblemTitle:          strings.TrimSpace(title),
		ProblemDescription:    strings.TrimSpace(description),
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
