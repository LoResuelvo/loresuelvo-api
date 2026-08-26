package consumer_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

func handleRegisterConsumerError(c *gin.Context, err error) {
	if errors.Is(err, validator.ErrEmailAlreadyRegistered) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, consumer.ErrAddressServiceUnavailable) {
		httphandler.RespondError(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	if errors.Is(err, consumer.ErrAddressResolverNotConfigured) || errors.Is(err, consumer.ErrCoverageZoneResolverNotConfigured) {
		httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if errors.Is(err, consumer.ErrAddressRequired) ||
		errors.Is(err, consumer.ErrStreetRequired) ||
		errors.Is(err, consumer.ErrStreetNumberRequired) ||
		errors.Is(err, consumer.ErrAddressFieldTooLong) ||
		errors.Is(err, consumer.ErrAddressNotValidated) ||
		errors.Is(err, consumer.ErrCoverageZoneNotAvailable) ||
		errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) ||
		errors.Is(err, validator.ErrInvalidEmailFormat) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, "Internal server error")
}
