package httpadapter

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/consumer_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/conversation_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/job_request_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/service_proposal_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/test_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/user_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/work_order_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/gin-gonic/gin"
)

type RouterConfig struct {
	CategoryHandler        *category_handler.CategoryHandler
	ConsumerHandler        *consumer_handler.ConsumerHandler
	ProviderHandler        *provider_handler.ProviderHandler
	ConversationHandler    *conversation_handler.ConversationHandler
	JobRequestHandler      *job_request_handler.JobRequestHandler
	PaymentAccountHandler  *payment_account_handler.PaymentAccountHandler
	PaymentHandler         *payment_handler.PaymentHandler
	UserHandler            *user_handler.UserHandler
	FileHandler            *file_handler.FileHandler
	ServiceProposalHandler *service_proposal_handler.ServiceProposalHandler
	WorkOrderHandler       *work_order_handler.WorkOrderHandler
	TestHandler            *test_handler.TestHandler
	RealtimeHandler        *realtime.Handler
	Auth0Validator         *validator.Validator
	Logger                 *slog.Logger
}

type Router struct {
	categoryHandler        *category_handler.CategoryHandler
	consumerHandler        *consumer_handler.ConsumerHandler
	providerHandler        *provider_handler.ProviderHandler
	conversationHandler    *conversation_handler.ConversationHandler
	jobRequestHandler      *job_request_handler.JobRequestHandler
	paymentAccountHandler  *payment_account_handler.PaymentAccountHandler
	paymentHandler         *payment_handler.PaymentHandler
	userHandler            *user_handler.UserHandler
	fileHandler            *file_handler.FileHandler
	serviceProposalHandler *service_proposal_handler.ServiceProposalHandler
	workOrderHandler       *work_order_handler.WorkOrderHandler
	testHandler            *test_handler.TestHandler
	realtimeHandler        *realtime.Handler
	auth0Validator         *validator.Validator
	logger                 *slog.Logger
}

func NewRouter(config RouterConfig) *Router {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	router := &Router{
		categoryHandler:        config.CategoryHandler,
		consumerHandler:        config.ConsumerHandler,
		providerHandler:        config.ProviderHandler,
		conversationHandler:    config.ConversationHandler,
		jobRequestHandler:      config.JobRequestHandler,
		paymentAccountHandler:  config.PaymentAccountHandler,
		paymentHandler:         config.PaymentHandler,
		userHandler:            config.UserHandler,
		fileHandler:            config.FileHandler,
		serviceProposalHandler: config.ServiceProposalHandler,
		workOrderHandler:       config.WorkOrderHandler,
		testHandler:            config.TestHandler,
		realtimeHandler:        config.RealtimeHandler,
		auth0Validator:         config.Auth0Validator,
		logger:                 logger,
	}

	return router
}

func (router *Router) middlewareSetup() (gin.HandlerFunc, error) {
	return middleware.BaseAutheticationLayer(router.auth0Validator)
}

func (router *Router) SetUp() (*gin.Engine, error) {
	authMiddleware, err := router.middlewareSetup()
	if err != nil {
		return nil, fmt.Errorf("setting up middleware: %w", err)
	}

	engine := gin.New()
	engine.Use(middleware.RequestLogger(router.logger))
	engine.Use(middleware.Recovery(router.logger))
	engine.Use(middleware.CORSLayer(middleware.NewCORSConfigFromEnv()))

	router.registerHealthRoutes(engine)
	router.registerCategoryRoutes(engine)
	router.registerConsumerRoutes(engine, authMiddleware)
	router.registerProviderRoutes(engine, authMiddleware)
	router.registerPaymentAccountRoutes(engine, authMiddleware)
	router.registerJobRequestRoutes(engine, authMiddleware)
	router.registerConversationRoutes(engine, authMiddleware)
	router.registerChatbotRoutes(engine, authMiddleware)
	router.registerServiceProposalRoutes(engine, authMiddleware)
	router.registerPaymentRoutes(engine, authMiddleware)
	router.registerWorkOrderRoutes(engine, authMiddleware)
	router.registerAuthenticatedRoutes(engine, authMiddleware)
	router.registerFileRoutes(engine, authMiddleware)
	router.registerRealtimeRoutes(engine, authMiddleware)
	router.registerTestRoutes(engine)

	return engine, nil
}

func (router *Router) registerHealthRoutes(engine *gin.Engine) {
	engine.GET("/", func(context *gin.Context) {
		context.String(http.StatusOK, "Hello World")
	})
}

