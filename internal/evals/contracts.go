package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type ContractResult struct {
	CaseID         string   `json:"case_id"`
	Status         string   `json:"status"`
	SemanticStatus string   `json:"semantic_status"`
	Evidence       []string `json:"evidence"`
}

// ExecuteContracts executes only controlled offline dependencies. A pass is not a
// semantic certification or evidence of PostgreSQL filtering.
func ExecuteContracts(ctx context.Context, dataset *Dataset) ([]ContractResult, error) {
	if dataset == nil {
		return nil, fmt.Errorf("%w: dataset required", ErrInvalidDataset)
	}
	results := make([]ContractResult, 0, len(dataset.CT))
	for _, c := range dataset.CT {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result, err := executeContract(ctx, dataset, c)
		if err != nil {
			return results, fmt.Errorf("execute %s: %w", c.ID, err)
		}
		results = append(results, result)
	}
	return results, nil
}

type contractInput struct {
	Eligible           []string          `json:"eligible"`
	EligibleCandidates []json.RawMessage `json:"eligible_candidates"`
	ProblemCategory    string            `json:"problem_category"`
	ConsumerZone       string            `json:"consumer_zone"`
	Candidates         []struct {
		Reference string   `json:"reference"`
		Category  string   `json:"category"`
		Zones     []string `json:"zones"`
	} `json:"candidates"`
	RawOutput struct {
		Recommendations []struct {
			Reference string `json:"reference"`
			Reason    string `json:"reason"`
		} `json:"recommendations"`
	} `json:"raw_output"`
	MaxResults          int            `json:"max_results"`
	ReturnedReferences  []string       `json:"returned_references"`
	KnownImageRefs      []string       `json:"known_image_refs"`
	SelectedImageRefs   []string       `json:"selected_image_refs"`
	AvailableCategories []string       `json:"available_categories"`
	Outcome             string         `json:"outcome"`
	ProblemCategoryName string         `json:"problem_category_name"`
	UserMessage         string         `json:"user_message"`
	PreviousSummary     string         `json:"previous_summary"`
	Messages            []MessageInput `json:"messages"`
	Path                string         `json:"path"`
	FamilyID            string         `json:"family_id"`
	Splits              []string       `json:"splits"`
}

func executeContract(ctx context.Context, d *Dataset, c CTCase) (ContractResult, error) {
	r := ContractResult{CaseID: c.ID, Status: "fail", SemanticStatus: "not_applicable"}
	var input contractInput
	if err := json.Unmarshal(c.Input, &input); err != nil {
		return r, err
	}
	if c.ID == "CT-09" {
		return executeTimeoutContract(ctx, d, r)
	}
	if c.ID == "CT-11" {
		copyDataset := *d
		image := ImageInput{AssetID: "contract-missing", Path: input.Path, FileID: "contract-missing", SHA256: digest(nil), MimeType: "image/png"}
		copyDataset.assets = map[string]ImageInput{image.AssetID: image}
		_, _, err := copyDataset.MapPD(PDInput{Images: []ImageInput{image}})
		if errors.Is(err, os.ErrNotExist) {
			r.Status = "pass"
			r.Evidence = []string{"asset_error: " + err.Error(), "Image mapping failed before any chatbot dependency was constructed; no textual substitution."}
		}
		return r, nil
	}
	if c.ID == "CT-12" {
		copyDataset := *d
		copyDataset.PD = nil
		copyDataset.RK = nil
		copyDataset.CT = nil
		for i, split := range input.Splits {
			copyDataset.PD = append(copyDataset.PD, PDCase{CaseMetadata: CaseMetadata{ID: fmt.Sprintf("CT-family-%d", i), Task: "prediagnosis", FamilyID: input.FamilyID, Split: split}})
		}
		err := copyDataset.validateReferences()
		if errors.Is(err, ErrInvalidDataset) && strings.Contains(err.Error(), "crosses splits") {
			r.Status = "pass"
			r.Evidence = []string{err.Error()}
		}
		return r, nil
	}
	if !slices.Contains([]string{"CT-01", "CT-02", "CT-03", "CT-04", "CT-05", "CT-06", "CT-07", "CT-08", "CT-10"}, c.ID) {
		r.Status = "not_implemented"
		r.Evidence = []string{"No registered executor."}
		return r, nil
	}
	return executeServiceContract(ctx, c.ID, input)
}

