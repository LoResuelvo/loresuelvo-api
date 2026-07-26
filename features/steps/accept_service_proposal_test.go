package steps_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const (
	workOrderStatusScheduled     = "scheduled"
	checkoutReadyStatus          = "checkout_ready"
	bookingDepositCheckoutPath   = "/service-proposals/%d/checkout-sessions"
	paymentIntentPath            = "/payment-intents/%s"
	defaultBookingCurrency       = "ARS"
	defaultBookingProviderEmail  = "juan.plomero@example.com"
	defaultBookingConsumerEmail  = "ana@example.com"
	testMercadoPagoWebhookSecret = "test-mercado-pago-webhook-secret"
)

type checkoutSessionResponse struct {
	PaymentIntentID string               `json:"payment_intent_id"`
	Status          string               `json:"status"`
	CheckoutURL     string               `json:"checkout_url"`
	ExpiresOn       time.Time            `json:"expires_on"`
	Pricing         bookingTermsResponse `json:"pricing"`
}

type paymentIntentResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type checkoutHTTPResponse struct {
	Status int
	Body   []byte
	Value  checkoutSessionResponse
}

type workOrderResponse struct {
	ID                int       `json:"id"`
	ServiceProposalID int       `json:"service_proposal_id"`
	ConsumerID        int       `json:"consumer_id"`
	ProviderID        int       `json:"provider_id"`
	AmountCents       int64     `json:"amount_cents"`
	ScheduledOn       time.Time `json:"scheduled_on"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	AcceptedOn        time.Time `json:"accepted_on"`
}

func registerAcceptServiceProposalSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)" programada para "([^"]*)"$`, suite.thereIsPendingServiceProposalScheduledOn)
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)" por "([^"]*)" programada para "([^"]*)"$`, suite.thereIsPendingServiceProposalForAmountScheduledOn)
	sc.Step(`^que "([^"]*)" inició el checkout de la seña de una propuesta pendiente de "([^"]*)"$`, suite.consumerStartedCheckoutForPendingProposal)
	sc.Step(`^que inicié el checkout de la seña de la propuesta$`, suite.startedCheckoutForPreparedProposal)
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)" con un intento de pago rechazado$`, suite.thereIsPendingProposalWithRejectedPayment)
	sc.Step(`^que existe una propuesta de servicio aceptada de "([^"]*)" para "([^"]*)"$`, suite.thereIsAcceptedServiceProposal)
	sc.Step(`^que existe una propuesta de servicio rechazada de "([^"]*)" para "([^"]*)"$`, suite.thereIsRejectedServiceProposal)
	sc.Step(`^que el límite para pagar la seña era "([^"]*)"$`, suite.bookingPaymentDeadlineWas)
	sc.Step(`^que un primer pago aprobado ya confirmó la propuesta y generó su orden de trabajo$`, suite.firstApprovedPaymentConfirmedProposal)
	sc.Step(`^que el pago aprobado de la seña confirmó la propuesta y generó su orden de trabajo$`, suite.approvedPaymentConfirmedProposal)
	sc.Step(`^que la credencial de Mercado Pago de "([^"]*)" venció$`, suite.mercadoPagoCredentialExpired)
	sc.Step(`^que su autorización permite renovarla$`, suite.mercadoPagoCredentialCanBeRefreshed)
	sc.Step(`^que Mercado Pago rechaza su renovación$`, suite.mercadoPagoRejectsCredentialRefresh)

	sc.Step(`^solicito pagar la seña de la propuesta de servicio pendiente$`, suite.requestPendingProposalCheckout)
	sc.Step(`^"([^"]*)" solicita nuevamente pagar la seña de la propuesta$`, suite.consumerRequestsCheckoutAgain)
	sc.Step(`^intento pagar la seña de la propuesta de servicio pendiente de "([^"]*)"$`, suite.tryPayPendingProposalForConsumer)
	sc.Step(`^intento pagar la seña de la propuesta de servicio pendiente$`, suite.tryPayPendingProposal)
	sc.Step(`^intento pagar la seña de la propuesta de servicio aceptada$`, suite.tryPayAcceptedProposal)
	sc.Step(`^intento pagar la seña de la propuesta de servicio rechazada$`, suite.tryPayRejectedProposal)
	sc.Step(`^solicito pagar la seña de la propuesta$`, suite.requestPreparedProposalCheckout)
	sc.Step(`^solicito concurrentemente dos veces pagar la seña de la propuesta$`, suite.requestCheckoutConcurrentlyTwice)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado por "([^"]*)" pesos argentinos para esa seña$`, suite.processApprovedPaymentForAmount)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado para esa seña$`, suite.processApprovedPayment)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago en proceso para esa seña$`, suite.processProcessingPayment)
	sc.Step(`^el sistema procesa dos veces la misma notificación válida de Mercado Pago y verifica el pago aprobado$`, suite.processSameApprovedPaymentNotificationTwice)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica que el pago se aprobó en "([^"]*)"$`, suite.processPaymentApprovedOn)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un segundo pago aprobado para la misma seña$`, suite.processSecondApprovedPayment)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica (una devolución|un contracargo) del pago de la seña$`, suite.processPaymentReversal)

	sc.Step(`^el sistema entrega una URL para completar el checkout de la seña$`, suite.systemReturnsBookingCheckoutURL)
	sc.Step(`^el sistema entrega una URL para completar un nuevo checkout$`, suite.systemReturnsNewCheckoutURL)
	sc.Step(`^el sistema entrega una URL HTTPS de Checkout Pro$`, suite.systemReturnsHTTPSCheckoutProURL)
	sc.Step(`^la respuesta identifica el intento de pago en estado "([^"]*)"$`, suite.responseIdentifiesPaymentIntentWithStatus)
	sc.Step(`^la respuesta identifica un nuevo intento de pago en estado "([^"]*)"$`, suite.responseIdentifiesNewPaymentIntentWithStatus)
	sc.Step(`^la respuesta informa el siguiente desglose en pesos argentinos:$`, suite.checkoutResponseIncludesPricingBreakdown)
	sc.Step(`^la respuesta informa que la sesión de checkout vence en "([^"]*)"$`, suite.checkoutSessionExpiresOn)
	sc.Step(`^el intento de pago puede consultarse en estado "([^"]*)"$`, suite.paymentIntentCanBeReadWithStatus)
	sc.Step(`^la propuesta de servicio queda aceptada$`, suite.serviceProposalIsAccepted)
	sc.Step(`^la propuesta de servicio permanece aceptada$`, suite.serviceProposalRemainsAccepted)
	sc.Step(`^la propuesta de servicio permanece pendiente$`, suite.serviceProposalRemainsPending)
	sc.Step(`^el sistema registra una única orden de trabajo programada$`, suite.systemRegistersOneScheduledWorkOrder)
	sc.Step(`^el sistema registra una única orden de trabajo para la propuesta$`, suite.systemKeepsOneWorkOrderForServiceProposal)
	sc.Step(`^el sistema conserva una única orden de trabajo para la propuesta$`, suite.systemKeepsOneWorkOrderForServiceProposal)
	sc.Step(`^el sistema no registra una orden de trabajo para la propuesta$`, suite.systemDoesNotRegisterWorkOrderForServiceProposal)
	sc.Step(`^la orden de trabajo queda vinculada a la propuesta aceptada$`, suite.workOrderIsLinkedToAcceptedServiceProposal)
	sc.Step(`^la orden de trabajo conserva el consumidor, el prestador, el precio del servicio, la fecha y hora y la descripción acordados$`, suite.workOrderKeepsAgreedTerms)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real la notificación de propuesta de servicio aceptada$`, suite.providerReceivesAcceptedServiceProposalNotification)
	sc.Step(`^el sistema deniega el pago de la seña$`, suite.systemDeniesBookingDepositPayment)
	sc.Step(`^el sistema rechaza pagar una propuesta de servicio ya aceptada$`, suite.systemRejectsCheckoutForAcceptedProposal)
	sc.Step(`^el sistema rechaza pagar una propuesta de servicio rechazada$`, suite.systemRejectsCheckoutForRejectedProposal)
	sc.Step(`^el sistema rechaza el pago porque finalizó el plazo para confirmar la contratación$`, suite.systemRejectsCheckoutAfterBookingDeadline)
	sc.Step(`^el sistema conserva un único intento de pago activo para la propuesta$`, suite.systemKeepsOneActivePaymentIntent)
	sc.Step(`^el sistema conserva una única sesión de checkout activa para la propuesta$`, suite.systemKeepsOneActiveCheckoutSession)
	sc.Step(`^ambas solicitudes obtienen la misma URL de checkout$`, suite.concurrentRequestsReturnSameCheckoutURL)
	sc.Step(`^el sistema registra una única transacción para el pago externo$`, suite.systemRegistersOneTransactionForExternalPayment)
	sc.Step(`^el sistema registra un incidente de pago "([^"]*)"$`, suite.systemRegistersPaymentIncident)
	sc.Step(`^el sistema registra un incidente de pago "([^"]*)" para el segundo pago$`, suite.systemRegistersPaymentIncidentForSecondPayment)
	sc.Step(`^la cuenta de pago del prestador permanece conectada$`, suite.providerPaymentAccountRemainsConnected)
	sc.Step(`^el sistema informa que la cuenta de pago del prestador debe volver a autorizarse$`, suite.systemReportsPaymentAccountReauthorizationRequired)
	sc.Step(`^la cuenta de pago del prestador queda en estado "([^"]*)"$`, suite.providerPaymentAccountHasStatus)
	sc.Step(`^el sistema no registra una sesión de checkout activa$`, suite.systemDoesNotRegisterActiveCheckoutSession)
	sc.Step(`^la orden de trabajo queda marcada como "([^"]*)"$`, suite.workOrderHasStatus)
}

