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
	require.JSONEq(t, `{"operation_id":"jr-12","conversation_id":9,"job_request_id":12,"service_proposal_id":null,"related_service_proposal_ids":[],"shared_conversation":false,"messages":[{"id":4,"sender_role":"consumer","content":"Help","created_on":"2026-09-20T12:00:00Z"}]}`, response.Body.String())
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

func TestGetIdentifiesSharedConversationWithoutAssigningMessagesToProposal(t *testing.T) {
	service := &chatServiceMock{}
	requestID, firstProposalID, selectedProposalID := 12, 21, 22
	found := &readmodel.OperationChat{
		OperationID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: selectedProposalID},
		ConversationAssociation: readmodel.ConversationAssociation{
			ConversationID: 9, JobRequestID: &requestID, ServiceProposalID: &selectedProposalID,
			RelatedServiceProposalIDs: []int{firstProposalID, selectedProposalID},
		},
		Messages: []conversation.Message{
			{ID: 4, ConversationID: 9, SenderRole: "consumer", Content: "First proposal", CreatedOn: time.Date(2026, 9, 20, 13, 5, 0, 0, time.UTC)},
			{ID: 5, ConversationID: 9, SenderRole: "consumer", Content: "Second proposal", CreatedOn: time.Date(2026, 9, 21, 13, 5, 0, 0, time.UTC)},
		},
	}
	service.On("Query", mock.Anything, "sp-22", "auth0|support", "chat-shared-request", "Investigate").Return(found, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/sp-22/conversation", nil)
	request.Header.Set("X-Audit-Reason", "Investigate")
	request.Header.Set("X-Request-ID", "chat-shared-request")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"operation_id":"sp-22","conversation_id":9,"job_request_id":12,"service_proposal_id":22,"related_service_proposal_ids":[21,22],"shared_conversation":true,"messages":[{"id":4,"sender_role":"consumer","content":"First proposal","created_on":"2026-09-20T13:05:00Z"},{"id":5,"sender_role":"consumer","content":"Second proposal","created_on":"2026-09-21T13:05:00Z"}]}`, response.Body.String())
	service.AssertExpectations(t)
}
