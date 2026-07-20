package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"
)

const (
	mercadoPagoAuthorizationPath = "/providers/me/payment-accounts/authorization"
	mercadoPagoCallbackPath      = "/oauth/payment-accounts/callback"
	mercadoPagoConnectionPath    = "/providers/me/payment-accounts"
)

type mercadoPagoAccountFixture struct {
	marketplacePaymentsEnabled bool
}

type mercadoPagoAuthorizationResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

type mercadoPagoConnectionResponse struct {
	Status                  string `json:"status"`
	AccountID               string `json:"account_id"`
	CanReceivePayments      bool   `json:"can_receive_payments"`
	CanSendServiceProposals bool   `json:"can_send_service_proposals"`
}

func registerConnectMercadoPagoAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el registro del prestador "([^"]*)" está pendiente de conectar una cuenta de Mercado Pago$`, suite.providerRegistrationIsPendingMercadoPagoConnection)
	sc.Step(`^que la cuenta de Mercado Pago "([^"]*)" está habilitada para recibir pagos de marketplace$`, suite.mercadoPagoAccountIsEnabledForMarketplacePayments)
	sc.Step(`^que la cuenta de Mercado Pago "([^"]*)" no está habilitada para recibir pagos de marketplace$`, suite.mercadoPagoAccountIsNotEnabledForMarketplacePayments)
	sc.Step(`^que la cuenta de Mercado Pago "([^"]*)" ya está vinculada al prestador "([^"]*)"$`, suite.mercadoPagoAccountIsAlreadyLinkedToProvider)
	sc.Step(`^que la cuenta de Mercado Pago "([^"]*)" está vinculada al prestador "([^"]*)"$`, suite.mercadoPagoAccountIsLinkedToProvider)
	sc.Step(`^inicio la conexión de mi cuenta de Mercado Pago$`, suite.startMyMercadoPagoAccountConnection)
	sc.Step(`^que inicié la conexión de mi cuenta de Mercado Pago$`, suite.startedMyMercadoPagoAccountConnection)
	sc.Step(`^autorizo a LoResuelvo a operar con la cuenta de Mercado Pago "([^"]*)"$`, suite.authorizeLoResuelvoForMercadoPagoAccount)
	sc.Step(`^rechazo autorizar a LoResuelvo en Mercado Pago$`, suite.rejectLoResuelvoMercadoPagoAuthorization)
	sc.Step(`^intento conectar nuevamente la cuenta de Mercado Pago "([^"]*)"$`, suite.tryReconnectMercadoPagoAccount)
	sc.Step(`^intento conectar la cuenta de Mercado Pago "([^"]*)"$`, suite.tryConnectMercadoPagoAccount)
	sc.Step(`^Mercado Pago retorna una autorización con un estado de seguridad inválido$`, suite.mercadoPagoReturnsAuthorizationWithInvalidSecurityState)
	sc.Step(`^Mercado Pago retorna un código de autorización "([^"]*)"$`, suite.mercadoPagoReturnsAuthorizationCode)
	sc.Step(`^intento iniciar la conexión de una cuenta de Mercado Pago para el prestador "([^"]*)"$`, suite.tryStartMercadoPagoConnectionForProvider)
	sc.Step(`^intento enviar una propuesta de servicio al consumidor "([^"]*)" sin haber conectado una cuenta de Mercado Pago$`, suite.trySendServiceProposalWithoutMercadoPagoAccount)
	sc.Step(`^el sistema confirma la conexión de la cuenta de Mercado Pago$`, suite.systemConfirmsMercadoPagoAccountConnection)
	sc.Step(`^la cuenta de Mercado Pago "([^"]*)" queda vinculada al prestador "([^"]*)"$`, suite.mercadoPagoAccountBecomesLinkedToProvider)
	sc.Step(`^el prestador "([^"]*)" queda habilitado para recibir pagos$`, suite.providerCanReceivePayments)
	sc.Step(`^el prestador "([^"]*)" queda habilitado para enviar propuestas de servicio$`, suite.providerCanSendServiceProposals)
	sc.Step(`^el sistema informa que el prestador ya tiene una cuenta de Mercado Pago conectada$`, suite.systemReportsProviderAlreadyHasMercadoPagoAccount)
	sc.Step(`^el sistema conserva una única conexión con la cuenta de Mercado Pago "([^"]*)"$`, suite.systemKeepsSingleMercadoPagoAccountConnection)
	sc.Step(`^el prestador "([^"]*)" permanece habilitado para recibir pagos$`, suite.providerRemainsAbleToReceivePayments)
	sc.Step(`^el sistema informa que la conexión de Mercado Pago está pendiente$`, suite.systemReportsMercadoPagoConnectionIsPending)
	sc.Step(`^la propuesta de servicio no se envía$`, suite.serviceProposalIsNotSent)
	sc.Step(`^el sistema no vincula ninguna cuenta de Mercado Pago al prestador$`, suite.systemDoesNotLinkAnyMercadoPagoAccount)
	sc.Step(`^el prestador "([^"]*)" permanece registrado$`, suite.providerRemainsRegistered)
	sc.Step(`^la conexión de Mercado Pago permanece pendiente$`, suite.mercadoPagoConnectionRemainsPending)
	sc.Step(`^el sistema permite volver a intentar la conexión$`, suite.systemAllowsRetryingMercadoPagoConnection)
	sc.Step(`^el sistema rechaza la conexión de la cuenta de Mercado Pago$`, suite.systemRejectsMercadoPagoAccountConnection)
	sc.Step(`^la cuenta de Mercado Pago "([^"]*)" permanece vinculada solamente al prestador "([^"]*)"$`, suite.mercadoPagoAccountRemainsLinkedOnlyToProvider)
	sc.Step(`^la conexión de Mercado Pago de "([^"]*)" permanece pendiente$`, suite.providerMercadoPagoConnectionRemainsPending)
	sc.Step(`^el sistema rechaza la respuesta de autorización$`, suite.systemRejectsAuthorizationResponse)
	sc.Step(`^el sistema permite volver a iniciar la conexión$`, suite.systemAllowsRestartingMercadoPagoConnection)
	sc.Step(`^el sistema deniega la conexión de la cuenta de Mercado Pago$`, suite.systemDeniesMercadoPagoAccountConnection)
	sc.Step(`^el sistema informa que la cuenta de Mercado Pago no está habilitada para recibir pagos$`, suite.systemReportsMercadoPagoAccountCannotReceivePayments)
	sc.Step(`^el sistema no vincula la cuenta de Mercado Pago "([^"]*)" al prestador$`, suite.systemDoesNotLinkMercadoPagoAccount)
}

