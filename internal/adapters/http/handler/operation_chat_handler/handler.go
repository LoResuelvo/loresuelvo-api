package operation_chat_handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, id, authSubject, correlationID, reason string, query operation.ChatQuery) (*readmodel.OperationChat, error)
}
type Handler struct {
	service service
	cursors *signedcursor.Codec
}

func NewHandler(service service, key []byte) (*Handler, error) {
	codec, err := signedcursor.New(key, chatCursorPurpose)
	if err != nil {
		return nil, err
	}
	return &Handler{service: service, cursors: codec}, nil
}

func (handler *Handler) Get(c *gin.Context) {
	authSubject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlationID, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	query, err := parseQuery(c.Request.URL.RawQuery, c.Param("operation_id"), handler.cursors)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid chat query")
		return
	}
	found, err := handler.service.Query(c.Request.Context(), c.Param("operation_id"), authSubject, correlationID, c.GetHeader("X-Audit-Reason"), query)
	switch {
	case errors.Is(err, operation.ErrInvalidOperationID):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid operation ID")
	case errors.Is(err, operation.ErrInvalidChatQuery):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid chat query")
	case errors.Is(err, operation.ErrInvalidChatReason):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid audit reason")
	case errors.Is(err, operation.ErrOperationNotFound):
		httphandler.RespondError(c, http.StatusNotFound, "operation not found")
	case err != nil:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	default:
		response := responseFromDomain(found)
		if found.Next != nil {
			token, err := handler.cursors.Encode(cursorPayload{Version: chatCursorVersion, OperationID: response.OperationID, ConversationID: found.ConversationID, Limit: query.EffectiveLimit(), After: cursorPosition{CreatedOn: found.Next.CreatedOn, ID: found.Next.ID}})
			if err != nil {
				httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
				return
			}
			response.NextCursor = &token
		}
		c.JSON(http.StatusOK, response)
	}
}

type chatResponse struct {
	OperationID               string            `json:"operation_id"`
	ConversationID            int               `json:"conversation_id"`
	JobRequestID              *int              `json:"job_request_id"`
	ServiceProposalID         *int              `json:"service_proposal_id"`
	RelatedServiceProposalIDs []int             `json:"related_service_proposal_ids"`
	SharedConversation        bool              `json:"shared_conversation"`
	Messages                  []messageResponse `json:"messages"`
	NextCursor                *string           `json:"next_cursor"`
}
type messageResponse struct {
	Images     []imageResponse `json:"images,omitempty"`
	Audio      *audioResponse  `json:"audio,omitempty"`
	Video      *videoResponse  `json:"video,omitempty"`
	ID         int             `json:"id"`
	SenderRole string          `json:"sender_role"`
	Content    string          `json:"content"`
	CreatedOn  time.Time       `json:"created_on"`
}

func responseFromDomain(found *readmodel.OperationChat) chatResponse {
	relatedProposalIDs := append(make([]int, 0, len(found.RelatedServiceProposalIDs)), found.RelatedServiceProposalIDs...)
	response := chatResponse{RelatedServiceProposalIDs: relatedProposalIDs, SharedConversation: found.IsShared(), OperationID: string(found.OperationID.Kind) + "-" + strconv.Itoa(found.OperationID.ResourceID), ConversationID: found.ConversationID, JobRequestID: found.JobRequestID, ServiceProposalID: found.ServiceProposalID, Messages: make([]messageResponse, 0, len(found.Messages))}
	for _, message := range found.Messages {
		mapped := messageResponse{ID: message.ID, SenderRole: message.SenderRole, Content: message.Content, CreatedOn: message.CreatedOn.UTC()}
		mediaFromDomain(message, &mapped)
		response.Messages = append(response.Messages, mapped)
	}
	return response
}
