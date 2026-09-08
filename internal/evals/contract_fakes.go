package evals

import (
	"context"
	"errors"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

var errUnexpectedContractCall = errors.New("unexpected contract dependency call")

// These controlled dependencies are part of the offline executable, not test mocks.
type contractUsers struct {
	providers                   []provider.Provider
	categoryID, zoneID, queries int
}

func (u *contractUsers) FindByAuthID(string) (user.User, error) {
	return nil, errUnexpectedContractCall
}
func (u *contractUsers) FindConsumerByAuthID(context.Context, string) (*consumer.Consumer, error) {
	base := user.RehydrateBaseUser(1, "contract", "consumer@example.test", "Contract", "Consumer", consumer.Role, nil)
	return consumer.RehydrateConsumer(base, consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{ID: 1}), nil
}
func (u *contractUsers) FindProviderByID(context.Context, int) (*provider.Provider, error) {
	return nil, errUnexpectedContractCall
}
func (u *contractUsers) FindProvidersByCategoryAndCoverageZoneID(_ context.Context, categoryID, zoneID int) ([]provider.Provider, error) {
	u.categoryID, u.zoneID = categoryID, zoneID
	u.queries++
	return u.providers, nil
}

type contractCategories []category.Category

func (c contractCategories) ListAll() ([]category.Category, error) { return c, nil }

type contractStore struct {
	current conversation.Conversation
	saves   int
}

func (s *contractStore) SaveConversation(_ context.Context, c conversation.Conversation) (conversation.Conversation, error) {
	s.current = c
	s.saves++
	return c, nil
}
func (s *contractStore) FindByID(context.Context, int) (conversation.Conversation, error) {
	if s.current == nil {
		return nil, errUnexpectedContractCall
	}
	return s.current, nil
}
func (*contractStore) CountMessagesBySenderRole(context.Context, int, string) (int, error) {
	return 0, errUnexpectedContractCall
}

type contractClock struct{}

func (contractClock) Now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

type contractEvidence struct{}

func (contractEvidence) FindRatingStatsByProviderIDs(context.Context, []int) (map[int]provider.RatingStats, error) {
	return map[int]provider.RatingStats{}, nil
}
func (contractEvidence) FindPaidWorkHistoryByProviderIDs(context.Context, []int) (map[int][]readmodel.WorkOrder, error) {
	return map[int][]readmodel.WorkOrder{}, nil
}

type contractChatbot struct {
	response        conversation.ChatbotResponse
	rank            func(conversation.ProviderRankingRequest) *conversation.ProviderRankingResponse
	rankingCalls    int
	rankingRequest  conversation.ProviderRankingRequest
	summaryCalls    int
	previousSummary string
	summaryMessages []conversation.Message
	question        conversation.ChatbotHomeProblemQuestion
}

func (c *contractChatbot) AnswerHomeProblemQuestion(_ context.Context, q conversation.ChatbotHomeProblemQuestion, _ []category.Category) (*conversation.ChatbotResponse, error) {
	c.question = q
	return &c.response, nil
}
func (c *contractChatbot) RankProviders(_ context.Context, r conversation.ProviderRankingRequest) (*conversation.ProviderRankingResponse, error) {
	c.rankingCalls++
	c.rankingRequest = r
	if c.rank == nil {
		return nil, errUnexpectedContractCall
	}
	return c.rank(r), nil
}
func (c *contractChatbot) SummarizeHomeProblemConversation(_ context.Context, previous string, messages []conversation.Message) (string, error) {
	c.summaryCalls++
	c.previousSummary = previous
	c.summaryMessages = append([]conversation.Message(nil), messages...)
	return "controlled summary", nil
}

type contractFiles struct{}

func (contractFiles) PrepareChatbotMessageImages(_ context.Context, _ string, ids []string) ([]filedomain.MessageImageContent, error) {
	if len(ids) > 0 {
		return nil, errUnexpectedContractCall
	}
	return nil, nil
}
func (contractFiles) ResolvePublicURLs(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (contractFiles) ResolvePublicURL(context.Context, string) (string, error) {
	return "", errUnexpectedContractCall
}
func (contractFiles) PrepareMessageImages(context.Context, string, []string) ([]filedomain.MessageImage, error) {
	return nil, errUnexpectedContractCall
}
func (contractFiles) PrepareMessageAudio(context.Context, string, string) (*filedomain.MessageAudio, error) {
	return nil, errUnexpectedContractCall
}
func (contractFiles) PrepareMessageVideo(context.Context, string, string) (*filedomain.MessageVideo, error) {
	return nil, errUnexpectedContractCall
}
func (contractFiles) ResolveMessageImages(context.Context, []string) (map[string]filedomain.MessageImage, error) {
	return nil, errUnexpectedContractCall
}
func (contractFiles) ResolveMessageAudios(context.Context, []string) (map[string]filedomain.MessageAudio, error) {
	return nil, errUnexpectedContractCall
}
func (contractFiles) ResolveMessageVideos(context.Context, []string) (map[string]filedomain.MessageVideo, error) {
	return nil, errUnexpectedContractCall
}

type contractTimeoutExecutor struct{}

func (contractTimeoutExecutor) Execute(ctx context.Context, _ string) (ExecutionOutput, error) {
	<-ctx.Done()
	return ExecutionOutput{RequestCount: 0}, ctx.Err()
}
