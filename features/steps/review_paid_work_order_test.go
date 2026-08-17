package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const reviewCompletionImageName = "review-completion.jpg"

type reviewAcceptanceRequest struct {
	Rating      int    `json:"rating"`
	Description string `json:"description,omitempty"`
}

type reviewAcceptanceResponse struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}

type workOrderReviewDetailAcceptanceResponse struct {
	Review *reviewAcceptanceResponse `json:"review"`
}

func registerReviewPaidWorkOrderSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una orden de trabajo para reseña en estado "([^"]*)" para "([^"]*)" y "([^"]*)"$`, suite.thereIsReviewWorkOrderInState)
	sc.Step(`^que la orden ya tiene una reseña de (\d+) estrellas$`, suite.thereIsExistingReviewWithoutDescription)
	sc.Step(`^que la orden ya tiene una reseña de (\d+) estrellas con la descripción "([^"]*)"$`, suite.thereIsExistingReviewWithDescription)

	sc.Step(`^creo una reseña para la orden con (\d+) estrellas y la descripción:$`, suite.createReviewWithDescription)
	sc.Step(`^creo una reseña para la orden con (\d+) estrellas y sin descripción$`, suite.createReviewWithoutDescription)
	sc.Step(`^intento crear una reseña para la orden con (\d+) estrellas y la descripción "([^"]*)"$`, suite.tryCreateReview)
	sc.Step(`^intento crear una reseña para la orden con una descripción de más de 500 caracteres y (\d+) estrellas$`, suite.tryCreateReviewWithLongDescription)

	sc.Step(`^el sistema registra la reseña con (\d+) estrellas$`, suite.reviewIsRegisteredWithRating)
	sc.Step(`^la reseña registrada tiene la descripción:$`, suite.registeredReviewHasDescription)
	sc.Step(`^la reseña registrada no tiene descripción$`, suite.registeredReviewHasNoDescription)
	sc.Step(`^el sistema rechaza la reseña con estado (\d+)$`, suite.reviewIsRejectedWithStatus)

	sc.Step(`^el detalle incluye la reseña de (\d+) estrellas$`, suite.workOrderDetailIncludesReviewWithRating)
	sc.Step(`^el detalle incluye la descripción de la reseña "([^"]*)"$`, suite.workOrderDetailIncludesReviewDescription)
}

func (suite *testSuite) thereIsReviewWorkOrderInState(status, consumerEmail, providerEmail string) error {
	if status != string(workorder.StatusScheduled) && status != string(workorder.StatusAwaitingPayment) {
		return fmt.Errorf("unsupported review work order fixture status %q", status)
	}

	if _, err := suite.persistedWorkOrderForLastServiceProposal(); err != nil {
		return fmt.Errorf("finding review work order fixture for %q and %q: %w", consumerEmail, providerEmail, err)
	}

	if status == string(workorder.StatusScheduled) {
		return nil
	}

	return suite.prepareReviewAwaitingPayment(providerEmail)
}

func (suite *testSuite) prepareReviewAwaitingPayment(providerEmail string) error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}

	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	if err := suite.requestTestClockMock(order.ScheduledOn().UTC().Add(time.Minute).Format(time.RFC3339)); err != nil {
		return err
	}
	if err := suite.uploadAndConfirmCompletionImage(reviewCompletionImageName); err != nil {
		return err
	}
	if err := suite.reportCompletion(
		"Trabajo finalizado y funcionamiento verificado.",
		[]string{reviewCompletionImageName},
	); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating awaiting payment review fixture returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) thereIsExistingReviewWithoutDescription(rating string) error {
	return suite.createReviewFixture(rating, "")
}

func (suite *testSuite) thereIsExistingReviewWithDescription(rating, description string) error {
	return suite.createReviewFixture(rating, description)
}

func (suite *testSuite) createReviewFixture(ratingText, description string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}

	suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	if err := suite.requestReview(rating, description); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating existing review fixture returned status %d with body %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) createReviewWithDescription(ratingText string, description *godog.DocString) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}
	return suite.requestReview(rating, normalizeDocString(description))
}