func executeServiceContract(ctx context.Context, id string, input contractInput) (ContractResult, error) {
	r := ContractResult{CaseID: id, Status: "fail", SemanticStatus: "not_applicable"}
	users := &contractUsers{}
	store := &contractStore{}
	bot := &contractChatbot{response: conversation.ChatbotResponse{Status: conversation.ChatbotResponseAnswered, Title: "Contract", Content: "Controlled response", Assessment: conversation.ChatbotAssessmentResponse{Action: conversation.ChatbotAssessmentReplace, Outcome: conversation.AssessmentProfessionalRequired, ProblemCategoryName: "Plumbing", ProblemTitle: "Leak", ProblemDescription: "Leak requiring repair"}}}
	categories := contractCategories{{ID: 1, Name: "Plumbing", NormalizedName: "plumbing"}}
	config := conversation.DefaultProviderRecommendationConfig()
	if len(input.AvailableCategories) > 0 {
		categories = nil
		for i, name := range input.AvailableCategories {
			cat, err := category.New(name)
			if err != nil {
				return r, err
			}
			cat.ID = i + 1
			categories = append(categories, *cat)
		}
	}
	references := input.Eligible
	if id == "CT-05" {
		references = input.ReturnedReferences
		config.MaxRecommendedProviders = input.MaxResults
	}
	if id == "CT-02" {
		cat, err := category.New(input.ProblemCategory)
		if err != nil {
			return r, err
		}
		cat.ID = 1
		categories = contractCategories{*cat}
		bot.response.Assessment.ProblemCategoryName = input.ProblemCategory
		// The repository boundary supplies its expected result. This does not test SQL.
		for _, candidate := range input.Candidates {
			if candidate.Category == input.ProblemCategory && slices.Contains(candidate.Zones, input.ConsumerZone) {
				references = append(references, candidate.Reference)
			}
		}
	}
	for i := range references {
		users.providers = append(users.providers, provider.Provider{BaseUser: user.RehydrateBaseUser(i+2, "provider", "provider@example.test", "Contract", "Provider", provider.Role, nil), Category: &categories[0]})
	}
	var rankingFixtureErr error
	bot.rank = func(request conversation.ProviderRankingRequest) *conversation.ProviderRankingResponse {
		result := &conversation.ProviderRankingResponse{}
		if id == "CT-02" {
			return result
		}
		mapping := map[string]string{}
		if len(request.Candidates) != len(references) {
			rankingFixtureErr = fmt.Errorf("candidate count differs from controlled eligible providers")
			return nil
		}
		byID := make(map[int]string, len(request.Candidates))
		for _, candidate := range request.Candidates {
			if _, duplicate := byID[candidate.ProviderID]; duplicate {
				rankingFixtureErr = fmt.Errorf("duplicate controlled provider ID")
				return nil
			}
			byID[candidate.ProviderID] = candidate.Reference
		}
		for i, reference := range references {
			actual, ok := byID[i+2]
			if !ok {
				rankingFixtureErr = fmt.Errorf("missing controlled provider ID %d", i+2)
				return nil
			}
			mapping[reference] = actual
		}
		if id == "CT-05" {
			for _, reference := range input.ReturnedReferences {
				result.Recommendations = append(result.Recommendations, conversation.ProviderRankingRecommendation{Reference: mapping[reference], Reason: "Controlled reason"})
			}
			return result
		}
		for _, item := range input.RawOutput.Recommendations {
			reference := item.Reference
			if mapped, ok := mapping[reference]; ok {
				reference = mapped
			}
			result.Recommendations = append(result.Recommendations, conversation.ProviderRankingRecommendation{Reference: reference, Reason: item.Reason})
		}
		return result
	}
	if id == "CT-06" {
		bot.response.Assessment.Outcome = conversation.AssessmentSelfService
		bot.response.Assessment.ProblemCategoryName = ""
		bot.response.Assessment.SelectedImageRefs = input.SelectedImageRefs
	}
	if id == "CT-07" {
		bot.response.Assessment.Outcome = conversation.ProblemAssessmentOutcome(input.Outcome)
		bot.response.Assessment.ProblemCategoryName = input.ProblemCategoryName
	}
	if id == "CT-08" {
		bot.response.Assessment.ProblemCategoryName = ""
	}
	service := conversation.NewService(store, users, nil, nil, bot, categories, contractFiles{}, contractClock{}, config, contractEvidence{})
	var callErr error
	if id == "CT-06" || id == "CT-10" {
		existing, err := conversation.NewChatbotConversation(1, "Contract")
		if err != nil {
			return r, err
		}
		chat := existing.(*conversation.ChatBotConversation)
		store.current = chat
		if id == "CT-06" {
			for i, reference := range input.KnownImageRefs {
				chat.AddMessage(conversation.Message{ID: i + 1, SenderRole: conversation.SenderConsumer, Content: "Image evidence", Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: strings.TrimPrefix(reference, "image:")}, Description: "Known image"}}})
			}
		} else {
			chat.Context.Summary = input.PreviousSummary
			for i, message := range input.Messages {
				chat.AddMessage(conversation.Message{ID: i + 1, SenderRole: message.SenderRole, Content: message.Content})
			}
			for len(chat.Messages()) < conversation.ChatbotRecentMessageLimit {
				chat.AddMessage(conversation.Message{ID: len(chat.Messages()) + 1, SenderRole: conversation.SenderChatbot, Content: "Controlled padding to reach summary threshold"})
			}
			bot.response.Assessment = conversation.ChatbotAssessmentResponse{Action: conversation.ChatbotAssessmentReplace, Outcome: conversation.AssessmentCollectingInformation}
		}
		_, callErr = service.ContinueChatbotConversation(ctx, "contract", 1, "Continue")
	} else {
		content := input.UserMessage
		if content == "" {
			content = "Evaluate contract"
		}
		_, callErr = service.CreateChatbotConversation(ctx, "contract", content)
	}
	if rankingFixtureErr != nil {
		r.Evidence = []string{rankingFixtureErr.Error()}
		return r, nil
	}
	switch id {
	case "CT-01":
		if callErr == nil && bot.rankingCalls == 0 && store.saves == 1 && len(store.current.(*conversation.ChatBotConversation).CurrentRecommendation.Recommendations) == 0 {
			r.Status = "pass"
		}
	case "CT-02":
		if callErr == nil && users.queries == 1 && users.categoryID == 1 && users.zoneID == 1 && bot.rankingCalls == 1 && len(bot.rankingRequest.Candidates) == len(references) {
			r.Status = "unassessed"
			r.Evidence = append(r.Evidence, "Service passed the requested category/zone to the repository and forwarded its candidates; actual SQL filtering requires repository integration evidence.")
		}
	case "CT-03", "CT-04", "CT-05":
		if errors.Is(callErr, conversation.ErrProviderRecommendationInvalid) && store.saves == 0 {
			r.Status = "pass"
		}
	case "CT-06", "CT-07":
		if errors.Is(callErr, conversation.ErrProblemAssessmentInvalid) {
			r.Status = "pass"
		}
	case "CT-08":
		r.SemanticStatus = "unassessed"
		if errors.Is(callErr, conversation.ErrProblemAssessmentInvalid) && bot.rankingCalls == 0 {
			r.Status = "unassessed"
			r.Evidence = append(r.Evidence, "Current service rejects professional_required without a category and does not rank. Controlled response does not establish whether real urgent guidance is delivered.")
		}
	case "CT-10":
		r.SemanticStatus = "unassessed"
		if callErr == nil && bot.summaryCalls == 1 && bot.previousSummary == input.PreviousSummary && contractMessagesPreserved(input.Messages, bot.summaryMessages) && bot.question.ContextSummary == "controlled summary" {
			r.Status = "unassessed"
			r.Evidence = append(r.Evidence, "Actual continuation forwarded previous summary and pending messages and reused controlled summary. Risk preservation requires live output and semantic review.")
		}
	}
	r.Evidence = append(r.Evidence, fmt.Sprintf("Controlled ranking calls: %d; persistence calls: %d.", bot.rankingCalls, store.saves))
	if callErr != nil {
		r.Evidence = append(r.Evidence, "Observed service error: "+callErr.Error())
	}
	return r, nil
}

