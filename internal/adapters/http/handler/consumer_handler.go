package handler

import (
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/gin-gonic/gin"
)

type ConsumerHandler struct {
	consumerService *consumer.Service
}

type registerConsumerRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func NewConsumerHandler(consumerService *consumer.Service) *ConsumerHandler {
	return &ConsumerHandler{
		consumerService: consumerService,
	}
}

func (h *ConsumerHandler) RegisterConsumer(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req registerConsumerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)

	err := h.consumerService.RegisterConsumer(auth0ID, req.Email, req.Name, req.Surname)
	if err == consumer.ErrEmailAlreadyRegistered {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "cuenta registrada exitosamente"})
}