func (suite *testSuite) createReviewWithoutDescription(ratingText string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}
	return suite.requestReview(rating, "")
}

func (suite *testSuite) tryCreateReview(ratingText, description string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}
	return suite.requestReview(rating, description)
}

func (suite *testSuite) tryCreateReviewWithLongDescription(ratingText string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing review rating %q: %w", ratingText, err)
	}
	return suite.requestReview(rating, strings.Repeat("a", 501))
}

func (suite *testSuite) requestReview(rating int, description string) error {
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}

	response, err := suite.postJSONWithAuth(
		suite.currentAuth0ID,
		fmt.Sprintf("/work-orders/%d/reviews", order.ID()),
		reviewAcceptanceRequest{Rating: rating, Description: description},
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	suite.lastStatus = response.StatusCode
	suite.lastLocation = response.Header.Get("Location")
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading review response: %w", err)
	}
	return nil
}

func (suite *testSuite) reviewIsRegisteredWithRating(ratingText string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing expected review rating %q: %w", ratingText, err)
	}
	review, err := suite.lastReviewResponse()
	if err != nil {
		return err
	}
	if review.Rating != rating {
		return fmt.Errorf("expected review rating %d, got %d", rating, review.Rating)
	}
	return nil
}

func (suite *testSuite) registeredReviewHasDescription(description *godog.DocString) error {
	review, err := suite.lastReviewResponse()
	if err != nil {
		return err
	}
	expected := normalizeDocString(description)
	if review.Description != expected {
		return fmt.Errorf("expected review description %q, got %q", expected, review.Description)
	}
	return nil
}

func (suite *testSuite) registeredReviewHasNoDescription() error {
	review, err := suite.lastReviewResponse()
	if err != nil {
		return err
	}
	if review.Description != "" {
		return fmt.Errorf("expected review without description, got %q", review.Description)
	}
	return nil
}

func (suite *testSuite) reviewIsRejectedWithStatus(statusText string) error {
	status, err := strconv.Atoi(statusText)
	if err != nil {
		return fmt.Errorf("parsing expected review status %q: %w", statusText, err)
	}
	return suite.lastResponseShouldHaveStatusCode(status)
}

func (suite *testSuite) lastReviewResponse() (reviewAcceptanceResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return reviewAcceptanceResponse{}, err
	}
	var review reviewAcceptanceResponse
	if err := json.Unmarshal(suite.lastBody, &review); err != nil {
		return reviewAcceptanceResponse{}, fmt.Errorf("review response is not valid JSON: %w", err)
	}
	return review, nil
}

func (suite *testSuite) workOrderDetailIncludesReviewWithRating(ratingText string) error {
	rating, err := strconv.Atoi(ratingText)
	if err != nil {
		return fmt.Errorf("parsing expected review rating %q: %w", ratingText, err)
	}
	review, err := suite.workOrderDetailReviewResponse()
	if err != nil {
		return err
	}
	if review.Rating != rating {
		return fmt.Errorf("expected work order detail review rating %d, got %d", rating, review.Rating)
	}
	return nil
}

func (suite *testSuite) workOrderDetailIncludesReviewDescription(expected string) error {
	review, err := suite.workOrderDetailReviewResponse()
	if err != nil {
		return err
	}
	if review.Description != expected {
		return fmt.Errorf("expected work order detail review description %q, got %q", expected, review.Description)
	}
	return nil
}

func (suite *testSuite) workOrderDetailReviewResponse() (reviewAcceptanceResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return reviewAcceptanceResponse{}, err
	}
	var detail workOrderReviewDetailAcceptanceResponse
	if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
		return reviewAcceptanceResponse{}, fmt.Errorf("work order detail response is not valid JSON: %w", err)
	}
	if detail.Review == nil {
		return reviewAcceptanceResponse{}, fmt.Errorf("expected work order detail to include a review, got body %s", suite.lastBody)
	}
	return *detail.Review, nil
}
