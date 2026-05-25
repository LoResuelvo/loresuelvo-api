package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const categoryListingAuth0ID = "auth0|category-list-test"

type categoryListItemResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func registerListCategoriesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existen rubros registrados$`, suite.thereAreNoRegisteredCategories)
	sc.Step(`^consulto el listado de rubros$`, suite.requestCategoryList)
	sc.Step(`^el sistema muestra los rubros disponibles$`, suite.systemShowsAvailableCategories)
	sc.Step(`^el listado incluye el rubro "([^"]*)"$`, suite.categoryListIncludes)
	sc.Step(`^el sistema muestra un listado de rubros vacío$`, suite.systemShowsEmptyCategoryList)
}

func (suite *testSuite) thereAreNoRegisteredCategories() error {
	if err := suite.categoryRepository.DeleteAll(); err != nil {
		return err
	}

	suite.categoryIDsByName = map[string]int{}

	return nil
}

func (suite *testSuite) requestCategoryList() error {
	resp, err := suite.getCategoryList()
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

func (suite *testSuite) systemShowsAvailableCategories() error {
	categories, err := suite.categoryListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(categories) == 0 {
		return fmt.Errorf("expected available categories, got empty list")
	}

	return nil
}

func (suite *testSuite) categoryListIncludes(name string) error {
	categories, err := suite.categoryListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, category := range categories {
		if category.Name == name {
			return nil
		}
	}

	return fmt.Errorf("expected category list to include %q, got body %s", name, string(suite.lastBody))
}

func (suite *testSuite) systemShowsEmptyCategoryList() error {
	categories, err := suite.categoryListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(categories) != 0 {
		return fmt.Errorf("expected empty category list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) categoryListResponseShouldHaveStatusCode(statusCode int) ([]categoryListItemResponse, error) {
	if suite.lastStatus != statusCode {
		return nil, fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}

	var categories []categoryListItemResponse
	if err := json.Unmarshal(suite.lastBody, &categories); err != nil {
		return nil, fmt.Errorf("response is not valid JSON category list: %w", err)
	}

	return categories, nil
}

func (suite *testSuite) getCategoryList() (*http.Response, error) {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/categories", nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(categoryListingAuth0ID, nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}

	return resp, nil
}
