package conversation_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/gin-gonic/gin"
)

func handleGetConversationError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationRequiresAcceptance) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationMessageLimitReached) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleSendMessageError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrMessageImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationRequiresAcceptance) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationMessageLimitReached) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleListWorkConversationsError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleListChatbotConversationsError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrOnlyConsumerCanListChatbotConversations) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleCreateChatbotConversationError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrChatbotResponseRequired) || errors.Is(err, conversation.ErrMessageImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrOnlyConsumerCanMessageChatbot) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleContinueChatbotConversationError(c *gin.Context, err error) {
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrChatbotResponseRequired) || errors.Is(err, conversation.ErrMessageImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrOnlyConsumerCanMessageChatbot) || errors.Is(err, conversation.ErrConversationAccessDenied) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrChatbotConversationAlreadyProcessing) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}
