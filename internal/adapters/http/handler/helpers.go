package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
)

var amountRegex = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

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

func ParseAmountToCents(value string) (int64, error) {
	value = strings.TrimSpace(value)

	if !amountRegex.MatchString(value) {
		return 0, fmt.Errorf("amount must be a positive decimal with at most two decimal places")
	}

	parts := strings.Split(value, ".")

	units, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount")
	}

	cents := int64(0)

	if len(parts) == 2 {
		decimalPart := parts[1]

		if len(decimalPart) == 1 {
			decimalPart += "0"
		}

		cents, err = strconv.ParseInt(decimalPart, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount decimals")
		}
	}

	return units*100 + cents, nil
}
