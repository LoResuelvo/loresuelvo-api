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
	for _, key := range []string{"category_id", "coverage_zone_id", "identity_verification_status"} {
		if _, present := c.Request.URL.Query()[key]; present {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
	}

	consumers, err := handler.service.ListConsumers(c.Request.Context(), c.Query("q"))
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, consumerDirectoryResponsesFromReadModel(consumers))
}

func (handler *AdminHandler) ListProviders(c *gin.Context) {
	filter := admin.ProviderDirectoryFilter{Query: c.Query("q")}
	categoryID, valid := positiveFilterID(c, "category_id")
	if !valid {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
	}
	filter.CategoryID = categoryID
	coverageZoneID, valid := positiveFilterID(c, "coverage_zone_id")
	if !valid {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
	}
	filter.CoverageZoneID = coverageZoneID
	if values, present := c.Request.URL.Query()["identity_verification_status"]; present {
		if len(values) != 1 {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		status := identityverification.VerificationStatus(values[0])
		if !status.IsValid() {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		filter.IdentityVerificationStatus = &status
	}
	providers, err := handler.service.ListProviders(c.Request.Context(), filter)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, providerDirectoryResponsesFromReadModel(providers))
}

func positiveFilterID(c *gin.Context, key string) (*int, bool) {
	values, present := c.Request.URL.Query()[key]
	if !present {
		return nil, true
	}
	if len(values) != 1 {
		return nil, false
	}
	id, err := strconv.Atoi(values[0])
	if err != nil || id <= 0 {
		return nil, false
	}
	return &id, true
}
