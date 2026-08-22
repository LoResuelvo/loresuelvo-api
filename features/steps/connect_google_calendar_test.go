package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"
)

const (
	googleCalendarAuthorizationPath = "/me/calendar-connection/authorizations"
	googleCalendarConnectionPath    = "/me/calendar-connection"
	googleCalendarCallbackPath      = "/oauth/google-calendar/callback"
	googleCalendarEventsOwnedScope  = "https://www.googleapis.com/auth/calendar.events.owned"
)

type googleCalendarAuthorizationResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

func registerConnectGoogleCalendarSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^inicio la vinculación de Google Calendar desde la web$`, suite.startGoogleCalendarWebAuthorization)
	sc.Step(`^el sistema devuelve una autorización web de Google Calendar$`, suite.systemReturnsGoogleCalendarWebAuthorization)
	sc.Step(`^la autorización solicita el permiso de eventos propios de Google Calendar$`, suite.googleCalendarAuthorizationRequestsOwnedEvents)
	sc.Step(`^autorizo el acceso de Google Calendar$`, suite.authorizeGoogleCalendarAccess)
	sc.Step(`^vinculo Google Calendar desde Android con el server auth code "([^"]*)"$`, suite.connectGoogleCalendarFromAndroid)
	sc.Step(`^el sistema confirma la vinculación de Google Calendar$`, suite.systemConfirmsGoogleCalendarConnection)
	sc.Step(`^el perfil informa el estado de Google Calendar "([^"]*)"$`, suite.profileReportsGoogleCalendarStatus)
}

func (suite *testSuite) startGoogleCalendarWebAuthorization() error {
	return suite.requestGoogleCalendar(http.MethodPost, googleCalendarAuthorizationPath, true)
}

func (suite *testSuite) systemReturnsGoogleCalendarWebAuthorization() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	var response googleCalendarAuthorizationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding Google Calendar authorization response: %w", err)
	}
	if strings.TrimSpace(response.AuthorizationURL) == "" {
		return fmt.Errorf("expected Google Calendar authorization response to include authorization_url")
	}

	authorizationURL, err := url.Parse(response.AuthorizationURL)
	if err != nil {
		return fmt.Errorf("parsing Google Calendar authorization URL: %w", err)
	}
	state := authorizationURL.Query().Get("state")
	if state == "" {
		return fmt.Errorf("expected Google Calendar authorization URL to include OAuth state")
	}
	if response.State != "" && response.State != state {
		return fmt.Errorf("expected Google Calendar response state to match authorization URL state")
	}

	suite.lastGoogleCalendarOAuthState = state
	return nil
}

func (suite *testSuite) googleCalendarAuthorizationRequestsOwnedEvents() error {
	var response googleCalendarAuthorizationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding Google Calendar authorization response: %w", err)
	}
	authorizationURL, err := url.Parse(response.AuthorizationURL)
	if err != nil {
		return fmt.Errorf("parsing Google Calendar authorization URL: %w", err)
	}

	scopes := strings.Fields(authorizationURL.Query().Get("scope"))
	for _, scope := range scopes {
		if scope == googleCalendarEventsOwnedScope {
			return nil
		}
	}
	return fmt.Errorf("expected Google Calendar authorization scope %q, got %q", googleCalendarEventsOwnedScope, authorizationURL.Query().Get("scope"))
}

func (suite *testSuite) authorizeGoogleCalendarAccess() error {
	return suite.requestGoogleCalendar(http.MethodGet, googleCalendarCallbackPath+"?"+url.Values{
		"code":  []string{"authorized:google-calendar-ana"},
		"state": []string{suite.lastGoogleCalendarOAuthState},
	}.Encode(), false)
}

func (suite *testSuite) systemConfirmsGoogleCalendarConnection() error {
	if suite.lastStatus != http.StatusSeeOther && suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("expected Google Calendar connection confirmation status 201 or 303, got %d with body %s", suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) connectGoogleCalendarFromAndroid(serverAuthCode string) error {
	return suite.requestGoogleCalendarWithBody(
		http.MethodPost,
		googleCalendarConnectionPath,
		true,
		map[string]string{"server_auth_code": serverAuthCode},
	)
}

func (suite *testSuite) profileReportsGoogleCalendarStatus(expectedStatus string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	var response struct {
		CalendarConnectionStatus string `json:"calendar_connection_status"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding current user response: %w", err)
	}
	if response.CalendarConnectionStatus != expectedStatus {
		return fmt.Errorf("expected Google Calendar status %q, got %q in body %s", expectedStatus, response.CalendarConnectionStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) requestGoogleCalendar(method, path string, authenticated bool) error {
	return suite.requestGoogleCalendarWithBody(method, path, authenticated, nil)
}

func (suite *testSuite) requestGoogleCalendarWithBody(method, path string, authenticated bool, payload any) error {
	var requestBody *bytes.Reader
	if payload == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encodedPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encoding Google Calendar request body: %w", err)
		}
		requestBody = bytes.NewReader(encodedPayload)
	}

	httpRequest, err := http.NewRequest(method, suite.server.URL+path, requestBody)
	if err != nil {
		return err
	}
	if authenticated && suite.currentAuth0ID != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}
	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	response, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = response.StatusCode
	suite.lastBody = responseBody
	suite.lastLocation = response.Header.Get("Location")
	return nil
}
