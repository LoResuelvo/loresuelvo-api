package user_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *user.Service
}

func NewUserHandler(userService *user.Service) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	currentUser, err := h.userService.GetCurrentUser(c.Request.Context(), auth0ID)
	if err != nil {
		handleGetCurrentUserError(c, err)
		return
	}

	response, err := currentUserResponseFromDomain(currentUser)
	if err != nil {
		handleGetCurrentUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
