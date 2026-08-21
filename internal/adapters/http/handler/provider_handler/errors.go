package provider_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

func handleRegisterProviderError(c *gin.Context, err error) {
	if errors.Is(err, validator.ErrEmailAlreadyRegistered) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	if errors.Is(err, filedomain.ErrProfilePhotoRequired) || errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if errors.Is(err, category.ErrIDRequired) || errors.Is(err, category.ErrDoesNotExist) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if errors.Is(err, coveragezone.ErrAtLeastOneRequired) ||
		errors.Is(err, coveragezone.ErrDoesNotExist) ||
		errors.Is(err, coveragezone.ErrNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleFilterProvidersError(c *gin.Context, err error) {
	if errors.Is(err, category.ErrIDRequired) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if errors.Is(err, category.ErrDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleGetProviderProfileError(c *gin.Context, err error) {
	if errors.Is(err, provider.ErrDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}