func (suite *testSuite) thereIsPendingServiceProposalScheduledOn(providerEmail, consumerEmail, scheduledOn string) error {
	return suite.thereIsPendingServiceProposalForAmountScheduledOn(providerEmail, consumerEmail, "100000.00", scheduledOn)
}

func (suite *testSuite) thereIsPendingServiceProposalForAmountScheduledOn(providerEmail, consumerEmail, amount, scheduledOn string) error {
	amountCents, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing service proposal scheduled_on: %w", err)
	}
	return suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusPending,
		amountCents,
		scheduledAt.UTC(),
		defaultServiceProposalDescription,
	)
}

func (suite *testSuite) consumerStartedCheckoutForPendingProposal(consumerEmail, providerEmail string) error {
	if err := suite.thereIsPendingServiceProposalForAmountScheduledOn(
		providerEmail,
		consumerEmail,
		"100000.00",
		"2026-07-06T10:00:00-03:00",
	); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	return suite.startCheckoutForPreparedProposal()
}

func (suite *testSuite) startedCheckoutForPreparedProposal() error {
	if suite.currentAuth0ID == "" {
		suite.currentAuth0ID = auth0IDForConsumerEmail(defaultBookingConsumerEmail)
	}
	return suite.startCheckoutForPreparedProposal()
}

