package payment_account_handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlePaymentAccountErrorDoesNotExposeInternalWrappingContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	handlePaymentAccountError(
		context,
		fmt.Errorf("exchanging payment account authorization code: %w", paymentaccount.ErrAuthorizationGrantUnavailable),
	)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, paymentaccount.ErrAuthorizationGrantUnavailable.Error(), response["error"])
}
