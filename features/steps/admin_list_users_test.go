package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/cucumber/godog"
)

type adminConsumerDirectoryResponse struct {
	ID              int       `json:"id"`
	Role            string    `json:"role"`
	Name            string    `json:"name"`
	Surname         string    `json:"surname"`
	Email           string    `json:"email"`
	ProfilePhotoURL *string   `json:"profile_photo_url"`
	CreatedOn       time.Time `json:"created_on"`
}

func registerAdminListUsersSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen los siguientes consumidores registrados:$`, suite.thereAreRegisteredConsumers)
	sc.Step(`^que no existen consumidores registrados$`, suite.thereAreNoRegisteredConsumers)
	sc.Step(`^que estoy autenticado como administrador "([^"]*)" con el permiso "([^"]*)"$`, suite.iAmAuthenticatedAsAdminWithPermission)
	sc.Step(`^consulto el directorio de consumidores$`, suite.requestConsumerDirectory)
	sc.Step(`^el directorio de consumidores contiene exactamente a:$`, suite.consumerDirectoryContainsExactly)
	sc.Step(`^el directorio de consumidores no incluye a "([^"]*)"$`, suite.consumerDirectoryDoesNotInclude)
	sc.Step(`^el consumidor "([^"]*)" incluye su identificador, rol, nombre, apellido, correo y fecha de registro$`, suite.consumerDirectoryIncludesAdministrativeData)
	sc.Step(`^el consumidor "([^"]*)" incluye la URL pública de su foto de perfil$`, suite.consumerDirectoryIncludesPublicProfilePhotoURL)
	sc.Step(`^el consumidor "([^"]*)" no expone credenciales ni identificadores del proveedor de identidad$`, suite.consumerDirectoryDoesNotExposeSensitiveData)
	sc.Step(`^el sistema devuelve un directorio de consumidores vacío$`, suite.systemReturnsEmptyConsumerDirectory)
}

func (suite *testSuite) thereAreRegisteredConsumers(table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("consumer fixture table must contain a header and at least one consumer")
	}
	if err := requireTableHeaders(table, "correo", "nombre", "apellido"); err != nil {
		return err
	}

	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 3 {
			return fmt.Errorf("expected three consumer fixture columns, got %d", len(row.Cells))
		}
		if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(
			row.Cells[0].Value,
			row.Cells[1].Value,
			row.Cells[2].Value,
		); err != nil {
			return fmt.Errorf("registering consumer %q: %w", row.Cells[0].Value, err)
		}
	}

	return nil
}

func requireTableHeaders(table *godog.Table, expected ...string) error {
	if len(table.Rows) == 0 || len(table.Rows[0].Cells) != len(expected) {
		return fmt.Errorf("expected table headers %v", expected)
	}
	for index, header := range expected {
		if table.Rows[0].Cells[index].Value != header {
			return fmt.Errorf("expected table header %q at column %d, got %q", header, index+1, table.Rows[0].Cells[index].Value)
		}
	}
	return nil
}

func (suite *testSuite) thereAreNoRegisteredConsumers() error {
	return suite.userRepository.DeleteAllOf(consumer.Role)
}

func (suite *testSuite) iAmAuthenticatedAsAdminWithPermission(email, permission string) error {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return fmt.Errorf("administrator permission must not be empty")
	}

	suite.currentAuth0ID = auth0IDForAdminEmail(email)
	suite.currentPermissions = []string{permission}
	return nil
}

func (suite *testSuite) requestConsumerDirectory() error {
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+"/admin/consumers", nil)
	if err != nil {
		return fmt.Errorf("creating consumer directory request: %w", err)
	}
	request.Header.Set(
		"Authorization",
		"Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions),
	)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting consumer directory: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading consumer directory response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) consumerDirectoryContainsExactly(table *godog.Table) error {
	if err := requireTableHeaders(table, "correo"); err != nil {
		return err
	}
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}

	expected := make(map[string]struct{}, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one consumer assertion column, got %d", len(row.Cells))
		}
		expected[row.Cells[0].Value] = struct{}{}
	}
	if len(directory) != len(expected) {
		return fmt.Errorf("expected %d consumers, got %d with body %s", len(expected), len(directory), string(suite.lastBody))
	}
	for _, found := range directory {
		if _, ok := expected[found.Email]; !ok {
			return fmt.Errorf("consumer directory unexpectedly contains %q; body %s", found.Email, string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) consumerDirectoryDoesNotInclude(email string) error {
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	for _, found := range directory {
		if found.Email == email {
			return fmt.Errorf("consumer directory unexpectedly includes %q", email)
		}
	}
	return nil
}

func (suite *testSuite) consumerDirectoryIncludesAdministrativeData(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	found, raw, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	if found.ID <= 0 || found.Role != consumer.Role || strings.TrimSpace(found.Name) == "" ||
		strings.TrimSpace(found.Surname) == "" || found.Email != email || found.CreatedOn.IsZero() {
		return fmt.Errorf("consumer %q has incomplete administrative data: %s", email, string(suite.lastBody))
	}

	expectedFields := map[string]struct{}{
		"id": {}, "role": {}, "name": {}, "surname": {}, "email": {}, "profile_photo_url": {}, "created_on": {},
	}
	if len(raw) != len(expectedFields) {
		return fmt.Errorf("consumer %q has unexpected response fields: %s", email, string(suite.lastBody))
	}
	for field := range expectedFields {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("consumer %q is missing response field %q", email, field)
		}
	}
	return nil
}

func (suite *testSuite) consumerDirectoryIncludesPublicProfilePhotoURL(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	found, _, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	if found.ProfilePhotoURL == nil {
		return fmt.Errorf("consumer %q has no profile photo URL", email)
	}
	parsed, err := url.Parse(*found.ProfilePhotoURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("consumer %q has invalid public profile photo URL %q", email, *found.ProfilePhotoURL)
	}
	return nil
}

func (suite *testSuite) consumerDirectoryDoesNotExposeSensitiveData(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	_, raw, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	for _, forbidden := range []string{"auth_id", "auth0_id", "password", "credentials", "permissions"} {
		if _, exposed := raw[forbidden]; exposed {
			return fmt.Errorf("consumer %q exposes sensitive field %q", email, forbidden)
		}
	}
	return nil
}

func (suite *testSuite) systemReturnsEmptyConsumerDirectory() error {
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	if directory == nil || len(directory) != 0 {
		return fmt.Errorf("expected JSON array [], got %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) consumerDirectoryResponse() (
	[]adminConsumerDirectoryResponse,
	[]map[string]json.RawMessage,
	error,
) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, nil, err
	}

	var directory []adminConsumerDirectoryResponse
	if err := json.Unmarshal(suite.lastBody, &directory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid consumer directory: %w", err)
	}
	var rawDirectory []map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &rawDirectory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid consumer directory object list: %w", err)
	}
	return directory, rawDirectory, nil
}

func findConsumerDirectoryEntry(
	directory []adminConsumerDirectoryResponse,
	rawDirectory []map[string]json.RawMessage,
	email string,
) (adminConsumerDirectoryResponse, map[string]json.RawMessage, error) {
	for index, found := range directory {
		if found.Email == email {
			return found, rawDirectory[index], nil
		}
	}
	return adminConsumerDirectoryResponse{}, nil, fmt.Errorf("consumer directory does not include %q", email)
}