func (suite *testSuite) providerRegistrationIsPendingMercadoPagoConnection(providerEmail string) error {
	if _, err := suite.providerIDByEmail(providerEmail); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) mercadoPagoAccountIsEnabledForMarketplacePayments(accountID string) error {
	suite.mercadoPagoAccounts[accountID] = mercadoPagoAccountFixture{marketplacePaymentsEnabled: true}
	return nil
}

func (suite *testSuite) mercadoPagoAccountIsNotEnabledForMarketplacePayments(accountID string) error {
	suite.mercadoPagoAccounts[accountID] = mercadoPagoAccountFixture{marketplacePaymentsEnabled: false}
	return nil
}

func (suite *testSuite) mercadoPagoAccountIsAlreadyLinkedToProvider(accountID, providerEmail string) error {
	return suite.prepareLinkedMercadoPagoAccount(accountID, providerEmail)
}

func (suite *testSuite) mercadoPagoAccountIsLinkedToProvider(accountID, providerEmail string) error {
	return suite.prepareLinkedMercadoPagoAccount(accountID, providerEmail)
}

func (suite *testSuite) prepareLinkedMercadoPagoAccount(accountID, providerEmail string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()
	suite.mercadoPagoAccounts[accountID] = mercadoPagoAccountFixture{marketplacePaymentsEnabled: true}

	if err := suite.startMyMercadoPagoAccountConnection(); err != nil {
		return err
	}
	if err := suite.authorizeLoResuelvoForMercadoPagoAccount(accountID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusSeeOther); err != nil {
		return fmt.Errorf("preparing linked Mercado Pago account: %w", err)
	}

	return nil
}

func (suite *testSuite) startMyMercadoPagoAccountConnection() error {
	if err := suite.requestMercadoPagoAuthorization(); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	var response mercadoPagoAuthorizationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding Mercado Pago authorization response: %w", err)
	}

	if strings.TrimSpace(response.AuthorizationURL) == "" {
		return fmt.Errorf("expected Mercado Pago authorization response to include authorization_url")
	}
	authorizationURL, err := url.Parse(response.AuthorizationURL)
	if err != nil {
		return fmt.Errorf("parsing Mercado Pago authorization URL: %w", err)
	}
	state := authorizationURL.Query().Get("state")
	if response.State != "" && response.State != state {
		return fmt.Errorf("expected Mercado Pago authorization URL state to match response state")
	}
	if state == "" {
		return fmt.Errorf("expected Mercado Pago authorization response to include OAuth state")
	}

	suite.lastMercadoPagoOAuthState = state
	return nil
}

