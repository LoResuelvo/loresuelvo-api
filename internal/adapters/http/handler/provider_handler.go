package handler

import (
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

type ProviderHandler struct {
	providerService *provider.Service
}

type registerProviderRequest struct {
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Surname      string   `json:"surname"`
	CategoryID   int      `json:"category_id"`
	CoverageZone []string `json:"coverage_zone"`
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

	err := h.providerService.RegisterProvider(auth0ID, req.Email, req.Name, req.Surname, req.CategoryID)
	if err == validator.ErrEmailAlreadyRegistered {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "cuenta registrada exitosamente"})
}
