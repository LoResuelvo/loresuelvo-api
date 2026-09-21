package category_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/gin-gonic/gin"
)

var (
	errCategoryNameMustBeText = errors.New("Category name must be text")
	errInvalidRequestBody     = errors.New("Invalid request body")
)

func handleCreateCategoryError(c *gin.Context, err error) {
	if errors.Is(err, category.ErrAlreadyExists) {
		httphandler.RespondError(c, http.StatusConflict, category.ErrAlreadyExists.Error())
		return
	}

	if errors.Is(err, category.ErrNameRequired) {
		httphandler.RespondError(c, http.StatusBadRequest, category.ErrNameRequired.Error())
		return
	}
	if errors.Is(err, category.ErrNameTooLong) {
		httphandler.RespondError(c, http.StatusBadRequest, category.ErrNameTooLong.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}