func (suite *testSuite) startedMyMercadoPagoAccountConnection() error {
	return suite.startMyMercadoPagoAccountConnection()
}

func (suite *testSuite) authorizeLoResuelvoForMercadoPagoAccount(accountID string) error {
	fixture, exists := suite.mercadoPagoAccounts[accountID]
	if !exists {
		return fmt.Errorf("Mercado Pago account fixture %q was not configured", accountID)
	}

	codePrefix := "marketplace-enabled"
	if !fixture.marketplacePaymentsEnabled {
		codePrefix = "marketplace-disabled"
	}

	return suite.requestMercadoPagoCallback(url.Values{
		"code":  []string{codePrefix + ":" + accountID},
		"state": []string{suite.lastMercadoPagoOAuthState},
	})
}

func (suite *testSuite) rejectLoResuelvoMercadoPagoAuthorization() error {
	return suite.requestMercadoPagoCallback(url.Values{
		"error":             []string{"access_denied"},
		"error_description": []string{"The provider denied authorization"},
		"state":             []string{suite.lastMercadoPagoOAuthState},
	})
}

func (suite *testSuite) tryReconnectMercadoPagoAccount(_ string) error {
	return suite.requestMercadoPagoAuthorization()
}

func (suite *testSuite) tryConnectMercadoPagoAccount(accountID string) error {
	if err := suite.startMyMercadoPagoAccountConnection(); err != nil {
		return err
	}
	return suite.authorizeLoResuelvoForMercadoPagoAccount(accountID)
}

func (suite *testSuite) mercadoPagoReturnsAuthorizationWithInvalidSecurityState() error {
	return suite.requestMercadoPagoCallback(url.Values{
		"code":  []string{"marketplace-enabled:mp-juan"},
		"state": []string{"invalid-" + suite.lastMercadoPagoOAuthState},
	})
}

func (suite *testSuite) mercadoPagoReturnsAuthorizationCode(code string) error {
	return suite.requestMercadoPagoCallback(url.Values{
		"code":  []string{code},
		"state": []string{suite.lastMercadoPagoOAuthState},
	})
}

func (suite *testSuite) tryStartMercadoPagoConnectionForProvider(_ string) error {
	return suite.requestMercadoPagoAuthorization()
}

func (suite *testSuite) trySendServiceProposalWithoutMercadoPagoAccount(consumerEmail string) error {
	return suite.requestServiceProposalToConsumer(consumerEmail, serviceProposalPayload{
		amount:      "15000.50",
		scheduledOn: "2026-07-21T12:30:00Z",
		description: "Servicio acordado entre consumidor y prestador.",
	})
}

func (suite *testSuite) systemConfirmsMercadoPagoAccountConnection() error {
	return suite.lastResponseShouldHaveStatusCode(http.StatusSeeOther)
}

