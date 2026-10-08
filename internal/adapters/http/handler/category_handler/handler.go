package category_handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	categoryService *category.Service
}

func NewCategoryHandler(categoryService *category.Service) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req createCategoryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		var typeError *json.UnmarshalTypeError
		if errors.As(err, &typeError) && typeError.Field == "name" {
			httphandler.RespondError(c, http.StatusBadRequest, errCategoryNameMustBeText.Error())
			return
		}

		httphandler.RespondError(c, http.StatusBadRequest, errInvalidRequestBody.Error())
		return
	}

	authSubject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlationID, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	createdCategory, err := h.categoryService.CreateCategory(c.Request.Context(), req.Name, authSubject, correlationID)
	if err != nil {
		handleCreateCategoryError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/categories/%d", createdCategory.ID))
	c.JSON(http.StatusCreated, categoryResponseFromDomain(*createdCategory))
}

func (h *CategoryHandler) ListCategories(c *gin.Context) {
	includeDisabled := false
	if value, exists := c.GetQuery("include_disabled"); exists {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			httphandler.RespondError(c, http.StatusBadRequest, "Invalid include_disabled")
			return
		}
		includeDisabled = parsed
	}
	if includeDisabled {
		c.Header("Cache-Control", "private, no-store")
		categories, err := h.categoryService.ListAllCategories()
		if err != nil {
			handleCategoryError(c, err)
			return
		}
		result := make([]administrativeCategoryResponse, 0, len(categories))
		for _, current := range categories {
			result = append(result, administrativeCategoryFromDomain(current))
		}
		c.JSON(http.StatusOK, result)
		return
	}
	categories, err := h.categoryService.ListCategories()
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	c.JSON(http.StatusOK, categoryListItemResponsesFromDomain(categories))
}
