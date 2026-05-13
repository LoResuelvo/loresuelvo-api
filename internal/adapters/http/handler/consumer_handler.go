package handler

import (
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/gin-gonic/gin"
)

type ConsumerHandler struct {
	consumerService *consumer.Service
}

type registerConsumerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Password string `json:"password"`
}

func NewConsumerHandler(consumerService *consumer.Service) *ConsumerHandler {
	return &ConsumerHandler{
		consumerService: consumerService,
	}
}

func (h *ConsumerHandler) RegisterConsumer(c *gin.Context) {
	var req registerConsumerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.Password = strings.TrimSpace(req.Password)

	// si es un consumidor -> lo registró
	// depende el tipo de error, se devuelve un status code diferente
	if err := h.consumerService.RegisterConsumer(req.Email, req.Name, req.Surname, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}) // cambiar a un error user friendly?
		return
	}

	// devuelvo en json los campos del nuevo consumidor (DTO)
	c.JSON(http.StatusCreated, gin.H{"message": "cuenta registrada exitosamente"})
}
