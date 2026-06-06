package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

type ProviderHandler struct {
	providerService *provider.Service
}

type registerProviderRequest struct {
	Email              string   `json:"email"`
	Name               string   `json:"name"`
	Surname            string   `json:"surname"`
	CategoryID         int      `json:"category_id"`
	CoverageZone       []string `json:"coverage_zone"`
	ProfilePhotoFileID string   `json:"profile_photo_file_id"`
}

type providerSummaryResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}

func NewProviderHandler(providerService *provider.Service) *ProviderHandler {
	return &ProviderHandler{
		providerService: providerService,
	}
}

func (h *ProviderHandler) RegisterProvider(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req registerProviderRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)

	err := h.providerService.RegisterProvider(c.Request.Context(), auth0ID, req.Email, req.Name, req.Surname, req.CategoryID, strings.TrimSpace(req.ProfilePhotoFileID))
	if err == validator.ErrEmailAlreadyRegistered {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err == filedomain.ErrProfilePhotoRequired || err == filedomain.ErrProfilePhotoNotAvailable {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "cuenta registrada exitosamente"})
}

func (h *ProviderHandler) FilterProvidersByCategory(c *gin.Context) {
	categoryID, err := categoryIDFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	providers, err := h.providerService.FilterProvidersByCategoryID(c.Request.Context(), categoryID)
	if err == category.ErrIDRequired {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err == category.ErrDoesNotExist {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]providerSummaryResponse, 0, len(providers))
	for _, provider := range providers {
		response = append(response, providerSummaryResponse{
			ID:              provider.ID,
			Name:            provider.Name,
			Surname:         provider.Surname,
			CategoryName:    provider.CategoryName,
			ProfilePhotoURL: provider.ProfilePhotoURL,
		})
	}

	c.JSON(http.StatusOK, response)
}

func categoryIDFromQuery(c *gin.Context) (int, error) {
	categoryID, err := strconv.Atoi(strings.TrimSpace(c.Query("category_id")))
	if err != nil || categoryID <= 0 {
		return 0, category.ErrIDRequired
	}

	return categoryID, nil
}
