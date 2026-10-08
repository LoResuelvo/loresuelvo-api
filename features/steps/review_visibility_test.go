package steps_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func registerReviewVisibilitySteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que "([^"]*)" tiene dos trabajos pagados reseñados: "([^"]*)" con 5 estrellas, oculta y con el comentario "([^"]*)", y "([^"]*)" con 3 estrellas visibles$`, func(provider, first, text, second string) error {
		if provider != "juan@example.com" {
			return fmt.Errorf("unexpected provider")
		}
		if err := s.reviewFixture(first, 5, text, true); err != nil {
			return err
		}
		return s.reviewFixture(second, 3, "Visible review", false)
	})
	sc.Step(`^que "([^"]*)" tiene dos trabajos pagados con reseñas: una reseña oculta de 5 estrellas con el comentario "([^"]*)" y una reseña visible de 3 estrellas$`, func(provider, text string) error {
		if provider != "juan@example.com" {
			return fmt.Errorf("unexpected provider")
		}
		if err := s.reviewFixture("O1", 5, text, true); err != nil {
			return err
		}
		return s.reviewFixture("O2", 3, "Visible review", false)
	})
	sc.Step(`^consulto el perfil público del prestador "([^"]*)"$`, func(provider string) error {
		id, err := s.providerIDByEmail(provider)
		if err != nil {
			return err
		}
		return s.getProviderProfile(id)
	})
	sc.Step(`^el perfil incluye el trabajo pagado "([^"]*)" en su historial y no incluye el comentario "([^"]*)"$`, func(label, text string) error {
		var body struct {
			WorkOrders []struct {
				ID     int             `json:"id"`
				Review json.RawMessage `json:"review"`
			} `json:"work_orders"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if strings.Contains(string(s.lastBody), text) {
			return fmt.Errorf("public profile leaked hidden original")
		}
		for _, order := range body.WorkOrders {
			if order.ID == s.reviewState().orders[label] {
				if len(order.Review) > 0 && string(order.Review) != "null" {
					return fmt.Errorf("hidden public review was exposed")
				}
				return nil
			}
		}
		return fmt.Errorf("hidden review removed paid history work order")
	})
	sc.Step(`^el perfil informa dos calificaciones registradas y un promedio de 4,00$`, func() error {
		var body struct {
			Count   int     `json:"rating_count"`
			Average float64 `json:"rating_average"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if body.Count != 2 || body.Average != 4 {
			return fmt.Errorf("public ratings changed by hiding")
		}
		return nil
	})
	sc.Step(`^veo dos calificaciones registradas, una reseña visible y un promedio de 4,00$`, func() error {
		var body struct {
			Count   int               `json:"review_count"`
			Visible int               `json:"visible_review_count"`
			Average float64           `json:"average_rating"`
			Reviews []json.RawMessage `json:"reviews"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if s.lastStatus != 200 || body.Count != 2 || body.Visible != 1 || body.Average != 4 || len(body.Reviews) != 1 {
			return fmt.Errorf("private reputation counts changed: %s", s.lastBody)
		}
		return nil
	})
	sc.Step(`^la distribución conserva una calificación de 5 estrellas y una de 3 estrellas$`, func() error {
		var body struct {
			Distribution []struct{ Rating, Count int } `json:"rating_distribution"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		counts := map[int]int{}
		for _, bucket := range body.Distribution {
			counts[bucket.Rating] = bucket.Count
		}
		if counts[5] != 1 || counts[3] != 1 {
			return fmt.Errorf("hidden rating missing from distribution")
		}
		return nil
	})
	sc.Step(`^la respuesta de reputación no incluye el comentario oculto "([^"]*)"$`, func(text string) error {
		if strings.Contains(string(s.lastBody), text) {
			return fmt.Errorf("private reputation leaked hidden comment")
		}
		return nil
	})
	sc.Step(`^intento crear otra reseña para la orden "([^"]*)" con (\d+) estrellas y el comentario "([^"]*)"$`, func(label string, rating int, text string) error {
		return s.sendReviewPost(fmt.Sprintf("/work-orders/%d/reviews", s.reviewState().orders[label]), map[string]any{"rating": rating, "description": text})
	})

	sc.Step(`^el detalle administrativo conserva las 5 estrellas y no incluye el comentario original$`, func() error {
		if strings.Contains(string(s.lastBody), "Quedó impecable y limpio") {
			return fmt.Errorf("operation detail leaked hidden comment")
		}
		var body struct {
			WorkOrder struct {
				Review struct {
					Rating int `json:"rating"`
				} `json:"review"`
			} `json:"work_order"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if body.WorkOrder.Review.Rating != 5 {
			return fmt.Errorf("operation detail lost hidden rating: %s", s.lastBody)
		}
		return nil
	})
	sc.Step(`^el diagnóstico informa una calificación registrada y un promedio de 5,00, sin incluir el comentario original de "([^"]*)"$`, func(label string) error {
		if strings.Contains(string(s.lastBody), s.reviewState().originals[label]) {
			return fmt.Errorf("diagnostic leaked hidden comment")
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		var rating struct {
			Count   int     `json:"count"`
			Average float64 `json:"average"`
		}
		if err := json.Unmarshal(body["reputation"], &rating); err != nil {
			return err
		}
		if rating.Count != 1 || rating.Average != 5 {
			return fmt.Errorf("diagnostic lost hidden rating: %s", s.lastBody)
		}
		return nil
	})
	sc.Step(`^consulto el detalle propio de la orden "([^"]*)"$`, func(label string) error {
		return s.sendAdminGet("/work-orders/"+strconv.Itoa(s.reviewState().orders[label]), nil, "")
	})
	sc.Step(`^el detalle conserva las 5 estrellas y no incluye el comentario original "([^"]*)"$`, func(text string) error {
		var body struct {
			Review struct {
				Rating      int    `json:"rating"`
				Description string `json:"description"`
			} `json:"review"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if body.Review.Rating != 5 || body.Review.Description != "" || strings.Contains(string(s.lastBody), text) {
			return fmt.Errorf("participant detail leaked/dropped hidden rating")
		}
		return nil
	})
}
