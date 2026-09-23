package admin_handler

import (
	"context"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/gin-gonic/gin"
)

type service interface {
	ListConsumers(ctx context.Context, query string) ([]readmodel.Consumer, error)
	ListProviders(ctx context.Context, query string) ([]readmodel.Provider, error)
}

type AdminHandler struct {
	service service
}

func NewAdminHandler(service service) *AdminHandler {
	return &AdminHandler{service: service}
}

func (handler *AdminHandler) ListConsumers(c *gin.Context) {
	consumers, err := handler.service.ListConsumers(c.Request.Context(), c.Query("q"))
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, consumerDirectoryResponsesFromReadModel(consumers))
}

func (handler *AdminHandler) ListProviders(c *gin.Context) {
	providers, err := handler.service.ListProviders(c.Request.Context(), c.Query("q"))
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, providerDirectoryResponsesFromReadModel(providers))
}
