package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const (
	serviceBalanceCheckoutPath    = "/work-orders/%d/checkout-sessions"
	workOrderConfirmationCodePath = "/work-orders/%d/confirmation-code"
)

func registerCompleteServicePaymentSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" inició el checkout del saldo de la orden de trabajo$`, suite.consumerStartedServiceBalanceCheckout)
	sc.Step(`^que la orden de trabajo tiene un intento de pago del saldo rechazado$`, suite.workOrderHasRejectedServiceBalanceIntent)
	sc.Step(`^que el pago aprobado del saldo habilitó el código de confirmación de la orden de trabajo$`, suite.approvedServiceBalancePaymentEnabledConfirmationCode)
	sc.Step(`^solicito completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckout)
	sc.Step(`^solicito nuevamente completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckoutAgain)
	sc.Step(`^intento completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckout)
	sc.Step(`^intento consultar el código de confirmación de la orden de trabajo$`, suite.tryToGetWorkOrderConfirmationCode)
	sc.Step(`^solicito concurrentemente dos veces completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckoutConcurrentlyTwice)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado por "([^"]*)" pesos argentinos para ese saldo$`, suite.processApprovedServiceBalancePayment)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago (en proceso|rechazado) para ese saldo$`, suite.processNonApprovedServiceBalancePayment)
	sc.Step(`^el sistema procesa dos veces la misma notificación válida de Mercado Pago y verifica el pago aprobado del saldo$`, suite.processSameApprovedServiceBalanceNotificationTwice)
	sc.Step(`^el sistema entrega una URL para completar el checkout del saldo$`, suite.systemReturnsServiceBalanceCheckoutURL)
	sc.Step(`^el sistema entrega una URL para completar un nuevo checkout del saldo$`, suite.systemReturnsNewServiceBalanceCheckoutURL)
	sc.Step(`^el sistema deniega el pago del saldo$`, suite.systemDeniesServiceBalancePayment)
	sc.Step(`^el sistema deniega la consulta del código de confirmación$`, suite.systemDeniesConfirmationCodeQuery)
	sc.Step(`^el sistema rechaza el pago porque todavía no llegó la fecha y hora programadas$`, suite.systemRejectsServiceBalancePaymentBeforeScheduledTime)
	sc.Step(`^el sistema informa que la orden de trabajo ya está pagada por completo$`, suite.systemReportsWorkOrderAlreadyFullyPaid)
	sc.Step(`^la respuesta identifica el intento de pago del saldo en estado "([^"]*)"$`, suite.responseIdentifiesServiceBalanceIntentWithStatus)
	sc.Step(`^la respuesta identifica un nuevo intento de pago del saldo en estado "([^"]*)"$`, suite.responseIdentifiesNewServiceBalanceIntentWithStatus)
	sc.Step(`^el intento de pago del saldo puede consultarse en estado "([^"]*)"$`, suite.serviceBalanceIntentCanBeReadWithStatus)
	sc.Step(`^la orden de trabajo queda pagada por completo$`, suite.workOrderIsFullyPaid)
	sc.Step(`^la orden de trabajo todavía no queda pagada por completo$`, suite.workOrderIsNotFullyPaid)
	sc.Step(`^la orden de trabajo conserva el saldo pendiente$`, suite.workOrderKeepsPendingBalance)
	sc.Step(`^el consumidor puede consultar un código de confirmación vinculado a la orden de trabajo$`, suite.consumerCanGetWorkOrderConfirmationCode)
	sc.Step(`^el código de confirmación todavía no está disponible$`, suite.confirmationCodeIsNotAvailable)
	sc.Step(`^el sistema no registra una sesión de checkout del saldo$`, suite.systemDoesNotRegisterServiceBalanceCheckoutSession)
	sc.Step(`^el sistema no registra un nuevo intento de pago del saldo$`, suite.systemDoesNotRegisterNewServiceBalanceIntent)
	sc.Step(`^el sistema no registra una nueva sesión de checkout del saldo$`, suite.systemDoesNotRegisterNewServiceBalanceCheckoutSession)
	sc.Step(`^el consumidor conserva el código de confirmación de la orden de trabajo$`, suite.consumerKeepsWorkOrderConfirmationCode)
	sc.Step(`^el sistema conserva un único intento de pago activo para el saldo$`, suite.systemKeepsOneActiveServiceBalanceIntent)
	sc.Step(`^el sistema conserva una única sesión de checkout activa para el saldo$`, suite.systemKeepsOneActiveServiceBalanceCheckoutSession)
	sc.Step(`^el sistema conserva un único código de confirmación para la orden de trabajo$`, suite.systemKeepsOneWorkOrderConfirmationCode)
	sc.Step(`^el servicio todavía no queda confirmado como realizado$`, suite.serviceIsNotYetConfirmedAsPerformed)
}

