package handler

import (
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
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
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	currentUser, err := h.userService.GetCurrentUser(auth0ID)
	if err == user.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, currentUser)
}
