package steps_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	chatbotadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/scheduler"
	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSuite struct {
	server                   *httptest.Server
	database                 *sql.DB
	categoryRepository       *repositories.CategoryRepository
	conversationRepository   *repositories.ConversationRepository
	messageRepository        *repositories.MessageRepository
	jobRequestRepository     *repositories.JobRequestRepository
	userRepository           *repositories.UserRepository
	fileRepository           *repositories.FileRepository
	notificationRepository   *repositories.NotificationRepository
	workOrderRepository      *repositories.WorkOrderRepository
	paymentAccountRepository *repositories.PaymentAccountRepository
	urgentWorkOrderScheduler *scheduler.Scheduler
	auth0Validator           *validator.Validator
	tokenBuilder             *auth0.TokenBuilder
	chatbot                  *chatbotadapter.FakeChatbot
	clock                    *clockadapter.SystemClock

	lastStatus                              int
	lastBody                                []byte
	currentAuth0ID                          string
	lastConversationID                      int
	lastJobRequestID                        int
	lastWorkRequestProviderID               int
	lastProviderProfileID                   int
	providerProfilePhotoFileID              string
	consumerProfilePhotoFileID              string
	realtimeConnections                     map[string]*realtimeTestConnection
	chatbotConversationIDs                  []int
	chatbotConversationStatuses             []string
	lastChatbotRecommendedCategoryName      string
	expectedChatbotContextSummary           string
	expectedRecentChatbotContextMessage     string
	lastAttemptedChatbotContinuationMessage string
	messageImagesByName                     map[string]messageImageFixture
	lastSentMessageID                       int
	lastAttemptedMessageImageNames          []string
	aiJobRequestsByProvider                 map[string]jobRequestCreationResponse
	aiSourceChatbotConversationID           int
	aiExpectedAssessmentID                  int
	aiExpectedJobRequestTitle               string
	aiExpectedJobRequestDescription         string
	aiJobRequestCountBeforeContact          int
	aiWorkConversationIDsBeforeContact      map[int]int
	aiAttemptedProviderIDs                  []int
	expectedChatbotImageDescriptions        map[string]string
	expectedAssessmentImageNames            []string
	previousAssessmentID                    int
	previousAssessmentImages                []filedomain.MessageImage
	consumerMessageCountBeforeAttempt       int
	chatbotMessageCountBeforeAttempt        int
	lastServiceProposalRequest              serviceProposalCreationRequest
	serviceProposalConversationIDs          map[string]int
	lastServiceProposalID                   int
	serviceProposalIDs                      []int
	serviceProposalFixtures                 map[int]serviceProposalFixture
	workOrdersByServiceProposalID           map[int][]workOrderResponse
	mercadoPagoAccounts                     map[string]mercadoPagoAccountFixture
	lastMercadoPagoOAuthState               string

	categoryIDsByName              map[string]int
	lastProviderFilterCategoryName string
	participantRolesByFullName     map[string]string
}

func (s *testSuite) registerAllSteps(sc *godog.ScenarioContext) {
	registerHelloWorldSteps(sc, s)
	registerConsumerAccountSteps(sc, s)
	registerProviderAccountSteps(sc, s)
	registerProviderWithProfilePhotoSteps(sc, s)
	registerCreateCategorySteps(sc, s)
	registerListCategoriesSteps(sc, s)
	registerFilterProvidersByCategorySteps(sc, s)
	registerLoginSteps(sc, s)
	registerGetProviderProfileSteps(sc, s)
	registerSendContactRequestToProviderSteps(sc, s)
	registerGetConversationSteps(sc, s)
	registerGetConversationsSteps(sc, s)
	registerSendMessageSteps(sc, s)
	registerPostJobRequestSteps(sc, s)
	registerPostServiceProposalSteps(sc, s)
	registerGetServiceProposalsSteps(sc, s)
	registerAcceptServiceProposalSteps(sc, s)
	registerGetWorkOrdersSteps(sc, s)
	registerConnectMercadoPagoAccountSteps(sc, s)
	registerNotifyUrgentWorkOrdersSteps(sc, s)
	registerGetJobRequestSteps(sc, s)
	registerJobRequestImagesSteps(sc, s)
	registerAcceptJobRequestSteps(sc, s)
	registerRealtimeMessageSteps(sc, s)
	registerAttachMessageImagesSteps(sc, s)
	registerChatbotSteps(sc, s)
	registerChatbotContinuationSteps(sc, s)
	registerChatbotAttachImagesSteps(sc, s)
	registerChatbotImageContextSteps(sc, s)
	registerChatbotGetConversationsSteps(sc, s)
	registerChatbotGetConversationSteps(sc, s)
	registerChatbotProviderRecommendationSteps(sc, s)
	registerAIJobRequestSteps(sc, s)
	registerAIJobRequestImageSteps(sc, s)
}

