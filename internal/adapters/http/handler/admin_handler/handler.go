package admin_handler

import (
	"context"
	"net/http"
	"strconv"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/gin-gonic/gin"
)

type service interface {
	ListConsumers(ctx context.Context, query string) ([]readmodel.Consumer, error)
	ListProviders(ctx context.Context, filter admin.ProviderDirectoryFilter) ([]readmodel.Provider, error)
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
	filter := admin.ProviderDirectoryFilter{Query: c.Query("q")}
	if raw, present := c.GetQuery("category_id"); present {
		id, err := strconv.Atoi(raw)
		if err != nil {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		filter.CategoryID = &id
	}
	if raw, present := c.GetQuery("coverage_zone_id"); present {
		id, err := strconv.Atoi(raw)
		if err != nil {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		filter.CoverageZoneID = &id
	}
	if raw, present := c.GetQuery("identity_verification_status"); present {
		status := identityverification.VerificationStatus(raw)
		filter.IdentityVerificationStatus = &status
	}
	providers, err := handler.service.ListProviders(c.Request.Context(), filter)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, providerDirectoryResponsesFromReadModel(providers))
}