func (router *Router) registerCategoryRoutes(engine *gin.Engine) {
	engine.GET("/categories", router.categoryHandler.ListCategories)
	engine.POST("/categories", router.categoryHandler.CreateCategory)
}

func (router *Router) registerConsumerRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/consumers", authMiddleware, router.consumerHandler.RegisterConsumer)
}

func (router *Router) registerProviderRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/providers", router.providerHandler.FilterProvidersByCategory)
	engine.GET("/providers/:providerID", authMiddleware, router.providerHandler.GetProviderProfile)
	engine.POST("/providers", authMiddleware, router.providerHandler.RegisterProvider)
}

func (router *Router) registerPaymentAccountRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/providers/me/payment-accounts/authorization", authMiddleware, router.paymentAccountHandler.StartAuthorization)
	engine.GET("/providers/me/payment-accounts", authMiddleware, router.paymentAccountHandler.GetConnection)
	engine.GET("/oauth/payment-accounts/callback", router.paymentAccountHandler.CompleteAuthorization)
}

func (router *Router) registerJobRequestRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/job-requests", authMiddleware, router.jobRequestHandler.CreateJobRequest)
	engine.GET("/job-requests", authMiddleware, router.jobRequestHandler.GetJobRequests)
	engine.POST("/job-requests/:jobRequestID/accept", authMiddleware, router.jobRequestHandler.AcceptJobRequest)
}

func (router *Router) registerConversationRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/conversations", authMiddleware, router.conversationHandler.ListWorkConversations)
	engine.GET("/conversations/:conversationID", authMiddleware, router.conversationHandler.GetConversation)
	engine.POST("/conversations/:conversationID/messages", authMiddleware, router.conversationHandler.SendMessage)
}

func (router *Router) registerChatbotRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/chatbot/conversations", authMiddleware, router.conversationHandler.ListChatbotConversations)
	engine.POST("/chatbot/conversations", authMiddleware, router.conversationHandler.CreateChatbotConversation)
	engine.POST("/chatbot/conversations/:conversationID/messages", authMiddleware, router.conversationHandler.ContinueChatbotConversation)
	engine.POST("/chatbot/conversations/:conversationID/job-requests", authMiddleware, router.jobRequestHandler.CreateFromChatbotAssessment)
}

func (router *Router) registerServiceProposalRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/service-proposals", authMiddleware, router.serviceProposalHandler.CreateServiceProposal)
	engine.GET("/service-proposals", authMiddleware, router.serviceProposalHandler.GetServiceProposals)
	engine.POST("/service-proposals/:serviceProposalID/checkout-sessions", authMiddleware, router.paymentHandler.StartBookingCheckout)
}

func (router *Router) registerPaymentRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/payment-intents/:paymentIntentID", authMiddleware, router.paymentHandler.GetIntent)
	engine.POST("/webhooks/mercado-pago", router.paymentHandler.ProcessMercadoPagoWebhook)
}

func (router *Router) registerWorkOrderRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/work-orders", authMiddleware, router.workOrderHandler.GetWorkOrders)
	engine.GET("/work-orders/:workOrderID", authMiddleware, router.workOrderHandler.GetWorkOrder)
	engine.POST("/work-orders/:workOrderID/completion-reports", authMiddleware, router.workOrderHandler.ReportCompletion)
	engine.POST("/work-orders/:workOrderID/reviews", authMiddleware, router.workOrderHandler.CreateReview)
	engine.POST("/work-orders/:workOrderID/checkout-sessions", authMiddleware, router.paymentHandler.StartServiceBalanceCheckout)
}

func (router *Router) registerAuthenticatedRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/me", authMiddleware, router.userHandler.GetCurrentUser)
}

func (router *Router) registerFileRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/files/presign", authMiddleware, router.fileHandler.PresignUpload)
	engine.POST("/files/:fileID/confirm", authMiddleware, router.fileHandler.ConfirmUpload)
}

func (router *Router) registerRealtimeRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/ws-tickets", authMiddleware, router.realtimeHandler.IssueTicket)
	engine.GET("/ws", router.realtimeHandler.Handle)
}

func (router *Router) registerTestRoutes(engine *gin.Engine) {
	engine.POST("/test/clock", router.testHandler.SetTime)
	engine.POST("/test/clear", router.testHandler.ClearTestData)
}
