package router

import (
	"net/http"
	"strings"

	usecases "github.com/LoResuelvo/loresuelvo-api/app/use_cases"
	"github.com/gin-gonic/gin"
)

type registerConsumerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Password string `json:"password"`
}

func SetupRouter() *gin.Engine {
	r := gin.Default()
	consumerManager := usecases.NewConsumerManager(nil)

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World")
	})

	r.POST("/consumers", func(c *gin.Context) {
		var req registerConsumerRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datos de registro inválidos"})
			return
		}

		req.Email = strings.TrimSpace(req.Email)
		req.Name = strings.TrimSpace(req.Name)
		req.Surname = strings.TrimSpace(req.Surname)
		req.Password = strings.TrimSpace(req.Password)

		// si es un consumidor -> lo registró
		// depende el tipo de error, se devuelve un status code diferente
		if err := consumerManager.RegisterConsumer(req.Email, req.Name, req.Surname, req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datos de registro inválidos"})
			return
		}

		// devuelvo en json los campos del nuevo consumidor (DTO)
		c.JSON(http.StatusCreated, gin.H{"message": "cuenta registrada exitosamente"})
	})

	return r
}