func (suite *testSuite) startCheckoutForPreparedProposal() error {
	if err := suite.requestPreparedProposalCheckout(); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated && suite.lastStatus != http.StatusOK {
		return fmt.Errorf("preparing checkout: expected status 200 or 201, got %d with body %s", suite.lastStatus, suite.lastBody)
	}
	response, err := checkoutSessionResponseFromBody(suite.lastBody)
	if err != nil {
		return err
	}
	if response.PaymentIntentID == "" {
		return fmt.Errorf("preparing checkout: response does not include payment_intent_id")
	}
	suite.rememberCheckoutResponse(response)
	return nil
}

func (suite *testSuite) thereIsPendingProposalWithRejectedPayment(providerEmail, consumerEmail string) error {
	if err := suite.consumerStartedCheckoutForPendingProposal(consumerEmail, providerEmail); err != nil {
		return err
	}
	externalPaymentID := suite.checkoutClient.AddRejectedPayment(
		suite.lastPaymentIntentID,
		"mp-juan",
		suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
	)
	if err := suite.sendMercadoPagoPaymentNotification(externalPaymentID); err != nil {
		return err
	}
	return suite.paymentIntentCanBeReadWithStatus(string(payment.StatusRejected))
}

func (suite *testSuite) thereIsAcceptedServiceProposal(providerEmail, consumerEmail string) error {
	if err := suite.consumerStartedCheckoutForPendingProposal(consumerEmail, providerEmail); err != nil {
		return err
	}
	if err := suite.processApprovedPayment(); err != nil {
		return err
	}
	return suite.serviceProposalIsAccepted()
}

func (suite *testSuite) thereIsRejectedServiceProposal(providerEmail, consumerEmail string) error {
	return suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusRejected,
		defaultServiceProposalAmount,
		defaultServiceProposalScheduledOn,
		defaultServiceProposalDescription,
	)
}

