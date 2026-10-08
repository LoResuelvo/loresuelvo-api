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

func handleCategoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, category.ErrDoesNotExist):
		httphandler.RespondError(c, http.StatusNotFound, category.ErrDoesNotExist.Error())
	case errors.Is(err, category.ErrVersionConflict):
		httphandler.RespondError(c, http.StatusConflict, category.ErrVersionConflict.Error())
	case errors.Is(err, category.ErrConfirmationRequired):
		httphandler.RespondError(c, http.StatusConflict, category.ErrConfirmationRequired.Error())
	case errors.Is(err, category.ErrAlreadyExists):
		httphandler.RespondError(c, http.StatusConflict, category.ErrAlreadyExists.Error())
	case errors.Is(err, category.ErrVersionRequired), errors.Is(err, category.ErrNoChanges), errors.Is(err, category.ErrInvalidReason), errors.Is(err, category.ErrIDRequired), errors.Is(err, category.ErrNameRequired), errors.Is(err, category.ErrNameTooLong):
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
}
