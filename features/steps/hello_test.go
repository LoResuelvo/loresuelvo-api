package steps

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	usecases "github.com/LoResuelvo/loresuelvo-api/app/use_cases"
	"github.com/LoResuelvo/loresuelvo-api/pkg/router"
	"github.com/cucumber/godog"
)

var startAPIServerOnce sync.Once

func queElSistemaEstaIniciado(ctx *testContext) error {
	startAPIServerOnce.Do(func() {
		// Encendemos la API en una goroutine (hilo aparte) para no bloquear el test
		go func() {
			r := router.NewRouter()
			consumerManager := usecases.NewConsumerManager(ctx.consumerRepository)
			router.RegisterConsumerRoutes(r, consumerManager)
			// Usamos un puerto estándar para pruebas
			_ = r.Run(":8080")
		}()
	})

	// Damos un respiro de 500ms para que el servidor levante
	time.Sleep(500 * time.Millisecond)
	return nil
}

func solicitoElSaludoEnLaRutaRaiz(ctx *testContext) error {
	// ¡Petición HTTP real a localhost!
	resp, err := http.Get("http://localhost:8080/")
	if err != nil {
		return fmt.Errorf("fallo la conexión a la API: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	ctx.responseBody = string(body)
	ctx.statusCode = resp.StatusCode
	return nil
}

func laRespuestaDebeSerConUnCodigo(ctx *testContext, mensaje string, codigo int) error {
	if ctx.statusCode != codigo {
		return fmt.Errorf("se esperaba código %d, pero se obtuvo %d", codigo, ctx.statusCode)
	}
	if ctx.responseBody != mensaje {
		return fmt.Errorf("se esperaba '%s', pero se obtuvo '%s'", mensaje, ctx.responseBody)
	}
	return nil
}

func registrarPasosDeHello(ctx *godog.ScenarioContext, tCtx *testContext) {
	ctx.Step(`^que el sistema esta iniciado$`, func() error {
		return queElSistemaEstaIniciado(tCtx)
	})
	ctx.Step(`^solicito el saludo en la ruta raiz$`, func() error {
		return solicitoElSaludoEnLaRutaRaiz(tCtx)
	})
	ctx.Step(`^la respuesta debe ser "([^"]*)" con un codigo (\d+)$`, func(msg string, code int) error {
		return laRespuestaDebeSerConUnCodigo(tCtx, msg, code)
	})
}
