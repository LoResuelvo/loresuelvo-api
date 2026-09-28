package operation_chat_handler

import (
	"encoding/json"
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
	router.GET("/admin/operations/:operation_id/conversation", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth0|support") }, newTestHandler(service).Get)
	return router
}
func TestGetMapsMessagesWithoutPrivateAttachmentMetadata(t *testing.T) {
	service := &chatServiceMock{}
	requestID := 12
	found := &readmodel.OperationChat{OperationID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 12}, ConversationAssociation: readmodel.ConversationAssociation{ConversationID: 9, JobRequestID: &requestID}, Messages: []conversation.Message{{ID: 4, SenderRole: "consumer", Content: "Help", Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: "private-file", OriginalName: "private-name"}}}, CreatedOn: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}}}
	service.On("Query", mock.Anything, "jr-12", "auth0|support", "chat-request", "Investigate", operation.ChatQuery{}).Return(found, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation", nil)
	request.Header.Set("X-Audit-Reason", "Investigate")
	request.Header.Set("X-Request-ID", "chat-request")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"next_cursor":null,"operation_id":"jr-12","conversation_id":9,"job_request_id":12,"service_proposal_id":null,"related_service_proposal_ids":[],"shared_conversation":false,"messages":[{"id":4,"sender_role":"consumer","content":"Help","created_on":"2026-09-20T12:00:00Z"}]}`, response.Body.String())
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
			service.On("Query", mock.Anything, "jr-12", "auth0|support", mock.Anything, "reason", operation.ChatQuery{}).Return(nil, test.err).Once()
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
	service.On("Query", mock.Anything, "jr-12", "auth0|support", mock.Anything, "", operation.ChatQuery{}).Return(nil, operation.ErrInvalidChatReason).Once()
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
	service.On("Query", mock.Anything, "sp-22", "auth0|support", "chat-shared-request", "Investigate", operation.ChatQuery{}).Return(found, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/sp-22/conversation", nil)
	request.Header.Set("X-Audit-Reason", "Investigate")
	request.Header.Set("X-Request-ID", "chat-shared-request")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"next_cursor":null,"operation_id":"sp-22","conversation_id":9,"job_request_id":12,"service_proposal_id":22,"related_service_proposal_ids":[21,22],"shared_conversation":true,"messages":[{"id":4,"sender_role":"consumer","content":"First proposal","created_on":"2026-09-20T13:05:00Z"},{"id":5,"sender_role":"consumer","content":"Second proposal","created_on":"2026-09-21T13:05:00Z"}]}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestGetReturnsEmptyMessagesAndNoNextCursor(t *testing.T) {
	service := &chatServiceMock{}
	requestID := 12
	found := &readmodel.OperationChat{OperationID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: requestID}, ConversationAssociation: readmodel.ConversationAssociation{ConversationID: 9, JobRequestID: &requestID}}
	service.On("Query", mock.Anything, "jr-12", "auth0|support", mock.Anything, "Check empty chat", operation.ChatQuery{}).Return(found, nil).Once()
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation", nil)
	request.Header.Set("X-Audit-Reason", "Check empty chat")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"messages":[]`)
	require.Contains(t, response.Body.String(), `"next_cursor":null`)
	service.AssertExpectations(t)
}

func TestGetRejectsRequestChosenConversationBeforeQueryingChat(t *testing.T) {
	service := &chatServiceMock{}
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation?conversation_id=99", nil)
	request.Header.Set("X-Audit-Reason", "Investigate request")
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.JSONEq(t, `{"error":"invalid chat query"}`, response.Body.String())
	require.Empty(t, service.Calls)
}

func newTestHandler(service service) *Handler {
	handler, err := NewHandler(service, []byte("test-audit-cursor-signing-key-2026-keep-private"))
	if err != nil {
		panic(err)
	}
	return handler
}

func TestGetSignsNextCursorAndContinuesWithInheritedLimit(t *testing.T) {
	service := &chatServiceMock{}
	position := conversation.MessagePosition{ID: 4, CreatedOn: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}
	found := &readmodel.OperationChat{OperationID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 12}, ConversationAssociation: readmodel.ConversationAssociation{ConversationID: 9}, Messages: []conversation.Message{{ID: 4, Content: "Help", CreatedOn: position.CreatedOn}}, Next: &position}
	service.On("Query", mock.Anything, "jr-12", "auth0|support", "page-1", "Support case", operation.ChatQuery{Limit: 2}).Return(found, nil).Once()
	service.On("Query", mock.Anything, "jr-12", "auth0|support", "page-2", "Support case", operation.ChatQuery{Limit: 2, ConversationID: 9, After: &position}).Return(&readmodel.OperationChat{OperationID: found.OperationID, ConversationAssociation: found.ConversationAssociation, Messages: []conversation.Message{{ID: 5, CreatedOn: position.CreatedOn}}}, nil).Once()
	router := chatRouter(service)
	first := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation?limit=2", nil)
	first.Header.Set("X-Audit-Reason", "Support case")
	first.Header.Set("X-Request-ID", "page-1")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, first)
	require.Equal(t, 200, response.Code)
	var page struct {
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.NotNil(t, page.NextCursor)
	second := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation?cursor="+*page.NextCursor, nil)
	second.Header.Set("X-Audit-Reason", "Support case")
	second.Header.Set("X-Request-ID", "page-2")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, second)
	require.Equal(t, 200, response.Code)
	page.NextCursor = nil
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.Nil(t, page.NextCursor)
	service.AssertExpectations(t)
}

func TestGetRejectsInvalidPagingBeforeService(t *testing.T) {
	service := &chatServiceMock{}
	router := chatRouter(service)
	for _, query := range []string{"limit=0", "cursor=invalid", "limit=2&limit=2", "unknown=1"} {
		request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation?"+query, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code)
	}
	require.Empty(t, service.Calls)
}

func TestGetDoesNotTakeReasonFromQuery(t *testing.T) {
	service := &chatServiceMock{}
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-12/conversation?reason=Investigate", nil)
	response := httptest.NewRecorder()
	chatRouter(service).ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.NotContains(t, response.Body.String(), "messages")
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestChatResponseMapsSafeResolvedMediaOnly(t *testing.T) {
	message := conversation.Message{ID: 1, Content: "Private", Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: "image", OriginalName: "photo.jpg", URL: "https://storage.invalid/signed-image"}, Description: "Evidence"}}, Audio: &filedomain.MessageAudio{FileID: "audio", OriginalName: "audio.ogg", URL: "https://storage.invalid/signed-audio", MimeType: "audio/ogg", Codec: "opus", DurationSeconds: 4}, Video: &filedomain.MessageVideo{FileID: "video", OriginalName: "video.mp4", URL: "https://storage.invalid/signed-video", MimeType: "video/mp4", VideoCodec: "h264", DurationSeconds: 5, Width: 640, Height: 480}}
	response := responseFromDomain(&readmodel.OperationChat{OperationID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 1}, Messages: []conversation.Message{message}})
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	mediaMessage := decoded["messages"].([]any)[0].(map[string]any)
	require.JSONEq(t, `{"id":"image","url":"https://storage.invalid/signed-image","original_name":"photo.jpg","description":"Evidence"}`, string(mustJSON(t, mediaMessage["images"].([]any)[0])))
	require.Contains(t, mediaMessage, "audio")
	require.Contains(t, mediaMessage, "video")
	for _, private := range []string{"bucket", "key", "uploaded_by_auth_id", "visibility", "credentials"} {
		require.NotContains(t, string(encoded), `"`+private+`"`)
	}
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}
