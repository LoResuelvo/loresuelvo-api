package category_handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
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

	createdCategory, err := h.categoryService.CreateCategory(req.Name)
	if err != nil {
		handleCreateCategoryError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/categories/%d", createdCategory.ID))
	c.JSON(http.StatusCreated, categoryResponseFromDomain(*createdCategory))
}

func (h *CategoryHandler) ListCategories(c *gin.Context) {
	categories, err := h.categoryService.ListCategories()
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	c.JSON(http.StatusOK, categoryListItemResponsesFromDomain(categories))
}
