package file_handler

import (
	"fmt"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestClaimEvidenceErrorMapperDoesNotLeakWrappedDetails(t *testing.T) {
	for _, tc := range []struct {
		err     error
		status  int
		message string
	}{
		{fmt.Errorf("private detail: %w", filedomain.ErrClaimEvidenceImageNotAvailable), 400, filedomain.ErrClaimEvidenceImageNotAvailable.Error()},
		{fmt.Errorf("private storage failure"), 500, "internal server error"},
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		handleFileError(ctx, tc.err)
		require.Equal(t, tc.status, recorder.Code)
		require.JSONEq(t, fmt.Sprintf(`{"error":%q}`, tc.message), recorder.Body.String())
	}
}