func contractMessagesPreserved(input []MessageInput, observed []conversation.Message) bool {
	if len(observed) < len(input) {
		return false
	}
	for i, message := range input {
		if observed[i].Content != message.Content || observed[i].SenderRole != message.SenderRole {
			return false
		}
	}
	return true
}

func executeTimeoutContract(ctx context.Context, _ *Dataset, result ContractResult) (_ ContractResult, resultErr error) {
	directory, err := os.MkdirTemp("", "eval-contract-")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, os.RemoveAll(directory)) }()
	dataset := &Dataset{
		Version: "contract", ManifestSHA256: digest([]byte("contract-timeout")),
		PD:          []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-timeout", Task: "prediagnosis", FamilyID: "timeout", Split: "development"}, Expected: json.RawMessage(`{"semantic_assertions":[{"id":"controlled-check","severity":"critical","criterion":"Requires actual output"}]}`)}},
		Suites:      map[string][]string{"smoke": {"PD-timeout"}},
		schemaFiles: map[string][]byte{"schemas/prediagnosis-output.schema.json": []byte(`{"type":"object"}`)},
	}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "controlled-timeout", Trials: 1, MaxRequests: 1})
	if err != nil {
		return result, err
	}
	limits := ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Millisecond, GlobalTimeout: time.Second, MinInterval: time.Millisecond, MaxOutputTokens: 1}
	record := RunRecord{FormatVersion: resultVersion, RunID: "contract-timeout", Mode: "contract", Plan: plan, Limits: limits}
	runDirectory := filepath.Join(directory, "run")
	journal, err := NewJournal(runDirectory, record)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.Close()) }()
	_, err = Run(ctx, dataset, plan, limits, contractTimeoutExecutor{}, journal, record, CredentialRedactor(""))
	if err != nil {
		return result, err
	}
	observed, attempts, err := ReadRun(runDirectory)
	if err != nil {
		return result, err
	}
	_, report, err := Replay(dataset, runDirectory)
	if err != nil {
		return result, err
	}
	semanticUnassessed := len(report.Attempts) == 1 && len(report.Attempts[0].Evaluation.SemanticChecks) > 0
	if semanticUnassessed {
		for _, check := range report.Attempts[0].Evaluation.SemanticChecks {
			if check.Result != "unassessed" {
				semanticUnassessed = false
			}
		}
	}
	result.SemanticStatus = report.SemanticStatus
	if semanticUnassessed && report.DeterministicFailures == 1 && !report.ReleaseApproved && len(attempts) == 1 && attempts[0].Status == "execution_error" && strings.Contains(attempts[0].Error, context.DeadlineExceeded.Error()) && !observed.ReleaseApproved {
		result.Status = "pass"
		result.Evidence = []string{"Real runner and replay recorded the controlled deadline as execution_error, failed deterministic scoring, left actual semantic checks unassessed, and did not approve release."}
	}
	return result, nil
}
