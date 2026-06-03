package httpadapter

import (
	"fmt"
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/gin-gonic/gin"
)

type Router struct {
	categoryHandler     *handler.CategoryHandler
	consumerHandler     *handler.ConsumerHandler
	providerHandler     *handler.ProviderHandler
	conversationHandler *handler.ConversationHandler
	jobRequestHandler   *handler.JobRequestHandler
	userHandler         *handler.UserHandler
	auth0Validator      *validator.Validator
}

func NewRouter(categoryHandler *handler.CategoryHandler, consumerHandler *handler.ConsumerHandler, providerHandler *handler.ProviderHandler, conversationHandler *handler.ConversationHandler, jobRequestHandler *handler.JobRequestHandler, userHandler *handler.UserHandler, auth0Validator *validator.Validator) *Router {
	router := &Router{
		categoryHandler:     categoryHandler,
		consumerHandler:     consumerHandler,
		providerHandler:     providerHandler,
		conversationHandler: conversationHandler,
		jobRequestHandler:   jobRequestHandler,
		userHandler:         userHandler,
		auth0Validator:      auth0Validator,
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
	engine.Use(gin.Recovery())
	engine.Use(middleware.CORSLayer(middleware.NewCORSConfigFromEnv()))

	router.registerHealthRoutes(engine)
	router.registerCategoryRoutes(engine)
	router.registerConsumerRoutes(engine, authMiddleware)
	router.registerProviderRoutes(engine, authMiddleware)
	router.registerJobRequestRoutes(engine, authMiddleware)
	router.registerConversationRoutes(engine, authMiddleware)
	router.registerAuthenticatedRoutes(engine, authMiddleware)

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
	engine.POST("/providers", authMiddleware, router.providerHandler.RegisterProvider)
}

func (router *Router) registerJobRequestRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/job-requests", authMiddleware, router.jobRequestHandler.CreateJobRequest)
	engine.GET("/job-requests", authMiddleware, router.jobRequestHandler.GetJobRequests)
	engine.POST("/job-requests/:jobRequestID/accept", authMiddleware, router.jobRequestHandler.AcceptJobRequest)
}

func (router *Router) registerConversationRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.POST("/conversations", authMiddleware, router.conversationHandler.CreateConversation)
	engine.GET("/conversations", authMiddleware, router.conversationHandler.ListConversations)
	engine.GET("/conversations/:conversationID", authMiddleware, router.conversationHandler.GetConversation)
	engine.POST("/conversations/:conversationID/messages", authMiddleware, router.conversationHandler.SendMessage)
}

func (router *Router) registerAuthenticatedRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	engine.GET("/me", authMiddleware, router.userHandler.GetCurrentUser)
}