func (suite *testSuite) workOrderHasRejectedServiceBalanceIntent() error {
	if err := suite.consumerStartedServiceBalanceCheckout("ana@example.com"); err != nil {
		return err
	}
	if err := suite.processNonApprovedServiceBalancePayment("rechazado"); err != nil {
		return err
	}
	return suite.serviceBalanceIntentCanBeReadWithStatus(string(payment.StatusRejected))
}

func (suite *testSuite) approvedServiceBalancePaymentEnabledConfirmationCode() error {
	if err := suite.consumerStartedServiceBalanceCheckout("ana@example.com"); err != nil {
		return err
	}
	externalPaymentID := suite.checkoutClient.AddApprovedPayment(
		suite.lastPaymentIntentID,
		"mp-juan",
		suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
	)
	if err := suite.sendMercadoPagoPaymentNotification(externalPaymentID); err != nil {
		return err
	}
	if err := suite.workOrderIsFullyPaid(); err != nil {
		return err
	}
	if err := suite.consumerCanGetWorkOrderConfirmationCode(); err != nil {
		return err
	}
	suite.previousConfirmationCode = suite.lastConfirmationCode
	return nil
}

func (suite *testSuite) consumerStartedServiceBalanceCheckout(consumerEmail string) error {
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.requestServiceBalanceCheckout(); err != nil {
		return err
	}
	return suite.responseIdentifiesServiceBalanceIntentWithStatus(string(payment.StatusCheckoutReady))
}

func (suite *testSuite) requestServiceBalanceCheckout() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	response, err := suite.performServiceBalanceCheckoutRequest(order.ID)
	if err != nil {
		return err
	}
	suite.lastStatus = response.Status
	suite.lastBody = response.Body
	return nil
}

func (suite *testSuite) performServiceBalanceCheckoutRequest(workOrderID int) (checkoutHTTPResponse, error) {
	request, err := http.NewRequest(
		http.MethodPost,
		suite.server.URL+fmt.Sprintf(serviceBalanceCheckoutPath, workOrderID),
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
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return checkoutHTTPResponse{}, fmt.Errorf("reading service balance checkout response: %w", err)
	}
	result := checkoutHTTPResponse{Status: response.StatusCode, Body: responseBody}
	if response.StatusCode == http.StatusCreated || response.StatusCode == http.StatusOK {
		result.Value, err = checkoutSessionResponseFromBody(responseBody)
		if err != nil {
			return checkoutHTTPResponse{}, err
		}
	}
	return result, nil
}

func (suite *testSuite) requestServiceBalanceCheckoutConcurrentlyTwice() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	results := make([]checkoutHTTPResponse, 2)
	errorsByRequest := make([]error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(results))
	for index := range results {
		go func(index int) {
			defer waitGroup.Done()
			results[index], errorsByRequest[index] = suite.performServiceBalanceCheckoutRequest(order.ID)
		}(index)
	}
	waitGroup.Wait()
	for index, requestErr := range errorsByRequest {
		if requestErr != nil {
			return fmt.Errorf("service balance checkout request %d failed: %w", index+1, requestErr)
		}
		if results[index].Status != http.StatusCreated && results[index].Status != http.StatusOK {
			return fmt.Errorf(
				"service balance checkout request %d returned status %d with body %s",
				index+1,
				results[index].Status,
				results[index].Body,
			)
		}
	}
	suite.concurrentCheckoutResponses = results
	suite.rememberCheckoutResponse(results[0].Value)
	return nil
}

func (suite *testSuite) requestServiceBalanceCheckoutAgain() error {
	suite.previousPaymentIntentID = suite.lastPaymentIntentID
	suite.previousCheckoutRequestCount = suite.checkoutClient.RequestCount()
	return suite.requestServiceBalanceCheckout()
}

func (suite *testSuite) systemReturnsServiceBalanceCheckoutURL() error {
	return suite.assertCheckoutURL(true)
}

func (suite *testSuite) systemReturnsNewServiceBalanceCheckoutURL() error {
	return suite.assertCheckoutURL(true)
}

func (suite *testSuite) systemDeniesServiceBalancePayment() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveError(); err != nil {
		return err
	}
	_, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if !errors.Is(err, payment.ErrIntentDoesNotExist) {
		return fmt.Errorf("expected denied checkout not to create a service balance intent, got %v", err)
	}
	return nil
}

