package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cucumber/godog"
)

func registerReportReviewSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^reporto la reseña de la orden con categoría "([^"]*)" y sin explicación$`, func(category string) error { return s.requestReviewReport(map[string]any{"category": category}) })
	sc.Step(`^reporto la reseña de la orden con categoría "([^"]*)" y explicación (vacía|compuesta solo por espacios)$`, func(category, form string) error {
		explanation := ""
		if form != "vacía" {
			explanation = "   "
		}
		return s.requestReviewReport(map[string]any{"category": category, "explanation": explanation})
	})
	sc.Step(`^reporto la reseña de la orden con categoría "([^"]*)" y una explicación de (\d+) caracteres$`, func(category string, n int) error {
		return s.requestReviewReport(map[string]any{"category": category, "explanation": strings.Repeat("ñ", n)})
	})
	sc.Step(`^intento reportar la reseña de la orden sin especificar categoría$`, func() error { return s.requestReviewReport(map[string]any{}) })
	sc.Step(`^intento reportar la reseña de la orden con categoría no admitida "([^"]*)"$`, func(category string) error { return s.requestReviewReport(map[string]any{"category": category}) })
	sc.Step(`^reporté la reseña de la orden con categoría "([^"]*)" y explicación "([^"]*)"$`, func(category, explanation string) error {
		return s.requestReviewReport(map[string]any{"category": category, "explanation": explanation})
	})
	sc.Step(`^el ingreso de ese reporte respondió con estado (\d+)$`, func(status int) error {
		if s.lastStatus != status {
			return fmt.Errorf("expected receipt status %d, got %d", status, s.lastStatus)
		}
		return nil
	})
	sc.Step(`^intento reportar la reseña de la orden con categoría "([^"]*)"$`, func(category string) error { return s.requestReviewReport(map[string]any{"category": category}) })
	sc.Step(`^la respuesta incluye el identificador y el estado pendiente del reporte$`, s.reviewReportReceiptIncludesIdentity)
	sc.Step(`^la respuesta no expone el texto original de la reseña "([^"]*)"$`, func(original string) error {
		if strings.Contains(string(s.lastBody), original) {
			return fmt.Errorf("receipt exposed original review")
		}
		return nil
	})
	sc.Step(`^la reseña conserva un solo reporte$`, s.reviewRetainsOneReport)
}
func (s *testSuite) requestReviewReport(body map[string]any) error {
	order, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	response, err := s.postJSONWithAuth(s.currentAuth0ID, fmt.Sprintf("/work-orders/%d/reviews/reports", order.ID()), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	s.lastStatus = response.StatusCode
	s.adminRequest.headers = response.Header.Clone()
	s.lastBody, err = io.ReadAll(response.Body)
	return err
}
func (s *testSuite) reviewReportReceiptIncludesIdentity() error {
	var receipt struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(s.lastBody, &receipt); err != nil {
		return err
	}
	if receipt.ID <= 0 || receipt.Status != "pending" {
		return fmt.Errorf("invalid review report receipt: %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) reviewRetainsOneReport() error {
	order, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	report, err := s.dependencies.Persistence.ReviewReportRepository.FindByWorkOrderID(context.Background(), order.ID())
	if err != nil {
		return err
	}
	if report.ID() <= 0 || report.Category() != "personal_data" {
		return fmt.Errorf("first review report was not retained")
	}
	return nil
}
