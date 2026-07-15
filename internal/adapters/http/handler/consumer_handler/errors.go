package consumer_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

func handleRegisterConsumerError(c *gin.Context, err error) {
	if errors.Is(err, validator.ErrEmailAlreadyRegistered) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusBadRequest, err.Error())
}