func (suite *testSuite) bookingPaymentDeadlineWas(expected string) error {
	expectedTime, err := time.Parse(time.RFC3339, expected)
	if err != nil {
		return fmt.Errorf("parsing expected booking payment deadline: %w", err)
	}
	fixture, exists := suite.serviceProposalFixtures[suite.lastServiceProposalID]
	if !exists {
		return fmt.Errorf("expected a prepared service proposal")
	}
	actual := fixture.scheduledOn.Add(-24 * time.Hour)
	if !actual.Equal(expectedTime.UTC()) {
		return fmt.Errorf("expected prepared proposal payment deadline %s, got %s", expectedTime.UTC(), actual)
	}
	return nil
}

func (suite *testSuite) firstApprovedPaymentConfirmedProposal() error {
	return suite.prepareConfirmedBooking()
}

func (suite *testSuite) approvedPaymentConfirmedProposal() error {
	return suite.prepareConfirmedBooking()
}

func (suite *testSuite) prepareConfirmedBooking() error {
	if err := suite.consumerStartedCheckoutForPendingProposal(defaultBookingConsumerEmail, defaultBookingProviderEmail); err != nil {
		return err
	}
	if err := suite.processApprovedPayment(); err != nil {
		return err
	}
	if err := suite.serviceProposalIsAccepted(); err != nil {
		return err
	}
	return suite.systemRegistersOneScheduledWorkOrder()
}

func (suite *testSuite) mercadoPagoCredentialExpired(providerEmail string) error {
	return godog.ErrPending
}

func (suite *testSuite) mercadoPagoCredentialCanBeRefreshed() error {
	return godog.ErrPending
}

func (suite *testSuite) mercadoPagoRejectsCredentialRefresh() error {
	return godog.ErrPending
}

func (suite *testSuite) requestPendingProposalCheckout() error {
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) consumerRequestsCheckoutAgain(consumerEmail string) error {
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) tryPayPendingProposalForConsumer(_ string) error {
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) tryPayPendingProposal() error {
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) tryPayAcceptedProposal() error {
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) tryPayRejectedProposal() error {
	return suite.requestPreparedProposalCheckout()
}

func (suite *testSuite) requestPreparedProposalCheckout() error {
	suite.previousPaymentIntentID = suite.lastPaymentIntentID
	response, err := suite.performCheckoutRequest(suite.lastServiceProposalID)
	if err != nil {
		return err
	}
	suite.lastStatus = response.Status
	suite.lastBody = response.Body
	if response.Value.PaymentIntentID != "" {
		suite.rememberCheckoutResponse(response.Value)
	}
	return nil
}

func (suite *testSuite) performCheckoutRequest(proposalID int) (checkoutHTTPResponse, error) {
	if proposalID == 0 {
		return checkoutHTTPResponse{}, fmt.Errorf("expected a prepared service proposal before requesting checkout")
	}
	request, err := http.NewRequest(
		http.MethodPost,
		suite.server.URL+fmt.Sprintf(bookingDepositCheckoutPath, proposalID),
		nil,
	)
	if err != nil {
		return checkoutHTTPResponse{}, err
	}
	if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return checkoutHTTPResponse{}, fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return checkoutHTTPResponse{}, fmt.Errorf("reading checkout response: %w", err)
	}
	result := checkoutHTTPResponse{Status: response.StatusCode, Body: body}
	if response.StatusCode == http.StatusCreated || response.StatusCode == http.StatusOK {
		result.Value, err = checkoutSessionResponseFromBody(body)
		if err != nil {
			return checkoutHTTPResponse{}, err
		}
	}
	return result, nil
}

