package steps_test

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

func registerHelloWorldSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que el sistema esta iniciado$`, s.systemIsStarted)
	sc.Step(`^solicito el saludo en la ruta raiz$`, s.requestRootGreeting)
	sc.Step(`^la respuesta debe ser "([^"]*)" con un codigo (\d+)$`, s.responseShouldBeWithStatusCode)
}

// No need to start anything — the server is already up in testSuite.
func (s *testSuite) systemIsStarted() error {
	if s.server == nil {
		return fmt.Errorf("test server was not initialized correctly")
	}
	return nil
}

func (s *testSuite) requestRootGreeting() error {
	// s.server.URL already has the right host+port — no hardcoded localhost:8080.
	resp, err := http.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	s.lastStatus = resp.StatusCode
	s.lastBody = body
	return nil
}

func (s *testSuite) responseShouldBeWithStatusCode(message string, statusCode int) error {
	if s.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d", statusCode, s.lastStatus)
	}
	if string(s.lastBody) != message {
		return fmt.Errorf("expected %q, got %q", message, string(s.lastBody))
	}
	return nil
}
