package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	sc.Step(`^rechazo autorizar el acceso de Google Calendar$`, suite.rejectGoogleCalendarAccess)
	sc.Step(`^que el consumidor "([^"]*)" ya tiene Google Calendar vinculado$`, suite.consumerAlreadyHasGoogleCalendarConnection)
	sc.Step(`^vuelvo a vincular Google Calendar desde la web$`, suite.reconnectGoogleCalendarWebAuthorization)
	sc.Step(`^que la conexión de Google Calendar de "([^"]*)" requiere atención$`, suite.googleCalendarConnectionRequiresAttention)
	sc.Step(`^vuelvo a autorizar Google Calendar desde la web$`, suite.reconnectGoogleCalendarWebAuthorization)
	sc.Step(`^vinculo Google Calendar desde Android con el server auth code "([^"]*)"$`, suite.connectGoogleCalendarFromAndroid)
	sc.Step(`^el sistema confirma la vinculación de Google Calendar$`, suite.systemConfirmsGoogleCalendarConnection)
	sc.Step(`^el sistema informa que la autorización de Google Calendar fue rechazada$`, suite.systemReportsGoogleCalendarAuthorizationRejected)
	sc.Step(`^el consumidor "([^"]*)" conserva una única conexión de Google Calendar$`, suite.consumerHasSingleGoogleCalendarConnection)
	sc.Step(`^el perfil informa el estado de Google Calendar "([^"]*)"$`, suite.profileReportsGoogleCalendarStatus)
}

type calendarConnectionCounter interface {
	CountByUserID(context.Context, int) (int, error)
}

func (suite *testSuite) startGoogleCalendarWebAuthorization() error {
	return suite.requestGoogleCalendar(http.MethodPost, googleCalendarAuthorizationPath, true)
}

func (suite *testSuite) reconnectGoogleCalendarWebAuthorization() error {
	if err := suite.startGoogleCalendarWebAuthorization(); err != nil {
		return err
	}
	return suite.systemReturnsGoogleCalendarWebAuthorization()
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

func (suite *testSuite) rejectGoogleCalendarAccess() error {
	if err := suite.systemReturnsGoogleCalendarWebAuthorization(); err != nil {
		return err
	}
	return suite.requestGoogleCalendar(http.MethodGet, googleCalendarCallbackPath+"?"+url.Values{
		"error": []string{"access_denied"},
		"state": []string{suite.lastGoogleCalendarOAuthState},
	}.Encode(), false)
}

func (suite *testSuite) consumerAlreadyHasGoogleCalendarConnection(email string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(email)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	if err := suite.startGoogleCalendarWebAuthorization(); err != nil {
		return err
	}
	if err := suite.systemReturnsGoogleCalendarWebAuthorization(); err != nil {
		return err
	}
	if err := suite.authorizeGoogleCalendarAccess(); err != nil {
		return err
	}
	return suite.systemConfirmsGoogleCalendarConnection()
}

func (suite *testSuite) googleCalendarConnectionRequiresAttention(email string) error {
	if err := suite.consumerAlreadyHasGoogleCalendarConnection(email); err != nil {
		return err
	}
	userID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding consumer %q: %w", email, err)
	}
	connection, err := suite.calendarConnectionRepository.FindByUserID(context.Background(), userID)
	if err != nil {
		return fmt.Errorf("finding Google Calendar connection for %q: %w", email, err)
	}
	connection.MarkActionRequired(time.Now().UTC())
	if err := suite.calendarConnectionRepository.Save(context.Background(), connection); err != nil {
		return fmt.Errorf("marking Google Calendar connection for %q as requiring attention: %w", email, err)
	}
	return nil
}

func (suite *testSuite) consumerHasSingleGoogleCalendarConnection(email string) error {
	userID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding consumer %q: %w", email, err)
	}
	if _, err := suite.calendarConnectionRepository.FindByUserID(context.Background(), userID); err != nil {
		return fmt.Errorf("finding Google Calendar connection for %q: %w", email, err)
	}

	counter, ok := any(suite.calendarConnectionRepository).(calendarConnectionCounter)
	if !ok {
		return fmt.Errorf("calendar connection repository does not expose a connection count")
	}
	count, err := counter.CountByUserID(context.Background(), userID)
	if err != nil {
		return fmt.Errorf("counting Google Calendar connections for %q: %w", email, err)
	}
	if count != 1 {
		return fmt.Errorf("expected one Google Calendar connection for %q, got %d", email, count)
	}
	return nil
}

func (suite *testSuite) systemConfirmsGoogleCalendarConnection() error {
	if suite.lastStatus != http.StatusSeeOther && suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("expected Google Calendar connection confirmation status 201 or 303, got %d with body %s", suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemReportsGoogleCalendarAuthorizationRejected() error {
	if suite.lastStatus != http.StatusSeeOther {
		return fmt.Errorf("expected rejected Google Calendar authorization to redirect with status 303, got %d with body %s", suite.lastStatus, string(suite.lastBody))
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
