package steps_test

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

func registerHelloWorldSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que el sistema esta iniciado$`, s.queElSistemaEstaIniciado)
	sc.Step(`^solicito el saludo en la ruta raiz$`, s.solicitoElSaludoEnLaRutaRaiz)
	sc.Step(`^la respuesta debe ser "([^"]*)" con un codigo (\d+)$`, s.laRespuestaDebeSerConUnCodigo)
}

// No need to start anything — the server is already up in testSuite
func (s *testSuite) queElSistemaEstaIniciado() error {
	if s.server == nil {
		return fmt.Errorf("el servidor de pruebas no fue inicializado correctamente")
	}
	return nil
}

func (s *testSuite) solicitoElSaludoEnLaRutaRaiz() error {
	// s.server.URL already has the right host+port — no hardcoded localhost:8080
	resp, err := http.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("fallo la conexión a la API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("fallo leyendo el cuerpo de la respuesta: %w", err)
	}

	s.lastStatus = resp.StatusCode
	s.lastBody = body
	return nil
}

func (s *testSuite) laRespuestaDebeSerConUnCodigo(mensaje string, codigo int) error {
	if s.lastStatus != codigo {
		return fmt.Errorf("se esperaba código %d, pero se obtuvo %d", codigo, s.lastStatus)
	}
	if string(s.lastBody) != mensaje {
		return fmt.Errorf("se esperaba '%s', pero se obtuvo '%s'", mensaje, string(s.lastBody))
	}
	return nil
}
