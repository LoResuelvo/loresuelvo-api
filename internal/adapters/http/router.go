package httpadapter

import (
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/gin-gonic/gin"
)

type Router struct {
	consumerHandler *handler.ConsumerHandler
}

func NewRouter(consumerHandler *handler.ConsumerHandler) *Router {
	return &Router{
		consumerHandler: consumerHandler,
	}
}

func (router *Router) SetUp() *gin.Engine {
	engine := gin.Default()

	router.registerHealthRoutes(engine)
	router.registerConsumerRoutes(engine)

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