func (suite *testSuite) systemRejectsServiceBalancePaymentBeforeScheduledTime() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding service balance checkout error response: %w", err)
	}
	if response.Error != payment.ErrServiceBalancePaymentNotAvailable.Error() {
		return fmt.Errorf(
			"expected service balance availability error %q, got %q",
			payment.ErrServiceBalancePaymentNotAvailable,
			response.Error,
		)
	}
	return nil
}

func (suite *testSuite) systemReportsWorkOrderAlreadyFullyPaid() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding fully paid work order response: %w", err)
	}
	if response.Error != payment.ErrWorkOrderAlreadyFullyPaid.Error() {
		return fmt.Errorf(
			"expected fully paid work order error %q, got %q",
			payment.ErrWorkOrderAlreadyFullyPaid,
			response.Error,
		)
	}
	return nil
}

func (suite *testSuite) responseIdentifiesServiceBalanceIntentWithStatus(expected string) error {
	if err := suite.assertCheckoutPaymentIntent(expected, false); err != nil {
		return err
	}
	intent, err := suite.paymentIntentRepository.FindByID(context.Background(), suite.lastPaymentIntentID)
	if err != nil {
		return err
	}
	if intent.Purpose != payment.PurposeServiceBalance {
		return fmt.Errorf("expected service balance payment purpose, got %q", intent.Purpose)
	}
	return nil
}

func (suite *testSuite) responseIdentifiesNewServiceBalanceIntentWithStatus(expected string) error {
	if err := suite.assertCheckoutPaymentIntent(expected, true); err != nil {
		return err
	}
	previousIntent, err := suite.paymentIntentRepository.FindByID(
		context.Background(),
		suite.previousPaymentIntentID,
	)
	if err != nil {
		return err
	}
	if previousIntent.Status != payment.StatusRejected {
		return fmt.Errorf("expected previous service balance intent to remain %q, got %q", payment.StatusRejected, previousIntent.Status)
	}
	intent, err := suite.paymentIntentRepository.FindByID(context.Background(), suite.lastPaymentIntentID)
	if err != nil {
		return err
	}
	if intent.Purpose != payment.PurposeServiceBalance {
		return fmt.Errorf("expected service balance payment purpose, got %q", intent.Purpose)
	}
	return nil
}

func (suite *testSuite) processApprovedServiceBalancePayment(amount string) error {
	return suite.processApprovedPaymentForAmount(amount)
}

func (suite *testSuite) processNonApprovedServiceBalancePayment(result string) error {
	if suite.lastPaymentIntentID == "" || suite.lastCheckoutResponse.Pricing.AmountDueNowCents <= 0 {
		return fmt.Errorf("expected service balance checkout before processing payment")
	}
	var externalPaymentID string
	switch result {
	case "en proceso":
		externalPaymentID = suite.checkoutClient.AddProcessingPayment(
			suite.lastPaymentIntentID,
			"mp-juan",
			suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
		)
	case "rechazado":
		externalPaymentID = suite.checkoutClient.AddRejectedPayment(
			suite.lastPaymentIntentID,
			"mp-juan",
			suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
		)
	default:
		return fmt.Errorf("unsupported service balance payment result %q", result)
	}
	return suite.sendMercadoPagoPaymentNotification(externalPaymentID)
}

func (suite *testSuite) processSameApprovedServiceBalanceNotificationTwice() error {
	if suite.lastPaymentIntentID == "" || suite.lastCheckoutResponse.Pricing.AmountDueNowCents <= 0 {
		return fmt.Errorf("expected service balance checkout before approving duplicate notification")
	}
	externalPaymentID := suite.checkoutClient.AddApprovedPayment(
		suite.lastPaymentIntentID,
		"mp-juan",
		suite.lastCheckoutResponse.Pricing.AmountDueNowCents,
	)
	if err := suite.sendMercadoPagoPaymentNotification(externalPaymentID); err != nil {
		return err
	}
	transaction, err := suite.paymentTransactionRepository.FindByExternalID(
		context.Background(),
		paymentaccount.PaymentProvider("mercado_pago"),
		externalPaymentID,
	)
	if err != nil {
		return err
	}
	suite.previousPaymentTransactionID = transaction.ID
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.CompletionAuthorization == nil {
		return fmt.Errorf("expected completion authorization after first approved notification")
	}
	suite.previousConfirmationCiphertext = append(
		[]byte(nil),
		order.CompletionAuthorization.CodeCiphertext()...,
	)
	suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	if err := suite.consumerCanGetWorkOrderConfirmationCode(); err != nil {
		return err
	}
	suite.previousConfirmationCode = suite.lastConfirmationCode
	return suite.sendMercadoPagoPaymentNotification(externalPaymentID)
}

