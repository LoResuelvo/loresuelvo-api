package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type providerWorkHistoryResponse struct {
	RatingAverage float64                                `json:"rating_average"`
	RatingCount   int                                    `json:"rating_count"`
	WorkOrders    []providerWorkHistoryWorkOrderResponse `json:"work_orders"`
}

type providerWorkHistoryWorkOrderResponse struct {
	ID               int                       `json:"id"`
	ScheduledOn      time.Time                 `json:"scheduled_on"`
	Description      string                    `json:"description"`
	Status           string                    `json:"status"`
	CompletionReport *completionReportResponse `json:"completion_report,omitempty"`
	Review           *reviewAcceptanceResponse `json:"review,omitempty"`
}

func registerGetProviderWorkHistorySteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto el detalle público del prestador "([^"]*)"$`, suite.requestPublicProviderWorkHistory)
	sc.Step(`^el detalle informa un promedio de rating de ([0-9.]+)$`, suite.providerWorkHistoryShowsAverageRating)
	sc.Step(`^el detalle informa una cantidad de ratings de (\d+)$`, suite.providerWorkHistoryShowsRatingCount)
	sc.Step(`^el detalle de trabajos realizados es un arreglo vacío$`, suite.providerWorkHistoryIsEmpty)
	sc.Step(`^el historial incluye un trabajo con la descripción:$`, suite.providerWorkHistoryIncludesDescription)
	sc.Step(`^el historial no incluye un trabajo con la descripción:$`, suite.providerWorkHistoryExcludesDescription)
	sc.Step(`^el primer trabajo del historial tiene la descripción:$`, suite.providerWorkHistoryFirstDescription)
	sc.Step(`^el trabajo del historial incluye el reporte de finalización:$`, suite.providerWorkHistoryIncludesCompletionReport)
	sc.Step(`^el trabajo del historial incluye una review de (\d+) estrellas$`, suite.providerWorkHistoryIncludesReviewRating)
	sc.Step(`^el trabajo del historial incluye el comentario de review "([^"]*)"$`, suite.providerWorkHistoryIncludesReviewDescription)
	sc.Step(`^el historial no expone la identidad del consumidor "([^"]*)"$`, suite.providerWorkHistoryDoesNotExposeConsumer)
	sc.Step(`^el historial no expone el importe "([^"]*)"$`, suite.providerWorkHistoryDoesNotExposeAmount)
	sc.Step(`^el historial no expone imágenes de evidencia$`, suite.providerWorkHistoryDoesNotExposeEvidenceImages)
}

func (suite *testSuite) requestPublicProviderWorkHistory(providerEmail string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return fmt.Errorf("finding provider for public work history: %w", err)
	}

	return suite.getProviderProfile(providerID)
}

