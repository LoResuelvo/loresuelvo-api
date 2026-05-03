package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// setupRouter configura la ruta principal
func setupRouter() *gin.Engine {
	r := gin.Default()

	// Ruta raíz que devuelve "Hola mundo" en texto plano
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hola mundo")
	})

	return r
}

func main() {
	r := setupRouter()
	r.Run(":8080")
}
