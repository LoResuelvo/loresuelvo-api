package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
)

func GetAuthenticatedUserID(c *gin.Context) (string, bool) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		RespondError(c, http.StatusUnauthorized, "missing user id")
		return "", false
	}
	return auth0ID, true
}

func RespondError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{"error": message})
}

func PositiveIDFromString(value string, fieldName string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", fieldName)
	}

	return id, nil
}