func (s *testSuite) cleanup() error {
	s.closeRealtimeConnections()

	if err := s.paymentAccountRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean payment accounts: %w", err)
	}

	if err := s.jobRequestRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean job requests: %w", err)
	}

	if err := s.notificationRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean notifications: %w", err)
	}

	if err := s.conversationRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean conversations: %w", err)
	}

	if err := s.messageRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean messages: %w", err)
	}

	if err := s.userRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean users: %w", err)
	}

	if err := s.fileRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean files: %w", err)
	}

	if err := s.categoryRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not clean categories: %w", err)
	}

	s.clock.Reset()

	s.categoryIDsByName = map[string]int{}
	s.participantRolesByFullName = map[string]string{}
	s.realtimeConnections = map[string]*realtimeTestConnection{}
	s.lastWorkRequestProviderID = 0
	s.lastProviderProfileID = 0
	s.lastJobRequestID = 0
	s.providerProfilePhotoFileID = ""
	s.chatbot.Reset()
	s.chatbotConversationIDs = nil
	s.chatbotConversationStatuses = nil
	s.lastChatbotRecommendedCategoryName = ""
	s.expectedChatbotContextSummary = ""
	s.expectedRecentChatbotContextMessage = ""
	s.lastAttemptedChatbotContinuationMessage = ""
	s.messageImagesByName = map[string]messageImageFixture{}
	s.lastSentMessageID = 0
	s.lastAttemptedMessageImageNames = nil
	s.aiJobRequestsByProvider = map[string]jobRequestCreationResponse{}
	s.aiSourceChatbotConversationID = 0
	s.aiExpectedAssessmentID = 0
	s.aiExpectedJobRequestTitle = ""
	s.aiExpectedJobRequestDescription = ""
	s.aiJobRequestCountBeforeContact = 0
	s.aiWorkConversationIDsBeforeContact = map[int]int{}
	s.aiAttemptedProviderIDs = nil
	s.expectedChatbotImageDescriptions = map[string]string{}
	s.expectedAssessmentImageNames = nil
	s.previousAssessmentID = 0
	s.previousAssessmentImages = nil
	s.consumerMessageCountBeforeAttempt = 0
	s.chatbotMessageCountBeforeAttempt = 0
	s.lastServiceProposalRequest = serviceProposalCreationRequest{}
	s.serviceProposalConversationIDs = map[string]int{}
	s.lastServiceProposalID = 0
	s.serviceProposalIDs = nil
	s.serviceProposalFixtures = map[int]serviceProposalFixture{}
	s.workOrdersByServiceProposalID = map[int][]workOrderResponse{}
	s.mercadoPagoAccounts = map[string]mercadoPagoAccountFixture{}
	s.lastMercadoPagoOAuthState = ""
	return nil
}

func newTestDb() *sql.DB {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	if err != nil {
		panic(fmt.Errorf("cannot connect to test database: %w", err))
	}
	return database
}

func newTestSuite(tb testing.TB, database *sql.DB) *testSuite {
	chatbot := chatbotadapter.NewFakeChatbot()
	credentialCipher, err := cryptography.NewAESGCMCipher(
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)),
	)
	require.NoError(tb, err, "could not initialize test credential cipher")
	dependencies := bootstrap.NewDependenciesWithPaymentAccountAdapters(
		database,
		chatbot,
		mercadopago.NewFakeOAuthClient(),
		credentialCipher,
		cryptography.NewSecureSecretGenerator(),
		payment_account_handler.Config{
			ConnectionSuccessURL: "http://frontend.loresuelvo.test/provider/register/mercado-pago?result=success",
		},
	)
	auth0Validator := auth0.NewFakeValidator()
	tokenBuilder := auth0.NewTokenBuilder()

	router := httpadapter.NewRouter(dependencies.RouterConfig(auth0Validator))
	engine, err := router.SetUp()
	require.NoError(tb, err, "could not initialize router")

	// httptest.Server wraps the engine — no port needed
	server := httptest.NewServer(engine)
	tb.Cleanup(func() {
		server.Close()
	})

	return &testSuite{
		server:                   server,
		database:                 database,
		categoryRepository:       dependencies.Persistence.CategoryRepository,
		conversationRepository:   dependencies.Persistence.ConversationRepository,
		messageRepository:        dependencies.Persistence.MessageRepository,
		jobRequestRepository:     dependencies.Persistence.JobRequestRepository,
		userRepository:           dependencies.Persistence.UserRepository,
		fileRepository:           dependencies.Persistence.FileRepository,
		notificationRepository:   dependencies.Persistence.NotificationRepository,
		workOrderRepository:      dependencies.Persistence.WorkOrderRepository,
		paymentAccountRepository: dependencies.Persistence.PaymentAccountRepository,
		urgentWorkOrderScheduler: dependencies.UrgentWorkOrderScheduler,
		auth0Validator:           auth0Validator,
		tokenBuilder:             tokenBuilder,
		chatbot:                  chatbot,
		clock:                    dependencies.Clock,

		categoryIDsByName:                  map[string]int{},
		participantRolesByFullName:         map[string]string{},
		realtimeConnections:                map[string]*realtimeTestConnection{},
		messageImagesByName:                map[string]messageImageFixture{},
		aiJobRequestsByProvider:            map[string]jobRequestCreationResponse{},
		aiWorkConversationIDsBeforeContact: map[int]int{},
		expectedChatbotImageDescriptions:   map[string]string{},
		serviceProposalConversationIDs:     map[string]int{},
		serviceProposalFixtures:            map[int]serviceProposalFixture{},
		workOrdersByServiceProposalID:      map[int][]workOrderResponse{},
		mercadoPagoAccounts:                map[string]mercadoPagoAccountFixture{},
	}
}

func ScenarioInitializer(sc *godog.ScenarioContext, t *testing.T, database *sql.DB) {
	testSuite := newTestSuite(t, database)
	testSuite.registerAllSteps(sc)
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if err := testSuite.cleanup(); err != nil {
			return ctx, fmt.Errorf("could not clean test status: %w", err)
		}
		return ctx, nil
	})
}

// Godog entry point
func TestFeatures(t *testing.T) {
	database := newTestDb()
	t.Cleanup(func() { database.Close() })

	suite := godog.TestSuite{
		Name: "LoResuelvo Features",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			ScenarioInitializer(sc, t, database)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../"},
			Tags:     "~@wip",
			TestingT: t,
		},
	}

	assert.Equal(t, 0, suite.Run(), "godog tests failed")
}
