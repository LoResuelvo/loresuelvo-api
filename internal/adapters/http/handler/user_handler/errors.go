package user_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/gin-gonic/gin"
)

func handleGetCurrentUserError(c *gin.Context, err error) {
	if err == user.ErrNotFound {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}
