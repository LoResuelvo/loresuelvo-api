package provider_handler

import (
	"net/http"
	"strconv"
	"strings"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/gin-gonic/gin"
)

type ProviderHandler struct {
	providerService *provider.Service
}

func NewProviderHandler(providerService *provider.Service) *ProviderHandler {
	return &ProviderHandler{
		providerService: providerService,
	}
}

func (h *ProviderHandler) RegisterProvider(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req registerProviderRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	req = normalizeRegisterProviderRequest(req)

	createdProvider, err := h.providerService.RegisterProvider(c.Request.Context(), auth0ID, req.Email, req.Name, req.Surname, req.CategoryID, req.ProfilePhotoFileID)
	if err != nil {
		handleRegisterProviderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, providerSummaryResponseFromDomain(*createdProvider))
}

func (h *ProviderHandler) FilterProvidersByCategory(c *gin.Context) {
	categoryID, err := categoryIDFromQuery(c)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	providers, err := h.providerService.FilterProvidersByCategoryID(c.Request.Context(), categoryID)
	if err != nil {
		handleFilterProvidersError(c, err)
		return
	}

	c.JSON(http.StatusOK, providerSummaryResponsesFromDomain(providers))
}

func categoryIDFromQuery(c *gin.Context) (int, error) {
	categoryID, err := strconv.Atoi(strings.TrimSpace(c.Query("category_id")))
	if err != nil || categoryID <= 0 {
		return 0, category.ErrIDRequired
	}

	return categoryID, nil
}