func (suite *testSuite) requestCheckoutConcurrentlyTwice() error {
	results := make([]checkoutHTTPResponse, 2)
	errorsByRequest := make([]error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	for index := range results {
		go func(index int) {
			defer waitGroup.Done()
			results[index], errorsByRequest[index] = suite.performCheckoutRequest(suite.lastServiceProposalID)
		}(index)
	}
	waitGroup.Wait()
	for index, err := range errorsByRequest {
		if err != nil {
			return fmt.Errorf("checkout request %d failed: %w", index+1, err)
		}
		if results[index].Status != http.StatusCreated && results[index].Status != http.StatusOK {
			return fmt.Errorf("checkout request %d returned status %d with body %s", index+1, results[index].Status, results[index].Body)
		}
	}
	suite.concurrentCheckoutResponses = results
	suite.rememberCheckoutResponse(results[0].Value)
	return nil
}

func (suite *testSuite) processApprovedPaymentForAmount(amount string) error {
	amountCents, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}
	if suite.lastPaymentIntentID == "" {
		return fmt.Errorf("expected a prepared payment intent")
	}
	externalPaymentID := suite.checkoutClient.AddApprovedPayment(
		suite.lastPaymentIntentID,
		"mp-juan",
		amountCents,
	)
	return suite.sendMercadoPagoPaymentNotification(externalPaymentID)
}

func (suite *testSuite) sendMercadoPagoPaymentNotification(externalPaymentID string) error {
	requestID := "bdd-mercado-pago-request"
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	manifest := "id:" + strings.ToLower(externalPaymentID) +
		";request-id:" + requestID +
		";ts:" + timestamp + ";"
	signatureMAC := hmac.New(sha256.New, []byte(testMercadoPagoWebhookSecret))
	if _, err := signatureMAC.Write([]byte(manifest)); err != nil {
		return fmt.Errorf("signing fake Mercado Pago notification: %w", err)
	}
	signature := "ts=" + timestamp + ",v1=" + hex.EncodeToString(signatureMAC.Sum(nil))
	body, err := json.Marshal(map[string]any{
		"type":    "payment",
		"user_id": "mp-juan",
		"data": map[string]string{
			"id": externalPaymentID,
		},
	})
	if err != nil {
		return fmt.Errorf("encoding fake Mercado Pago notification: %w", err)
	}
	request, err := http.NewRequest(
		http.MethodPost,
		suite.server.URL+"/webhooks/mercado-pago?data.id="+url.QueryEscape(externalPaymentID)+"&type=payment",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-Id", requestID)
	request.Header.Set("X-Signature", signature)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("sending fake Mercado Pago notification: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading Mercado Pago notification response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = responseBody
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"expected Mercado Pago notification status 200, got %d with body %s",
			response.StatusCode,
			responseBody,
		)
	}
	return nil
}

func (suite *testSuite) processApprovedPayment() error {
	if suite.lastCheckoutResponse.Pricing.AmountDueNowCents <= 0 {
		return fmt.Errorf("expected checkout pricing before approving payment")
	}
	return suite.processApprovedPaymentForAmount(
		fmt.Sprintf("%.2f", float64(suite.lastCheckoutResponse.Pricing.AmountDueNowCents)/100),
	)
}

func (suite *testSuite) processProcessingPayment() error {
	if suite.lastPaymentIntentID == "" ||
		suite.lastCheckoutResponse.Pricing.AmountDueNowCents <= 0 {
		return fmt.Errorf("expected checkout pricing and payment intent before processing payment")
	}
	externalPaymentID := suite.checkoutClient.AddProcessingPayment(
		suite.lastPaymentIntentID,
		"mp-juan",
		suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
	)
	return suite.sendMercadoPagoPaymentNotification(externalPaymentID)
}

func (suite *testSuite) processSameApprovedPaymentNotificationTwice() error {
	return godog.ErrPending
}

func (suite *testSuite) processPaymentApprovedOn(approvedOn string) error {
	return godog.ErrPending
}

func (suite *testSuite) processSecondApprovedPayment() error {
	return godog.ErrPending
}

func (suite *testSuite) processPaymentReversal(reversal string) error {
	return godog.ErrPending
}

func (suite *testSuite) systemReturnsBookingCheckoutURL() error {
	return suite.assertCheckoutURL(false)
}

func (suite *testSuite) systemReturnsNewCheckoutURL() error {
	return suite.assertCheckoutURL(false)
}

func (suite *testSuite) systemReturnsHTTPSCheckoutProURL() error {
	return suite.assertCheckoutURL(true)
}

func (suite *testSuite) assertCheckoutURL(requireHTTPS bool) error {
	if suite.lastStatus != http.StatusCreated && suite.lastStatus != http.StatusOK {
		return fmt.Errorf("expected checkout response status 200 or 201, got %d with body %s", suite.lastStatus, suite.lastBody)
	}
	response, err := checkoutSessionResponseFromBody(suite.lastBody)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(response.CheckoutURL)
	if err != nil {
		return fmt.Errorf("parsing checkout_url: %w", err)
	}
	if parsed.IsAbs() == false || parsed.Host == "" {
		return fmt.Errorf("expected absolute checkout_url, got %q", response.CheckoutURL)
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return fmt.Errorf("expected HTTPS checkout_url, got %q", response.CheckoutURL)
	}
	suite.rememberCheckoutResponse(response)
	return nil
}

