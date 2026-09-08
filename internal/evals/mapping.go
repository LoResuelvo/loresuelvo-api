package evals

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ImageInput struct {
	AssetID      string `json:"asset_id"`
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
	MimeType     string `json:"mime_type"`
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
}
type HistoricalImage struct {
	FileID      string `json:"file_id"`
	Description string `json:"description"`
}
type MessageInput struct {
	SenderRole string            `json:"sender_role"`
	Content    string            `json:"content"`
	Images     []HistoricalImage `json:"images"`
}
type PDInput struct {
	UserMessage         string         `json:"user_message"`
	ContextSummary      string         `json:"context_summary"`
	RecentMessages      []MessageInput `json:"recent_messages"`
	Images              []ImageInput   `json:"images"`
	IsNewConversation   bool           `json:"is_new_conversation"`
	AvailableCategories []string       `json:"available_categories"`
}

// MapPD copies only model-facing fields and verifies image bytes again at use time.
func (d *Dataset) MapPD(input PDInput) (conversation.ChatbotHomeProblemQuestion, []category.Category, error) {
	question := conversation.ChatbotHomeProblemQuestion{UserMessage: input.UserMessage, ContextSummary: input.ContextSummary, IsNewConversation: input.IsNewConversation}
	var categories []category.Category
	for _, name := range input.AvailableCategories {
		cat, err := category.New(name)
		if err != nil {
			return question, nil, fmt.Errorf("map category: %w", err)
		}
		categories = append(categories, *cat)
	}
	for _, item := range input.RecentMessages {
		role := item.SenderRole
		switch role {
		case "assistant":
			role = conversation.SenderChatbot
		case conversation.SenderConsumer, conversation.SenderProvider:
		default:
			return question, nil, fmt.Errorf("%w: unknown sender role %q", ErrInvalidDataset, role)
		}
		message := conversation.Message{SenderRole: role, Content: item.Content}
		for _, img := range item.Images {
			message.Images = append(message.Images, filedomain.MessageImage{Image: filedomain.Image{FileID: img.FileID}, Description: img.Description})
		}
		question.RecentMessages = append(question.RecentMessages, message)
	}
	if len(input.Images) == 0 {
		return question, categories, nil
	}
	root, err := os.OpenRoot(d.Root)
	if err != nil {
		return question, nil, fmt.Errorf("open image root: %w", err)
	}
	defer root.Close()
	for _, img := range input.Images {
		if err = d.validateImage(img); err != nil {
			return question, nil, err
		}
		data, readErr := root.ReadFile(img.Path)
		if readErr != nil {
			return question, nil, fmt.Errorf("read image %s: %w", img.AssetID, readErr)
		}
		if digest(data) != img.SHA256 {
			return question, nil, fmt.Errorf("%w: checksum mismatch for image %s", ErrInvalidDataset, img.AssetID)
		}
		question.Images = append(question.Images, filedomain.MessageImageContent{MessageImage: filedomain.MessageImage{Image: filedomain.Image{FileID: img.FileID, OriginalName: img.OriginalName}}, MimeType: img.MimeType, Data: data})
	}
	return question, categories, nil
}

type RKInput struct {
	ProblemTitle       string           `json:"problem_title"`
	ProblemDescription string           `json:"problem_description"`
	MaxResults         int              `json:"max_results"`
	Candidates         []CandidateInput `json:"candidates"`
}
type CandidateInput struct {
	Reference string        `json:"reference"`
	Evidence  EvidenceInput `json:"evidence"`
}
type EvidenceInput struct {
	RatingAverage      float64     `json:"rating_average"`
	RatingCount        int         `json:"rating_count"`
	RatingDistribution []int       `json:"rating_distribution"`
	PaidWorkCount      int         `json:"paid_work_count"`
	MostRecentPaidWork *time.Time  `json:"most_recent_paid_work"`
	WorkHistory        []WorkInput `json:"work_history"`
}
type WorkInput struct {
	ID               int              `json:"id"`
	ScheduledOn      time.Time        `json:"scheduled_on"`
	Description      string           `json:"description"`
	Status           string           `json:"status"`
	CompletionReport *CompletionInput `json:"completion_report"`
	Review           *ReviewInput     `json:"review"`
}
type CompletionInput struct {
	Description string    `json:"description"`
	ReportedOn  time.Time `json:"reported_on"`
}
type ReviewInput struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}

func (input RKInput) DomainRequest() (conversation.ProviderRankingRequest, error) {
	request := conversation.ProviderRankingRequest{ProblemTitle: input.ProblemTitle, ProblemDescription: input.ProblemDescription, MaxResults: input.MaxResults}
	if input.MaxResults <= 0 {
		return request, fmt.Errorf("%w: max results must be positive", ErrInvalidDataset)
	}
	seen := make(map[string]bool)
	for _, candidate := range input.Candidates {
		if strings.TrimSpace(candidate.Reference) == "" || seen[candidate.Reference] {
			return request, fmt.Errorf("%w: duplicate or empty candidate reference", ErrInvalidDataset)
		}
		seen[candidate.Reference] = true
		e := candidate.Evidence
		if len(e.RatingDistribution) != provider.RatingDistributionSize {
			return request, fmt.Errorf("%w: rating distribution requires five entries", ErrInvalidDataset)
		}
		evidence := conversation.ProviderRecommendationEvidence{RatingAverage: e.RatingAverage, RatingCount: e.RatingCount, PaidWorkCount: e.PaidWorkCount}
		copy(evidence.RatingDistribution[:], e.RatingDistribution)
		if e.MostRecentPaidWork != nil {
			evidence.MostRecentPaidWork = *e.MostRecentPaidWork
		}
		for _, work := range e.WorkHistory {
			mapped := readmodel.WorkOrder{ID: work.ID, ScheduledOn: work.ScheduledOn, Description: work.Description, Status: work.Status}
			if work.CompletionReport != nil {
				mapped.CompletionReport = &readmodel.CompletionReport{Description: work.CompletionReport.Description, ReportedOn: work.CompletionReport.ReportedOn}
			}
			if work.Review != nil {
				mapped.Review = &readmodel.Review{Rating: work.Review.Rating, Description: work.Review.Description}
			}
			evidence.WorkHistory = append(evidence.WorkHistory, mapped)
		}
		request.Candidates = append(request.Candidates, conversation.ProviderRecommendationCandidate{Reference: candidate.Reference, Evidence: evidence})
	}
	return request, nil
}