func (suite *testSuite) serviceBalanceIntentCanBeReadWithStatus(expected string) error {
	return suite.paymentIntentCanBeReadWithStatus(expected)
}

func (suite *testSuite) workOrderIsFullyPaid() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.Status != workorder.StatusPaid {
		return fmt.Errorf("expected work order status %q, got %q", workorder.StatusPaid, order.Status)
	}
	if order.CompletionAuthorization == nil {
		return fmt.Errorf("expected fully paid work order to own a completion authorization")
	}
	return nil
}

func (suite *testSuite) workOrderIsNotFullyPaid() error {
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if err != nil {
		return err
	}
	if intent.Status == payment.StatusPaid {
		return fmt.Errorf("expected service balance payment to remain unpaid")
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.Status != workorder.StatusScheduled {
		return fmt.Errorf("expected work order status %q, got %q", workorder.StatusScheduled, order.Status)
	}
	return nil
}

func (suite *testSuite) workOrderKeepsPendingBalance() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.Status != workorder.StatusScheduled {
		return fmt.Errorf("expected work order status %q, got %q", workorder.StatusScheduled, order.Status)
	}
	if order.RemainingServiceBalance() <= 0 ||
		order.RemainingPlatformFee() <= 0 ||
		order.RemainingAmountDue() <= 0 {
		return fmt.Errorf("expected work order to preserve a positive pending balance")
	}
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if errors.Is(err, payment.ErrIntentDoesNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if intent.Status == payment.StatusPaid {
		return fmt.Errorf("expected service balance payment to remain unpaid")
	}
	if intent.SellerAmountCents != order.RemainingServiceBalance() ||
		intent.PlatformFeeCents != order.RemainingPlatformFee() ||
		intent.TotalAmountCents != order.RemainingAmountDue() {
		return fmt.Errorf("expected service balance intent to preserve the work order pending balance")
	}
	return nil
}

func (suite *testSuite) confirmationCodeIsNotAvailable() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.CompletionAuthorization != nil {
		return fmt.Errorf("expected work order not to own a completion authorization")
	}
	consumer, err := suite.userRepository.FindByID(context.Background(), order.ConsumerID())
	if err != nil {
		return fmt.Errorf("finding work order consumer: %w", err)
	}
	status, body, err := suite.getWorkOrderConfirmationCodeAs(consumer.AuthID())
	if err != nil {
		return err
	}
	if status != http.StatusConflict {
		return fmt.Errorf("expected unavailable confirmation code status 409, got %d with body %s", status, body)
	}
	return nil
}

func (suite *testSuite) tryToGetWorkOrderConfirmationCode() error {
	status, body, err := suite.getWorkOrderConfirmationCode()
	if err != nil {
		return err
	}
	suite.lastStatus = status
	suite.lastBody = body
	return nil
}

func (suite *testSuite) systemDeniesConfirmationCodeQuery() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding confirmation code error response: %w", err)
	}
	if response.Error != workorder.ErrOnlyConsumerCanViewConfirmationCode.Error() {
		return fmt.Errorf(
			"expected confirmation code ownership error %q, got %q",
			workorder.ErrOnlyConsumerCanViewConfirmationCode,
			response.Error,
		)
	}
	return nil
}