func (suite *testSuite) responseIdentifiesPaymentIntentWithStatus(expected string) error {
	return suite.assertCheckoutPaymentIntent(expected, false)
}

func (suite *testSuite) responseIdentifiesNewPaymentIntentWithStatus(expected string) error {
	return suite.assertCheckoutPaymentIntent(expected, true)
}

func (suite *testSuite) assertCheckoutPaymentIntent(expected string, mustBeNew bool) error {
	response, err := checkoutSessionResponseFromBody(suite.lastBody)
	if err != nil {
		return err
	}
	if response.PaymentIntentID == "" {
		return fmt.Errorf("expected checkout response to include payment_intent_id")
	}
	if response.Status != expected {
		return fmt.Errorf("expected checkout payment intent status %q, got %q", expected, response.Status)
	}
	if mustBeNew && response.PaymentIntentID == suite.previousPaymentIntentID {
		return fmt.Errorf("expected a new payment intent, got previous id %q", response.PaymentIntentID)
	}
	suite.rememberCheckoutResponse(response)
	return nil
}

func (suite *testSuite) checkoutResponseIncludesPricingBreakdown(table *godog.Table) error {
	response, err := checkoutSessionResponseFromBody(suite.lastBody)
	if err != nil {
		return err
	}
	expected, err := bookingPricingTableInCents(table)
	if err != nil {
		return err
	}
	actual := map[string]int64{
		"precio total del servicio":            response.Pricing.ServiceTotalCents,
		"seña del prestador":                   response.Pricing.DepositCents,
		"comisión total de LoResuelvo":         response.Pricing.PlatformFeeTotalCents,
		"comisión de LoResuelvo cobrada ahora": response.Pricing.PlatformFeeDueNowCents,
		"total a pagar ahora":                  response.Pricing.AmountDueNowCents,
		"saldo total a pagar más adelante":     response.Pricing.RemainingAmountDueCents,
	}
	for concept, expectedCents := range expected {
		actualCents, exists := actual[concept]
		if !exists {
			return fmt.Errorf("unsupported checkout pricing concept %q", concept)
		}
		if actualCents != expectedCents {
			return fmt.Errorf("expected %s to be %d cents, got %d", concept, expectedCents, actualCents)
		}
	}
	if response.Pricing.Currency != defaultBookingCurrency {
		return fmt.Errorf("expected checkout currency %q, got %q", defaultBookingCurrency, response.Pricing.Currency)
	}
	return nil
}

func (suite *testSuite) checkoutSessionExpiresOn(expected string) error {
	expectedTime, err := time.Parse(time.RFC3339, expected)
	if err != nil {
		return fmt.Errorf("parsing expected checkout expiration: %w", err)
	}
	response, err := checkoutSessionResponseFromBody(suite.lastBody)
	if err != nil {
		return err
	}
	if !response.ExpiresOn.Equal(expectedTime.UTC()) {
		return fmt.Errorf("expected checkout expiration %s, got %s", expectedTime.UTC(), response.ExpiresOn)
	}
	return nil
}

func (suite *testSuite) paymentIntentCanBeReadWithStatus(expected string) error {
	if suite.lastPaymentIntentID == "" {
		return fmt.Errorf("expected a prepared payment intent")
	}
	request, err := http.NewRequest(
		http.MethodGet,
		suite.server.URL+fmt.Sprintf(paymentIntentPath, url.PathEscape(suite.lastPaymentIntentID)),
		nil,
	)
	if err != nil {
		return err
	}
	if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading payment intent response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected payment intent status 200, got %d with body %s", response.StatusCode, body)
	}
	var intent paymentIntentResponse
	if err := json.Unmarshal(body, &intent); err != nil {
		return fmt.Errorf("decoding payment intent response: %w", err)
	}
	if intent.ID != suite.lastPaymentIntentID {
		return fmt.Errorf("expected payment intent id %q, got %q", suite.lastPaymentIntentID, intent.ID)
	}
	if intent.Status != expected {
		return fmt.Errorf("expected payment intent status %q, got %q", expected, intent.Status)
	}
	return nil
}

