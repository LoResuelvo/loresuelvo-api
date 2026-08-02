package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
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
	sc.Step(`^solicito completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckout)
	sc.Step(`^solicito nuevamente completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckoutAgain)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado por "([^"]*)" pesos argentinos para ese saldo$`, suite.processApprovedServiceBalancePayment)
	sc.Step(`^el sistema procesa una notificación válida de Mercado Pago y verifica un pago (en proceso|rechazado) para ese saldo$`, suite.processNonApprovedServiceBalancePayment)
	sc.Step(`^el sistema entrega una URL para completar el checkout del saldo$`, suite.systemReturnsServiceBalanceCheckoutURL)
	sc.Step(`^el sistema entrega una URL para completar un nuevo checkout del saldo$`, suite.systemReturnsNewServiceBalanceCheckoutURL)
	sc.Step(`^la respuesta identifica el intento de pago del saldo en estado "([^"]*)"$`, suite.responseIdentifiesServiceBalanceIntentWithStatus)
	sc.Step(`^la respuesta identifica un nuevo intento de pago del saldo en estado "([^"]*)"$`, suite.responseIdentifiesNewServiceBalanceIntentWithStatus)
	sc.Step(`^el intento de pago del saldo puede consultarse en estado "([^"]*)"$`, suite.serviceBalanceIntentCanBeReadWithStatus)
	sc.Step(`^la orden de trabajo queda pagada por completo$`, suite.workOrderIsFullyPaid)
	sc.Step(`^la orden de trabajo todavía no queda pagada por completo$`, suite.workOrderIsNotFullyPaid)
	sc.Step(`^la orden de trabajo conserva el saldo pendiente$`, suite.workOrderKeepsPendingBalance)
	sc.Step(`^el consumidor puede consultar un código de confirmación vinculado a la orden de trabajo$`, suite.consumerCanGetWorkOrderConfirmationCode)
	sc.Step(`^el código de confirmación todavía no está disponible$`, suite.confirmationCodeIsNotAvailable)
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
	request, err := http.NewRequest(
		http.MethodPost,
		suite.server.URL+fmt.Sprintf(serviceBalanceCheckoutPath, order.ID),
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
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading service balance checkout response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = responseBody
	return nil
}

func (suite *testSuite) requestServiceBalanceCheckoutAgain() error {
	suite.previousPaymentIntentID = suite.lastPaymentIntentID
	return suite.requestServiceBalanceCheckout()
}

func (suite *testSuite) systemReturnsServiceBalanceCheckoutURL() error {
	return suite.assertCheckoutURL(true)
}

func (suite *testSuite) systemReturnsNewServiceBalanceCheckoutURL() error {
	return suite.assertCheckoutURL(true)
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
	if err := suite.workOrderIsNotFullyPaid(); err != nil {
		return err
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	intent, err := suite.paymentIntentRepository.FindByID(context.Background(), suite.lastPaymentIntentID)
	if err != nil {
		return err
	}
	if intent.SellerAmountCents != order.RemainingServiceBalance() ||
		intent.PlatformFeeCents != order.RemainingPlatformFee() ||
		intent.TotalAmountCents != order.RemainingAmountDue() {
		return fmt.Errorf("expected retry to preserve the work order pending balance")
	}
	return nil
}

func (suite *testSuite) confirmationCodeIsNotAvailable() error {
	status, body, err := suite.getWorkOrderConfirmationCode()
	if err != nil {
		return err
	}
	if status != http.StatusConflict {
		return fmt.Errorf("expected unavailable confirmation code status 409, got %d with body %s", status, body)
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
	return nil
}

func (suite *testSuite) getWorkOrderConfirmationCode() (int, []byte, error) {
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
	if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
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