func (suite *testSuite) systemDoesNotRegisterServiceBalanceCheckoutSession() error {
	_, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if errors.Is(err, payment.ErrIntentDoesNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected no service balance checkout session to be persisted")
}

func (suite *testSuite) systemDoesNotRegisterNewServiceBalanceIntent() error {
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if err != nil {
		return err
	}
	if intent.ID != suite.previousPaymentIntentID {
		return fmt.Errorf("expected paid service balance intent %q to remain the latest, got %q", suite.previousPaymentIntentID, intent.ID)
	}
	if intent.Status != payment.StatusPaid {
		return fmt.Errorf("expected preserved service balance intent status %q, got %q", payment.StatusPaid, intent.Status)
	}
	return nil
}

func (suite *testSuite) systemDoesNotRegisterNewServiceBalanceCheckoutSession() error {
	requestCount := suite.checkoutClient.RequestCount()
	if requestCount != suite.previousCheckoutRequestCount {
		return fmt.Errorf(
			"expected checkout request count to remain %d, got %d",
			suite.previousCheckoutRequestCount,
			requestCount,
		)
	}
	return nil
}

func (suite *testSuite) consumerKeepsWorkOrderConfirmationCode() error {
	if suite.previousConfirmationCode == "" {
		return fmt.Errorf("expected confirmation code captured before retry")
	}
	if err := suite.consumerCanGetWorkOrderConfirmationCode(); err != nil {
		return err
	}
	if suite.lastConfirmationCode != suite.previousConfirmationCode {
		return fmt.Errorf("expected consumer confirmation code to remain unchanged")
	}
	return nil
}

func (suite *testSuite) systemKeepsOneActiveServiceBalanceIntent() error {
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if err != nil {
		return err
	}
	if intent.Status != payment.StatusCheckoutReady && intent.Status != payment.StatusProcessing {
		return fmt.Errorf("expected an active service balance intent, got %q", intent.Status)
	}
	if len(suite.concurrentCheckoutResponses) != 2 ||
		intent.ID != suite.concurrentCheckoutResponses[0].Value.PaymentIntentID ||
		intent.ID != suite.concurrentCheckoutResponses[1].Value.PaymentIntentID {
		return fmt.Errorf("expected both concurrent responses to reference the persisted service balance intent")
	}
	return nil
}

func (suite *testSuite) systemKeepsOneActiveServiceBalanceCheckoutSession() error {
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if err != nil {
		return err
	}
	if intent.CheckoutSession == nil || !intent.CheckoutSession.ExpiresOn.After(suite.clock.Now()) {
		return fmt.Errorf("expected one active service balance checkout session")
	}
	if suite.checkoutClient.RequestCount() != 1 {
		return fmt.Errorf("expected exactly one external checkout request, got %d", suite.checkoutClient.RequestCount())
	}
	return nil
}

func (suite *testSuite) systemKeepsOneWorkOrderConfirmationCode() error {
	if len(suite.previousConfirmationCiphertext) == 0 || suite.previousConfirmationCode == "" {
		return fmt.Errorf("expected confirmation authorization captured after first notification")
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.CompletionAuthorization == nil {
		return fmt.Errorf("expected work order completion authorization")
	}
	if string(order.CompletionAuthorization.CodeCiphertext()) != string(suite.previousConfirmationCiphertext) {
		return fmt.Errorf("expected duplicate notification to preserve the encrypted confirmation code")
	}
	if err := suite.consumerCanGetWorkOrderConfirmationCode(); err != nil {
		return err
	}
	if suite.lastConfirmationCode != suite.previousConfirmationCode {
		return fmt.Errorf("expected duplicate notification to preserve the consumer confirmation code")
	}
	return nil
}

func (suite *testSuite) consumerCanGetWorkOrderConfirmationCode() error {
	status, body, err := suite.getWorkOrderConfirmationCode()
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("expected confirmation code status 200, got %d with body %s", status, body)
	}
	var response struct {
		ConfirmationCode string `json:"confirmation_code"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decoding work order confirmation code response: %w", err)
	}
	if _, err := workorder.NewConfirmationCode(response.ConfirmationCode); err != nil {
		return fmt.Errorf("expected a four-digit work order confirmation code: %w", err)
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if order.CompletionAuthorization == nil {
		return fmt.Errorf("expected work order completion authorization")
	}
	if string(order.CompletionAuthorization.CodeCiphertext()) == response.ConfirmationCode {
		return fmt.Errorf("expected confirmation code to be encrypted at rest")
	}
	suite.lastConfirmationCode = response.ConfirmationCode
	return nil
}

func (suite *testSuite) getWorkOrderConfirmationCode() (int, []byte, error) {
	return suite.getWorkOrderConfirmationCodeAs(suite.currentAuth0ID)
}

func (suite *testSuite) getWorkOrderConfirmationCodeAs(auth0ID string) (int, []byte, error) {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return 0, nil, err
	}
	request, err := http.NewRequest(
		http.MethodGet,
		suite.server.URL+fmt.Sprintf(workOrderConfirmationCodePath, order.ID),
		nil,
	)
	if err != nil {
		return 0, nil, err
	}
	if auth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, nil))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("reading work order confirmation code response: %w", err)
	}
	return response.StatusCode, body, nil
}

func (suite *testSuite) serviceIsNotYetConfirmedAsPerformed() error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	intent, err := suite.paymentIntentRepository.FindLatestByProposalIDAndPurpose(
		context.Background(),
		suite.lastServiceProposalID,
		payment.PurposeServiceBalance,
	)
	if err != nil {
		return err
	}
	expectedStatus := workorder.StatusScheduled
	if intent.Status == payment.StatusPaid {
		expectedStatus = workorder.StatusPaid
	}
	if order.Status != expectedStatus {
		return fmt.Errorf("expected unconfirmed work order status %q, got %q", expectedStatus, order.Status)
	}
	return nil
}