func (suite *testSuite) serviceProposalIsAccepted() error {
	return suite.serviceProposalHasStatus(suite.lastServiceProposalID, serviceproposal.StatusAccepted)
}

func (suite *testSuite) serviceProposalRemainsAccepted() error {
	return suite.serviceProposalIsAccepted()
}

func (suite *testSuite) serviceProposalRemainsPending() error {
	return suite.serviceProposalHasStatus(suite.lastServiceProposalID, serviceproposal.StatusPending)
}

func (suite *testSuite) serviceProposalHasStatus(proposalID int, expected serviceproposal.Status) error {
	fixture, exists := suite.serviceProposalFixtures[proposalID]
	if !exists {
		return fmt.Errorf("expected fixture for service proposal id %d", proposalID)
	}
	proposals, err := repositories.NewServiceProposalRepository(suite.database).FindByUserID(context.Background(), fixture.consumerID)
	if err != nil {
		return fmt.Errorf("finding service proposal fixture: %w", err)
	}
	for _, proposal := range proposals {
		if proposal.ID == proposalID {
			if proposal.Status != expected {
				return fmt.Errorf("expected service proposal %d status %q, got %q", proposalID, expected, proposal.Status)
			}
			return nil
		}
	}
	return fmt.Errorf("expected service proposal id %d", proposalID)
}

func (suite *testSuite) systemRegistersOneScheduledWorkOrder() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.ID == 0 {
		return fmt.Errorf("expected persisted work order id")
	}
	if order.Status != workorder.StatusScheduled {
		return fmt.Errorf("expected work order status %q, got %q", workorder.StatusScheduled, order.Status)
	}
	if order.AcceptedOn.IsZero() {
		return fmt.Errorf("expected work order accepted_on")
	}
	suite.rememberPersistedWorkOrder(order)
	return nil
}

func (suite *testSuite) systemKeepsOneWorkOrderForServiceProposal() error {
	return suite.systemRegistersOneScheduledWorkOrder()
}

func (suite *testSuite) systemDoesNotRegisterWorkOrderForServiceProposal() error {
	_, err := suite.workOrderRepository.FindByServiceProposalID(context.Background(), suite.lastServiceProposalID)
	if errors.Is(err, workorder.ErrDoesNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected no work order for service proposal %d", suite.lastServiceProposalID)
}

func (suite *testSuite) workOrderIsLinkedToAcceptedServiceProposal() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.ServiceProposalID() != suite.lastServiceProposalID {
		return fmt.Errorf("expected work order service proposal id %d, got %d", suite.lastServiceProposalID, order.ServiceProposalID())
	}
	return nil
}

func (suite *testSuite) workOrderKeepsAgreedTerms() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	fixture, exists := suite.serviceProposalFixtures[suite.lastServiceProposalID]
	if !exists {
		return fmt.Errorf("expected service proposal fixture %d", suite.lastServiceProposalID)
	}
	if order.ConsumerID() != fixture.consumerID ||
		order.ProviderID() != fixture.providerID ||
		order.Amount() != fixture.amountCents ||
		!order.ScheduledOn().Equal(fixture.scheduledOn.UTC()) ||
		order.Description() != fixture.description {
		return fmt.Errorf("persisted work order does not preserve the agreed service proposal terms")
	}
	return nil
}

func (suite *testSuite) providerReceivesAcceptedServiceProposalNotification(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	if event.Type != "notification.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "notification.created", event.Type)
	}
	notification := event.Notification
	providerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding expected notification provider: %w", err)
	}
	if notification.ID == 0 ||
		notification.UserID != providerID ||
		notification.Type != "service_proposal_accepted" ||
		notification.ResourceType != "service_proposal" ||
		notification.ResourceID != suite.lastServiceProposalID ||
		notification.ReadAt != nil ||
		notification.CreatedAt.IsZero() {
		return fmt.Errorf("unexpected accepted service proposal notification: %+v", notification)
	}
	return nil
}

func (suite *testSuite) systemDeniesBookingDepositPayment() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemRejectsCheckoutForAcceptedProposal() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemRejectsCheckoutForRejectedProposal() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemRejectsCheckoutAfterBookingDeadline() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemKeepsOneActivePaymentIntent() error {
	return godog.ErrPending
}

