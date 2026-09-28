package operation_chat_handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func chatRouter(service *chatServiceMock) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.GET("/admin/operations/:operation_id/conversation", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth0|support") }, NewHandler(service).Get)
	return router
}
func TestGetMapsMessagesWithoutPrivateAttachmentMetadata(t *testing.T) {
	service := &chatServiceMock{}
	requestID := 12
	found := &readmodel.OperationChat{OperationID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 12}, ConversationAssociation: readmodel.ConversationAssociation{ConversationID: 9, JobRequestID: &requestID}, Messages: []conversation.Message{{ID: 4, SenderRole: "consumer", Content: "Help", Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: "private-file", OriginalName: "private-name"}}}, CreatedOn: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}}}
	service.On("Query", mock.Anything, "jr-12", "auth0|support", "chat-request", "Investigate").Return(found, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation", nil)
	request.Header.Set("X-Audit-Reason", "Investigate")
	request.Header.Set("X-Request-ID", "chat-request")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"operation_id":"jr-12","conversation_id":9,"job_request_id":12,"service_proposal_id":null,"messages":[{"id":4,"sender_role":"consumer","content":"Help","created_on":"2026-09-20T12:00:00Z"}]}`, response.Body.String())
	service.AssertExpectations(t)
}
func TestGetMapsDomainErrorsWithoutLeakingChat(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{{"reason", operation.ErrInvalidChatReason, 400}, {"identifier", operation.ErrInvalidOperationID, 400}, {"absent", operation.ErrOperationNotFound, 404}, {"audit", errors.New("secret chat store failure"), 500}} {
		t.Run(test.name, func(t *testing.T) {
			service := &chatServiceMock{}
			service.On("Query", mock.Anything, "jr-12", "auth0|support", mock.Anything, "reason").Return(nil, test.err).Once()
			request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation", nil)
			request.Header.Set("X-Audit-Reason", "reason")
			response := httptest.NewRecorder()
			chatRouter(service).ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
			require.NotContains(t, response.Body.String(), "messages")
			require.NotContains(t, response.Body.String(), "secret")
			service.AssertExpectations(t)
		})
	}
}
func TestGetDoesNotTakeReasonFromGETBody(t *testing.T) {
	service := &chatServiceMock{}
	service.On("Query", mock.Anything, "jr-12", "auth0|support", mock.Anything, "").Return(nil, operation.ErrInvalidChatReason).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation", strings.NewReader(`{"reason":"body reason"}`))
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, 400, response.Code)
	service.AssertExpectations(t)
}
