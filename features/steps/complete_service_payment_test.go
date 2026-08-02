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

const serviceBalanceCheckoutPath = "/work-orders/%d/checkout-sessions"

func registerCompleteServicePaymentSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^solicito completar el pago de la orden de trabajo$`, suite.requestServiceBalanceCheckout)
	sc.Step(`^el sistema entrega una URL para completar el checkout del saldo$`, suite.systemReturnsServiceBalanceCheckoutURL)
	sc.Step(`^la respuesta identifica el intento de pago del saldo en estado "([^"]*)"$`, suite.responseIdentifiesServiceBalanceIntentWithStatus)
	sc.Step(`^la orden de trabajo todavía no queda pagada por completo$`, suite.workOrderIsNotFullyPaid)
	sc.Step(`^el código de confirmación todavía no está disponible$`, suite.confirmationCodeIsNotAvailable)
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

func (suite *testSuite) systemReturnsServiceBalanceCheckoutURL() error {
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

func (suite *testSuite) confirmationCodeIsNotAvailable() error {
	var response map[string]any
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding service balance checkout response: %w", err)
	}
	if responseContainsKey(response, "confirmation_code") || responseContainsKey(response, "completion_code") {
		return fmt.Errorf("expected checkout response not to expose a confirmation code")
	}
	return nil
}

func responseContainsKey(value any, expected string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if key == expected || responseContainsKey(nested, expected) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if responseContainsKey(nested, expected) {
				return true
			}
		}
	}
	return false
}
