package operation_chat_handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, id, authSubject, correlationID, reason string) (*readmodel.OperationChat, error)
}
type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

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
	// This slice exposes only the bounded initial page, not arbitrary selection.
	if len(c.Request.URL.Query()) != 0 {
		httphandler.RespondError(c, http.StatusBadRequest, "unsupported query parameter")
		return
	}
	found, err := handler.service.Query(c.Request.Context(), c.Param("operation_id"), authSubject, correlationID, c.GetHeader("X-Audit-Reason"))
	switch {
	case errors.Is(err, operation.ErrInvalidOperationID):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid operation ID")
	case errors.Is(err, operation.ErrInvalidChatReason):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid audit reason")
	case errors.Is(err, operation.ErrOperationNotFound):
		httphandler.RespondError(c, http.StatusNotFound, "operation not found")
	case err != nil:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	default:
		c.JSON(http.StatusOK, responseFromDomain(found))
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
}
type messageResponse struct {
	ID         int       `json:"id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedOn  time.Time `json:"created_on"`
}

func responseFromDomain(found *readmodel.OperationChat) chatResponse {
	relatedProposalIDs := append(make([]int, 0, len(found.RelatedServiceProposalIDs)), found.RelatedServiceProposalIDs...)
	response := chatResponse{RelatedServiceProposalIDs: relatedProposalIDs, SharedConversation: found.IsShared(), OperationID: string(found.OperationID.Kind) + "-" + strconv.Itoa(found.OperationID.ResourceID), ConversationID: found.ConversationID, JobRequestID: found.JobRequestID, ServiceProposalID: found.ServiceProposalID, Messages: make([]messageResponse, 0, len(found.Messages))}
	for _, message := range found.Messages {
		response.Messages = append(response.Messages, messageResponse{ID: message.ID, SenderRole: message.SenderRole, Content: message.Content, CreatedOn: message.CreatedOn.UTC()})
	}
	return response
}