func (suite *testSuite) systemKeepsOneActiveCheckoutSession() error {
	return godog.ErrPending
}

func (suite *testSuite) concurrentRequestsReturnSameCheckoutURL() error {
	if len(suite.concurrentCheckoutResponses) != 2 {
		return fmt.Errorf("expected exactly two concurrent checkout responses")
	}
	first := suite.concurrentCheckoutResponses[0].Value
	second := suite.concurrentCheckoutResponses[1].Value
	if first.CheckoutURL == "" || first.CheckoutURL != second.CheckoutURL {
		return fmt.Errorf("expected both concurrent requests to return the same checkout URL, got %q and %q", first.CheckoutURL, second.CheckoutURL)
	}
	if first.PaymentIntentID == "" || first.PaymentIntentID != second.PaymentIntentID {
		return fmt.Errorf("expected both concurrent requests to identify the same payment intent")
	}
	return nil
}

func (suite *testSuite) systemRegistersOneTransactionForExternalPayment() error {
	return godog.ErrPending
}

func (suite *testSuite) systemRegistersPaymentIncident(expected string) error {
	return godog.ErrPending
}

func (suite *testSuite) systemRegistersPaymentIncidentForSecondPayment(expected string) error {
	return godog.ErrPending
}

func (suite *testSuite) providerPaymentAccountRemainsConnected() error {
	return suite.providerPaymentAccountHasStatus("connected")
}

func (suite *testSuite) systemReportsPaymentAccountReauthorizationRequired() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) providerPaymentAccountHasStatus(expected string) error {
	connection, err := suite.mercadoPagoConnectionForProvider(defaultBookingProviderEmail)
	if err != nil {
		return err
	}
	if connection.Status != expected {
		return fmt.Errorf("expected provider payment account status %q, got %q", expected, connection.Status)
	}
	return nil
}

func (suite *testSuite) systemDoesNotRegisterActiveCheckoutSession() error {
	return godog.ErrPending
}

func (suite *testSuite) workOrderHasStatus(expected string) error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if string(order.Status) != expected {
		return fmt.Errorf("expected work order status %q, got %q", expected, order.Status)
	}
	return nil
}

func (suite *testSuite) persistedWorkOrderForLastServiceProposal() (*workorder.WorkOrder, error) {
	order, err := suite.workOrderRepository.FindByServiceProposalID(context.Background(), suite.lastServiceProposalID)
	if err != nil {
		return nil, fmt.Errorf("finding work order for service proposal %d: %w", suite.lastServiceProposalID, err)
	}
	return order, nil
}

func (suite *testSuite) rememberPersistedWorkOrder(order *workorder.WorkOrder) {
	if order == nil {
		return
	}
	response := workOrderResponse{
		ID:                order.ID,
		ServiceProposalID: order.ServiceProposalID(),
		ConsumerID:        order.ConsumerID(),
		ProviderID:        order.ProviderID(),
		AmountCents:       order.Amount(),
		ScheduledOn:       order.ScheduledOn(),
		Description:       order.Description(),
		Status:            string(order.Status),
		AcceptedOn:        order.AcceptedOn,
	}
	suite.workOrdersByServiceProposalID[order.ServiceProposalID()] = []workOrderResponse{response}
}

func (suite *testSuite) workOrderForLastServiceProposal() (workOrderResponse, error) {
	if order, err := suite.persistedWorkOrderForLastServiceProposal(); err == nil {
		suite.rememberPersistedWorkOrder(order)
	}
	workOrders := suite.workOrdersByServiceProposalID[suite.lastServiceProposalID]
	if len(workOrders) != 1 {
		return workOrderResponse{}, fmt.Errorf(
			"expected exactly one work order for service proposal %d, got %d",
			suite.lastServiceProposalID,
			len(workOrders),
		)
	}
	return workOrders[0], nil
}

func (suite *testSuite) rememberCheckoutResponse(response checkoutSessionResponse) {
	suite.lastCheckoutResponse = response
	if response.PaymentIntentID != "" {
		suite.lastPaymentIntentID = response.PaymentIntentID
	}
}

func checkoutSessionResponseFromBody(body []byte) (checkoutSessionResponse, error) {
	var response checkoutSessionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return checkoutSessionResponse{}, fmt.Errorf("response is not a valid checkout session: %w", err)
	}
	return response, nil
}
