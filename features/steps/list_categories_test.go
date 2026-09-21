package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/cucumber/godog"
)

const categoryListingAuth0ID = "auth0|category-list-test"

type categoryListItemResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func registerListCategoriesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existen rubros registrados$`, suite.thereAreNoRegisteredCategories)
	sc.Step(`^que existen los siguientes rubros:$`, suite.thereAreRegisteredCategories)
	sc.Step(`^que estoy autenticado sin permisos administrativos$`, suite.iAmAuthenticatedWithoutAdministrativePermissions)
	sc.Step(`^consulto el listado de rubros$`, suite.requestCategoryList)
	sc.Step(`^el sistema muestra los rubros disponibles$`, suite.systemShowsAvailableCategories)
	sc.Step(`^el listado incluye el rubro "([^"]*)"$`, suite.categoryListIncludes)
	sc.Step(`^el sistema muestra un listado de rubros vacío$`, suite.systemShowsEmptyCategoryList)
	sc.Step(`^el sistema devuelve los siguientes rubros en este orden:$`, suite.systemReturnsCategoriesInOrder)
	sc.Step(`^cada rubro incluye solamente su identificador y su nombre visible$`, suite.everyCategoryOnlyIncludesPublicFields)
	sc.Step(`^el listado no expone el nombre normalizado ni datos internos de persistencia$`, suite.categoryListDoesNotExposeInternalData)
}

func (suite *testSuite) thereAreNoRegisteredCategories() error {
	if err := suite.categoryRepository.DeleteAll(); err != nil {
		return fmt.Errorf("deleting category fixtures: %w", err)
	}
	suite.categoryIDsByName = map[string]int{}
	return nil
}

func (suite *testSuite) thereAreRegisteredCategories(table *godog.Table) error {
	if err := requireTableHeaders(table, "nombre"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one category fixture column, got %d", len(row.Cells))
		}
		if err := suite.thereIsCategoryNamed(row.Cells[0].Value); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) iAmAuthenticatedWithoutAdministrativePermissions() error {
	suite.currentAuth0ID = categoryListingAuth0ID
	suite.currentPermissions = nil
	suite.invalidSession = false
	return nil
}

func (suite *testSuite) requestCategoryList() error {
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+"/categories", nil)
	if err != nil {
		return fmt.Errorf("creating category list request: %w", err)
	}

	if !suite.invalidSession {
		auth0ID := suite.currentAuth0ID
		if auth0ID == "" {
			auth0ID = categoryListingAuth0ID
		}
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, suite.currentPermissions))
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting category list: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading category list response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) systemShowsAvailableCategories() error {
	categories, err := suite.categoryListResponse()
	if err != nil {
		return err
	}
	if len(categories) == 0 {
		return fmt.Errorf("expected available categories, got empty list")
	}
	return nil
}

func (suite *testSuite) categoryListIncludes(name string) error {
	categories, err := suite.categoryListResponse()
	if err != nil {
		return err
	}
	for _, foundCategory := range categories {
		if foundCategory.Name == name {
			return nil
		}
	}
	return fmt.Errorf("expected category list to include %q, got body %s", name, string(suite.lastBody))
}

func (suite *testSuite) systemShowsEmptyCategoryList() error {
	categories, err := suite.categoryListResponse()
	if err != nil {
		return err
	}
	if len(categories) != 0 {
		return fmt.Errorf("expected empty category list, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemReturnsCategoriesInOrder(table *godog.Table) error {
	if err := requireTableHeaders(table, "nombre"); err != nil {
		return err
	}
	categories, err := suite.categoryListResponse()
	if err != nil {
		return err
	}
	expectedNames := make([]string, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one category assertion column, got %d", len(row.Cells))
		}
		expectedNames = append(expectedNames, row.Cells[0].Value)
	}
	actualNames := make([]string, 0, len(categories))
	for _, foundCategory := range categories {
		actualNames = append(actualNames, foundCategory.Name)
	}
	if !reflect.DeepEqual(actualNames, expectedNames) {
		return fmt.Errorf("expected category order %v, got %v", expectedNames, actualNames)
	}
	return nil
}

func (suite *testSuite) everyCategoryOnlyIncludesPublicFields() error {
	if _, err := suite.categoryListResponse(); err != nil {
		return err
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &items); err != nil {
		return fmt.Errorf("category list response is not valid JSON: %w", err)
	}
	for _, item := range items {
		if len(item) != 2 || item["id"] == nil || item["name"] == nil {
			return fmt.Errorf("expected category item to contain only id and name, got body %s", string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) categoryListDoesNotExposeInternalData() error {
	return suite.everyCategoryOnlyIncludesPublicFields()
}

func (suite *testSuite) categoryListResponse() ([]categoryListItemResponse, error) {
	if suite.lastStatus != http.StatusOK {
		return nil, fmt.Errorf("expected status code %d, got %d with body %s", http.StatusOK, suite.lastStatus, string(suite.lastBody))
	}

	var categories []categoryListItemResponse
	if err := json.Unmarshal(suite.lastBody, &categories); err != nil {
		return nil, fmt.Errorf("category list response is not valid JSON: %w", err)
	}
	return categories, nil
}
