package handler

import (
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	categoryService *category.Service
}

type createCategoryRequest struct {
	Name string `json:"name"`
}

func NewCategoryHandler(categoryService *category.Service) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req createCategoryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.categoryService.CreateCategory(req.Name)
	if err == category.ErrAlreadyExists {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "rubro creado exitosamente"})
}
