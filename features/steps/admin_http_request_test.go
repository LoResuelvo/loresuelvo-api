package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cucumber/godog"
)

type adminRequestState struct {
	omitBearer    bool
	invalidBearer bool
	headers       http.Header
}

func registerAdminHTTPRequestSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no envío un token Bearer$`, suite.doNotSendAdminBearer)
	sc.Step(`^que envío un token Bearer inválido$`, suite.sendInvalidAdminBearer)
	sc.Step(`^la respuesta incluye la cabecera "([^"]*)" con valor "([^"]*)"$`, suite.adminResponseHeaderEquals)
	sc.Step(`^la página contiene una colección vacía, no nula, y no tiene cursor siguiente$`, suite.adminPageIsEmpty)
	sc.Step(`^el inicio del rango es inclusivo y el fin es exclusivo$`, suite.queriedRangeHasExpectedBounds)
}

func (suite *testSuite) queriedRangeHasExpectedBounds() error {
	if suite.operationInbox.startWindow != nil {
		return suite.inboxStartWindowHasExpectedBounds()
	}
	return suite.auditRangeHasExpectedBounds()
}

func (suite *testSuite) doNotSendAdminBearer() error {
	suite.adminRequest.omitBearer = true
	return nil
}

func (suite *testSuite) sendInvalidAdminBearer() error {
	suite.adminRequest.invalidBearer = true
	return nil
}

func (suite *testSuite) sendAdminGet(path string, query url.Values, correlation string) error {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("building administrative request %s: %w", path, err)
	}
	if !suite.adminRequest.omitBearer {
		if suite.adminRequest.invalidBearer {
			request.Header.Set("Authorization", "Bearer invalid-token")
		} else {
			request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
		}
	}
	if correlation != "" {
		request.Header.Set("X-Request-ID", correlation)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", path, err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	suite.adminRequest.headers = response.Header.Clone()
	return nil
}

func (suite *testSuite) adminResponseHeaderEquals(name, value string) error {
	if got := suite.adminRequest.headers.Get(name); got != value {
		return fmt.Errorf("expected %s header %q, got %q", name, value, got)
	}
	return nil
}

func (suite *testSuite) adminPageIsEmpty() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("invalid administrative page JSON: %w", err)
	}
	cursor, exists := raw["next_cursor"]
	if !exists || len(raw) != 2 {
		return fmt.Errorf("expected one collection and next_cursor, got %s", suite.lastBody)
	}
	if string(cursor) != "null" {
		return fmt.Errorf("expected no next cursor, got %s", cursor)
	}
	for field, value := range raw {
		if field != "next_cursor" && string(value) != "[]" {
			return fmt.Errorf("expected %s to be an empty non-null collection, got %s", field, value)
		}
	}
	return nil
}