func (suite *testSuite) providerWorkHistoryShowsAverageRating(expectedText string) error {
	expected, err := strconv.ParseFloat(expectedText, 64)
	if err != nil {
		return fmt.Errorf("parsing expected provider rating average %q: %w", expectedText, err)
	}

	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}
	if math.Abs(response.RatingAverage-expected) > 0.000001 {
		return fmt.Errorf("expected provider rating average %v, got %v", expected, response.RatingAverage)
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryShowsRatingCount(expectedText string) error {
	expected, err := strconv.Atoi(expectedText)
	if err != nil {
		return fmt.Errorf("parsing expected provider rating count %q: %w", expectedText, err)
	}

	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.RatingCount != expected {
		return fmt.Errorf("expected provider rating count %d, got %d", expected, response.RatingCount)
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryIsEmpty() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &payload); err != nil {
		return fmt.Errorf("provider work history response is not valid JSON: %w", err)
	}

	rawWorkOrders, exists := payload["work_orders"]
	if !exists {
		return fmt.Errorf("provider work history response does not include work_orders")
	}
	if trimmed := bytes.TrimSpace(rawWorkOrders); len(trimmed) == 0 || trimmed[0] != '[' {
		return fmt.Errorf("provider work history work_orders is not an array")
	}

	var workOrders []json.RawMessage
	if err := json.Unmarshal(rawWorkOrders, &workOrders); err != nil {
		return fmt.Errorf("provider work history work_orders is not valid: %w", err)
	}
	if len(workOrders) != 0 {
		return fmt.Errorf("expected empty provider work history, got %d work orders", len(workOrders))
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryIncludesDescription(description *godog.DocString) error {
	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}

	expected := normalizeDocString(description)
	if _, err := providerWorkHistoryWorkOrderByDescription(response.WorkOrders, expected); err != nil {
		return fmt.Errorf("expected provider work history to include description %q: %w", expected, err)
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryExcludesDescription(description *godog.DocString) error {
	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}

	expected := normalizeDocString(description)
	if _, err := providerWorkHistoryWorkOrderByDescription(response.WorkOrders, expected); err == nil {
		return fmt.Errorf("expected provider work history not to include description %q", expected)
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryFirstDescription(description *godog.DocString) error {
	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}
	if len(response.WorkOrders) == 0 {
		return fmt.Errorf("expected provider work history to contain a first work order")
	}

	expected := normalizeDocString(description)
	if response.WorkOrders[0].Description != expected {
		return fmt.Errorf("expected first provider work history description %q, got %q", expected, response.WorkOrders[0].Description)
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryIncludesCompletionReport(description *godog.DocString) error {
	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}

	expected := normalizeDocString(description)
	for _, workOrder := range response.WorkOrders {
		if workOrder.CompletionReport != nil && workOrder.CompletionReport.Description == expected {
			return nil
		}
	}
	return fmt.Errorf("expected provider work history to include completion report %q", expected)
}

func (suite *testSuite) providerWorkHistoryIncludesReviewRating(expectedText string) error {
	expected, err := strconv.Atoi(expectedText)
	if err != nil {
		return fmt.Errorf("parsing expected provider review rating %q: %w", expectedText, err)
	}

	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, workOrder := range response.WorkOrders {
		if workOrder.Review != nil && workOrder.Review.Rating == expected {
			return nil
		}
	}
	return fmt.Errorf("expected provider work history to include a review with rating %d", expected)
}

func (suite *testSuite) providerWorkHistoryIncludesReviewDescription(expected string) error {
	response, err := suite.providerWorkHistoryResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, workOrder := range response.WorkOrders {
		if workOrder.Review != nil && workOrder.Review.Description == expected {
			return nil
		}
	}
	return fmt.Errorf("expected provider work history to include review description %q", expected)
}

func (suite *testSuite) providerWorkHistoryDoesNotExposeConsumer(identity string) error {
	workOrders, err := suite.providerWorkHistoryRawWorkOrders()
	if err != nil {
		return err
	}
	if strings.Contains(string(suite.lastBody), identity) || strings.Contains(string(suite.lastBody), "ana@example.com") {
		return fmt.Errorf("provider work history exposes consumer identity %q", identity)
	}

	for _, workOrder := range workOrders {
		for _, forbiddenField := range []string{"consumer", "consumer_id", "consumer_email", "consumer_name", "consumer_surname"} {
			if _, exists := workOrder[forbiddenField]; exists {
				return fmt.Errorf("provider work history exposes consumer field %q", forbiddenField)
			}
		}
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryDoesNotExposeAmount(amount string) error {
	workOrders, err := suite.providerWorkHistoryRawWorkOrders()
	if err != nil {
		return err
	}
	if strings.Contains(string(suite.lastBody), amount) {
		return fmt.Errorf("provider work history exposes amount %q", amount)
	}

	for _, workOrder := range workOrders {
		for _, forbiddenField := range []string{"amount", "amount_cents", "price", "balance", "paid_amount_cents"} {
			if _, exists := workOrder[forbiddenField]; exists {
				return fmt.Errorf("provider work history exposes amount field %q", forbiddenField)
			}
		}
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryDoesNotExposeEvidenceImages() error {
	workOrders, err := suite.providerWorkHistoryRawWorkOrders()
	if err != nil {
		return err
	}

	for _, workOrder := range workOrders {
		if field, exists := providerWorkHistoryFieldContaining(workOrder, "image", "evidence"); exists {
			return fmt.Errorf("provider work history exposes evidence field %q", field)
		}

		rawReport, exists := workOrder["completion_report"]
		if !exists || string(bytes.TrimSpace(rawReport)) == "null" {
			continue
		}
		var report map[string]json.RawMessage
		if err := json.Unmarshal(rawReport, &report); err != nil {
			return fmt.Errorf("provider work history completion report is not valid JSON: %w", err)
		}
		if field, exists := providerWorkHistoryFieldContaining(report, "image", "evidence"); exists {
			return fmt.Errorf("provider work history exposes completion evidence field %q", field)
		}
	}
	return nil
}

func (suite *testSuite) providerWorkHistoryResponseFromLastBody() (providerWorkHistoryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return providerWorkHistoryResponse{}, err
	}

	var response providerWorkHistoryResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return providerWorkHistoryResponse{}, fmt.Errorf("provider work history response is not valid JSON: %w", err)
	}
	return response, nil
}

func (suite *testSuite) providerWorkHistoryRawWorkOrders() ([]map[string]json.RawMessage, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, err
	}

	var payload struct {
		WorkOrders []map[string]json.RawMessage `json:"work_orders"`
	}
	if err := json.Unmarshal(suite.lastBody, &payload); err != nil {
		return nil, fmt.Errorf("provider work history response is not valid JSON: %w", err)
	}
	if len(payload.WorkOrders) == 0 {
		return nil, fmt.Errorf("expected provider work history to contain a work order")
	}
	return payload.WorkOrders, nil
}

func providerWorkHistoryWorkOrderByDescription(workOrders []providerWorkHistoryWorkOrderResponse, expected string) (providerWorkHistoryWorkOrderResponse, error) {
	for _, workOrder := range workOrders {
		if workOrder.Description == expected {
			return workOrder, nil
		}
	}
	return providerWorkHistoryWorkOrderResponse{}, fmt.Errorf("work order was not found")
}

func providerWorkHistoryFieldContaining(fields map[string]json.RawMessage, tokens ...string) (string, bool) {
	for field := range fields {
		lowerField := strings.ToLower(field)
		for _, token := range tokens {
			if strings.Contains(lowerField, token) {
				return field, true
			}
		}
	}
	return "", false
}
