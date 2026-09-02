package health_handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ReadinessChecker interface {
	CheckReadiness(context.Context) error
}

type HealthHandler struct {
	readiness ReadinessChecker
}

func NewHealthHandler(readiness ReadinessChecker) *HealthHandler {
	return &HealthHandler{readiness: readiness}
}

func (handler *HealthHandler) Live(context *gin.Context) {
	context.Status(http.StatusOK)
}

func (handler *HealthHandler) Ready(context *gin.Context) {
	if err := handler.readiness.CheckReadiness(context.Request.Context()); err != nil {
		context.Status(http.StatusServiceUnavailable)
		return
	}

	context.Status(http.StatusOK)
}
