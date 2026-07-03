package consumer_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/gin-gonic/gin"
)

func handleRegisterConsumerError(c *gin.Context, err error) {
	if err == validator.ErrEmailAlreadyRegistered {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusBadRequest, err.Error())
}
