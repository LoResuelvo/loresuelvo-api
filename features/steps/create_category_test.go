package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

const categoryCreationAdminAuth0ID = "auth0|category-admin-test"

var categoryCreationAdminPermissions = []string{
	"create:categories",
	"create:category",
	"create:rubros",
	"create:rubro",
	"manage:categories",
	"manage:rubros",
}

type categoryCreationRequest struct {
	Name string `json:"name"`
}

func registerCreateCategorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe el rubro "([^"]*)"$`, suite.thereIsNoCategoryNamed)
	sc.Step(`^existe el rubro "([^"]*)"$`, suite.thereIsCategoryNamed)
	sc.Step(`^creo el rubro "([^"]*)"$`, suite.requestCategoryCreationWithName)
	sc.Step(`^intento crear un rubro sin nombre$`, suite.tryCreateCategoryWithoutName)
	sc.Step(`^intento crear el rubro "([^"]*)"$`, suite.requestCategoryCreationWithName)
	sc.Step(`^el sistema confirma la creación del rubro$`, suite.systemConfirmsCategoryCreation)
	sc.Step(`^el sistema me indica que el nombre del rubro es obligatorio$`, suite.systemReportsCategoryNameIsRequired)
	sc.Step(`^el sistema me indica que el rubro ya existe$`, suite.systemReportsCategoryAlreadyExists)
}

func (suite *testSuite) thereIsNoCategoryNamed(name string) error {
	_, err := suite.database.Exec(`DELETE FROM categories WHERE lower(trim(name)) = lower(trim($1))`, name)
	if err != nil {
		return fmt.Errorf("could not remove category %q: %w", name, err)
	}

	return nil
}

func (suite *testSuite) thereIsCategoryNamed(name string) error {
	resp, err := suite.postCategoryCreation(categoryCreationRequest{Name: name})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare existing category: status %d, body %s", resp.StatusCode, string(body))
}

func (suite *testSuite) requestCategoryCreationWithName(name string) error {
	return suite.requestCategoryCreation(categoryCreationRequest{Name: name})
}

func (suite *testSuite) tryCreateCategoryWithoutName() error {
	return suite.requestCategoryCreation(map[string]any{})
}

func (suite *testSuite) requestCategoryCreation(payload any) error {
	resp, err := suite.postCategoryCreation(payload)
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

func (suite *testSuite) systemConfirmsCategoryCreation() error {
	return suite.categoryCreationResponseShouldHaveStatusCode(http.StatusCreated)
}

func (suite *testSuite) systemReportsCategoryNameIsRequired() error {
	return suite.categoryCreationResponseShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) systemReportsCategoryAlreadyExists() error {
	return suite.categoryCreationResponseShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) categoryCreationResponseShouldFailWithStatus(statusCode int) error {
	if err := suite.categoryCreationResponseShouldHaveStatusCode(statusCode); err != nil {
		return err
	}

	return suite.categoryCreationResponseShouldHaveError()
}

func (suite *testSuite) categoryCreationResponseShouldHaveStatusCode(statusCode int) error {
	if suite.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) categoryCreationResponseShouldHaveError() error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	if strings.TrimSpace(response.Error) == "" {
		return fmt.Errorf("expected an error response, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) postCategoryCreation(payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/categories", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(categoryCreationAdminAuth0ID, categoryCreationAdminPermissions))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}

	return resp, nil
}
