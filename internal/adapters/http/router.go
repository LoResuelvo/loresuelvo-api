package httpadapter

import (
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/gin-gonic/gin"
)

type Router struct {
	consumerHandler *handler.ConsumerHandler
	authMiddleware  gin.HandlerFunc
}

func NewRouter(consumerHandler *handler.ConsumerHandler, authMiddleware ...gin.HandlerFunc) *Router {
	router := &Router{
		consumerHandler: consumerHandler,
	}

	if len(authMiddleware) > 0 {
		router.authMiddleware = authMiddleware[0]
	}

	return router
}

func (router *Router) SetUp() *gin.Engine {
	engine := gin.Default()

	router.registerHealthRoutes(engine)
	router.registerConsumerRoutes(engine)
	router.registerAuthenticatedRoutes(engine)

	return engine
}

func (router *Router) registerHealthRoutes(engine *gin.Engine) {
	engine.GET("/", func(context *gin.Context) {
		context.String(http.StatusOK, "Hello World")
	})
}

func (router *Router) registerConsumerRoutes(engine *gin.Engine) {
	engine.POST("/consumers", router.consumerHandler.RegisterConsumer)
}

func (router *Router) registerAuthenticatedRoutes(engine *gin.Engine) {
	if router.authMiddleware == nil {
		return
	}

	engine.GET("/me", router.authMiddleware, AuthenticatedUser)
}
