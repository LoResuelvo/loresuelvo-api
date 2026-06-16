package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type consumerRegistrationRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type registrationResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func registerConsumerAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^existe un consumidor registrado con correo "([^"]*)"$`, suite.thereIsRegisteredConsumerWithEmail)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)"`, suite.requestConsumerAccountRegistration)
	sc.Step(`^el sistema confirma el registro$`, suite.systemConfirmsRegistration)
	sc.Step(`^el sistema me indica que el formato del correo es inválido$`, suite.systemReportsInvalidEmailFormat)
	sc.Step(`^el sistema me indica que el correo electrónico ya está registrado$`, suite.systemReportsEmailAlreadyRegistered)
	sc.Step(`^la respuesta de registro debe tener un codigo (\d+)$`, suite.registrationResponseShouldHaveStatusCode)
	sc.Step(`^la respuesta de registro debe indicar "([^"]*)"$`, suite.registrationResponseShouldSay)
}

func (suite *testSuite) thereIsRegisteredConsumerWithEmail(email string) error {
	req := consumerRegistrationRequest{
		Email: email,
		Name:  "Existing Consumer",
	}

	resp, err := suite.postConsumerRegistration(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("could not prepare existing consumer: status %d, body %s", resp.StatusCode, string(body))
	}

	return nil
}

func (suite *testSuite) requestConsumerAccountRegistration(email, name, surname string) error {
	resp, err := suite.postConsumerRegistration(consumerRegistrationRequest{
		Email:   email,
		Name:    name,
		Surname: surname,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) systemConfirmsRegistration() error {
	return suite.registrationResponseShouldHaveStatusCode(http.StatusCreated)
}

func (suite *testSuite) systemReportsInvalidEmailFormat() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) systemReportsEmailAlreadyRegistered() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) registrationResponseShouldHaveStatusCode(statusCode int) error {
	if suite.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) registrationResponseShouldSay(message string) error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	if response.Message == message || response.Error == message {
		return nil
	}

	return fmt.Errorf("expected message %q, got body %s", message, string(suite.lastBody))
}

func (suite *testSuite) postConsumerRegistration(req consumerRegistrationRequest) (*http.Response, error) {
	return suite.postConsumerRegistrationWithAuth0ID("auth0|consumer-test", req)
}

func (suite *testSuite) postConsumerRegistrationWithAuth0ID(auth0ID string, req consumerRegistrationRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/consumers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}

	return resp, nil
}