func (suite *testSuite) mercadoPagoAccountBecomesLinkedToProvider(accountID, providerEmail string) error {
	connection, err := suite.mercadoPagoConnectionForProvider(providerEmail)
	if err != nil {
		return err
	}
	if connection.Status != "connected" || connection.AccountID != accountID {
		return fmt.Errorf("expected Mercado Pago account %q to be connected to provider %q, got body %s", accountID, providerEmail, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerCanReceivePayments(providerEmail string) error {
	connection, err := suite.mercadoPagoConnectionForProvider(providerEmail)
	if err != nil {
		return err
	}
	if !connection.CanReceivePayments {
		return fmt.Errorf("expected provider %q to be able to receive payments, got body %s", providerEmail, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerCanSendServiceProposals(providerEmail string) error {
	connection, err := suite.mercadoPagoConnectionForProvider(providerEmail)
	if err != nil {
		return err
	}
	if !connection.CanSendServiceProposals {
		return fmt.Errorf("expected provider %q to be able to send service proposals, got body %s", providerEmail, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemReportsProviderAlreadyHasMercadoPagoAccount() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemKeepsSingleMercadoPagoAccountConnection(accountID string) error {
	connection, err := suite.currentProviderMercadoPagoConnection()
	if err != nil {
		return err
	}
	if connection.Status != "connected" || connection.AccountID != accountID {
		return fmt.Errorf("expected the single connected Mercado Pago account to be %q, got body %s", accountID, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) providerRemainsAbleToReceivePayments(providerEmail string) error {
	return suite.providerCanReceivePayments(providerEmail)
}

func (suite *testSuite) systemReportsMercadoPagoConnectionIsPending() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) serviceProposalIsNotSent() error {
	if suite.lastStatus == http.StatusCreated {
		return fmt.Errorf("expected service proposal not to be sent, got status %d with body %s", suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemDoesNotLinkAnyMercadoPagoAccount() error {
	return suite.currentProviderMercadoPagoConnectionShouldBePending()
}

func (suite *testSuite) providerRemainsRegistered(providerEmail string) error {
	_, err := suite.providerIDByEmail(providerEmail)
	return err
}

func (suite *testSuite) mercadoPagoConnectionRemainsPending() error {
	return suite.currentProviderMercadoPagoConnectionShouldBePending()
}

func (suite *testSuite) systemAllowsRetryingMercadoPagoConnection() error {
	return suite.startMyMercadoPagoAccountConnection()
}

func (suite *testSuite) systemRejectsMercadoPagoAccountConnection() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) mercadoPagoAccountRemainsLinkedOnlyToProvider(accountID, providerEmail string) error {
	if err := suite.mercadoPagoAccountBecomesLinkedToProvider(accountID, providerEmail); err != nil {
		return err
	}
	return suite.currentProviderMercadoPagoConnectionShouldBePending()
}

func (suite *testSuite) providerMercadoPagoConnectionRemainsPending(providerEmail string) error {
	connection, err := suite.mercadoPagoConnectionForProvider(providerEmail)
	if err != nil {
		return err
	}
	if connection.Status != "pending" || connection.AccountID != "" {
		return fmt.Errorf("expected Mercado Pago connection for provider %q to remain pending, got body %s", providerEmail, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemRejectsAuthorizationResponse() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) systemAllowsRestartingMercadoPagoConnection() error {
	return suite.startMyMercadoPagoAccountConnection()
}

func (suite *testSuite) systemDeniesMercadoPagoAccountConnection() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemReportsMercadoPagoAccountCannotReceivePayments() error {
	return suite.mercadoPagoRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemDoesNotLinkMercadoPagoAccount(accountID string) error {
	connection, err := suite.currentProviderMercadoPagoConnection()
	if err != nil {
		return err
	}
	if connection.Status != "pending" || connection.AccountID == accountID {
		return fmt.Errorf("expected Mercado Pago account %q not to be linked, got body %s", accountID, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) requestMercadoPagoAuthorization() error {
	return suite.requestMercadoPago(http.MethodPost, mercadoPagoAuthorizationPath, true)
}

func (suite *testSuite) requestMercadoPagoCallback(query url.Values) error {
	return suite.requestMercadoPago(http.MethodGet, mercadoPagoCallbackPath+"?"+query.Encode(), false)
}

func (suite *testSuite) requestCurrentProviderMercadoPagoConnection() error {
	return suite.requestMercadoPago(http.MethodGet, mercadoPagoConnectionPath, true)
}

func (suite *testSuite) requestMercadoPago(method, path string, authenticated bool) error {
	httpRequest, err := http.NewRequest(method, suite.server.URL+path, nil)
	if err != nil {
		return err
	}
	if authenticated && suite.currentAuth0ID != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
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
	return nil
}

func (suite *testSuite) mercadoPagoConnectionForProvider(providerEmail string) (mercadoPagoConnectionResponse, error) {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	return suite.currentProviderMercadoPagoConnection()
}

func (suite *testSuite) currentProviderMercadoPagoConnection() (mercadoPagoConnectionResponse, error) {
	if err := suite.requestCurrentProviderMercadoPagoConnection(); err != nil {
		return mercadoPagoConnectionResponse{}, err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return mercadoPagoConnectionResponse{}, err
	}

	var response mercadoPagoConnectionResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return mercadoPagoConnectionResponse{}, fmt.Errorf("decoding Mercado Pago connection response: %w", err)
	}
	return response, nil
}

func (suite *testSuite) currentProviderMercadoPagoConnectionShouldBePending() error {
	connection, err := suite.currentProviderMercadoPagoConnection()
	if err != nil {
		return err
	}
	if connection.Status != "pending" || connection.AccountID != "" {
		return fmt.Errorf("expected current provider Mercado Pago connection to remain pending, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) mercadoPagoRequestShouldFailWithStatus(status int) error {
	if err := suite.lastResponseShouldHaveStatusCode(status); err != nil {
		return err
	}
	return suite.lastResponseShouldHaveError()
}
