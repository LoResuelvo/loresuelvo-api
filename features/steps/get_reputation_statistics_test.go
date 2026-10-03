package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const reputationStatisticsPath = "/providers/me/statistics/reputation"

type reputationReviewResponse struct {
	WorkOrderID int    `json:"work_order_id"`
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
type reputationDistributionResponse struct {
	Rating int   `json:"rating"`
	Count  int64 `json:"count"`
}
type reputationStatisticsResponse struct {
	CalculatedAt       time.Time                        `json:"calculated_at"`
	AverageRating      *float64                         `json:"average_rating"`
	ReviewCount        int64                            `json:"review_count"`
	RatingDistribution []reputationDistributionResponse `json:"rating_distribution"`
	EligiblePaidOrders int64                            `json:"eligible_paid_orders"`
	ReviewedPaidOrders int64                            `json:"reviewed_paid_orders"`
	CoveragePercentage *float64                         `json:"coverage_percentage"`
	Reviews            []reputationReviewResponse       `json:"reviews"`
	NextCursor         *string                          `json:"next_cursor"`
}
type reputationStatisticsState struct {
	nextFixture       int
	orders            []int
	labels            map[string]int
	reviews           map[int]reputationReviewResponse
	registrationOrder []int
	response          reputationStatisticsResponse
	responseValid     bool
	pages             []reputationStatisticsResponse
	cursors           map[string]string
	selectedCursor    string
	authorization     string
}

func (s *testSuite) reputationState() *reputationStatisticsState {
	if s.reputationStatistics == nil {
		s.reputationStatistics = &reputationStatisticsState{labels: make(map[string]int), reviews: make(map[int]reputationReviewResponse), cursors: make(map[string]string)}
	}
	return s.reputationStatistics
}

func registerGetReputationStatisticsSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que existe el consumidor "([^"]*)"$`, s.reputationConsumerExists)
	sc.Step(`^que tengo cinco trabajos pagados, con reseñas de 1, 2, 3, 4 y 5 estrellas$`, func() error { return s.reputationCreateOrders(5, []int{1, 2, 3, 4, 5}, "juan@example.com") })
	sc.Step(`^que tengo un trabajo pagado con una reseña de (\d+) estrellas$`, func(rating int) error { return s.reputationCreateOrders(1, []int{rating}, "juan@example.com") })
	sc.Step(`^que tengo dos trabajos pagados y uno todavía sin pagar$`, s.reputationPaidAndUnpaidOrders)
	sc.Step(`^que uno de mis trabajos pagados tiene una reseña de 4 estrellas$`, func() error { return s.reputationAddReview(s.reputationState().orders[0], 4, "Registered review") })
	sc.Step(`^que tengo dos trabajos pagados sin reseña$`, func() error { return s.reputationCreateOrders(2, nil, "juan@example.com") })
	sc.Step(`^que no tengo trabajos pagados$`, s.reputationNoPaidOrders)
	sc.Step(`^que tengo un trabajo pagado con una reseña de 4 estrellas y descripción vacía$`, s.reputationOrderWithEmptyReview)
	sc.Step(`^que tengo 32 trabajos pagados, de los cuales 8 tienen reseña$`, func() error { return s.reputationCreateOrders(32, []int{1, 4, 4, 4, 4, 4, 4, 4}, "juan@example.com") })
	sc.Step(`^las ocho calificaciones son una de 1 estrella y siete de 4 estrellas$`, s.reputationHalfAverageFixture)
	sc.Step(`^que tengo 32 trabajos pagados y uno tiene una reseña de 5 estrellas$`, func() error { return s.reputationCreateOrders(32, []int{5}, "juan@example.com") })
	sc.Step(`^que tengo tres trabajos pagados con reseñas de 2, 4 y 5 estrellas$`, func() error { return s.reputationCreateOrders(3, []int{2, 4, 5}, "juan@example.com") })
	sc.Step(`^que tengo tres trabajos pagados con reseñas, identificados como A, B y C$`, s.reputationNamedReviewedOrders)
	sc.Step(`^que el identificador de A es menor que el de B y el de B es menor que el de C$`, s.reputationNamedIDsIncrease)
	sc.Step(`^que las reseñas se registraron primero para C, luego para A y por último para B$`, s.reputationRegistrationWasCAB)
	sc.Step(`^que tengo (\d+) trabajos pagados, todos con reseña$`, s.reputationAllOrdersReviewed)
	sc.Step(`^que yo y "([^"]*)" tenemos tres trabajos pagados con reseña cada uno$`, s.reputationBothProvidersReviewed)
	sc.Step(`^obtuve una continuación real para mis reseñas después de pedir una página de una reseña$`, func() error { return s.reputationObtainCursor("juan@example.com") })
	sc.Step(`^"([^"]*)" obtuvo una continuación real para sus reseñas después de pedir una página de una reseña$`, s.reputationObtainCursor)
	sc.Step(`^vuelvo a estar autenticado como prestador "([^"]*)"$`, s.iAmAuthenticatedAsProvider)
	sc.Step(`^alteré la continuación de mi página anterior$`, s.reputationAlterCursor)
	sc.Step(`^uso la continuación real emitida para "([^"]*)"$`, s.reputationSelectCursor)
	sc.Step(`^uso mi continuación sin alterarla$`, func() error { return s.reputationSelectCursor("juan@example.com") })

	sc.Step(`^consulto mi reputación$`, s.reputationQuery)
	sc.Step(`^consulto mi reputación y sus reseñas$`, s.reputationQuery)
	sc.Step(`^consulto mis reseñas$`, s.reputationQuery)
	sc.Step(`^intento consultar mi reputación$`, s.reputationQuery)
	sc.Step(`^consulto mi reputación con una página de una reseña$`, func() error { return s.reputationRequest(url.Values{"limit": {"1"}}) })
	sc.Step(`^recorro mis reseñas con el tamaño de página predeterminado$`, s.reputationTraverseDefaultPages)
	sc.Step(`^consulto mis reseñas con un límite de (\d+)$`, s.reputationQueryWithLimit)
	sc.Step(`^intento consultar mis reseñas con un límite de (\d+)$`, s.reputationQueryWithLimit)
	sc.Step(`^intento continuar mis reseñas pidiendo una página de (una reseña|dos reseñas)$`, s.reputationContinue)

	sc.Step(`^veo (\d+) trabajos pagados elegibles y (\d+) con reseña$`, s.reputationCounts)
	sc.Step(`^veo (\d+) reseñas, un promedio de ([0-9,]+) y una cobertura del ([0-9,]+) %$`, s.reputationReviewCountAndMetrics)
	sc.Step(`^la distribución muestra una reseña para cada calificación de 1 a 5 estrellas$`, func() error { return s.reputationDistributionMatches([5]int64{1, 1, 1, 1, 1}) })
	sc.Step(`^la distribución incluye las cinco calificaciones de 1 a 5 estrellas$`, s.reputationFiveRatings)
	sc.Step(`^la calificación de 5 estrellas tiene una reseña y las demás tienen cero$`, func() error { return s.reputationDistributionMatches([5]int64{0, 0, 0, 0, 1}) })
	sc.Step(`^la suma de la distribución coincide con la cantidad total de reseñas$`, s.reputationDistributionAddsUp)
	sc.Step(`^veo (\d+) trabajos pagados elegibles, (\d+) con reseña, una cobertura del ([0-9,]+) % y un promedio de ([0-9,]+)$`, s.reputationCountsAndMetrics)
	sc.Step(`^la reseña del trabajo pagado aporta al promedio y no se cuenta el trabajo sin pagar$`, s.reputationExcludesUnpaid)
	sc.Step(`^veo (\d+) trabajos pagados elegibles, (\d+) con reseña y (\d+) reseñas$`, s.reputationAllCounts)
	sc.Step(`^la cobertura es 0,00 %, el promedio no tiene valor y la página de reseñas está vacía$`, s.reputationEligibleWithoutReviews)
	sc.Step(`^no hay otra página de reseñas para consultar$`, s.reputationNoNextPage)
	sc.Step(`^veo cero trabajos pagados elegibles, cero trabajos con reseña y cero reseñas$`, func() error { return s.reputationAllCounts(0, 0, 0) })
	sc.Step(`^las cinco calificaciones tienen cero reseñas$`, func() error { return s.reputationDistributionMatches([5]int64{}) })
	sc.Step(`^el promedio y la cobertura no tienen valor, la página está vacía y no hay otra página para consultar$`, s.reputationNoActivityMetrics)
	sc.Step(`^veo la reseña con su calificación y descripción vacía$`, s.reputationEmptyDescription)
	sc.Step(`^esa reseña cuenta para el promedio, la distribución y la cobertura$`, s.reputationEmptyDescriptionContributes)
	sc.Step(`^no se inventa una descripción para completar la reseña$`, s.reputationEmptyDescription)
	sc.Step(`^veo un promedio de ([0-9,]+) y una cobertura del ([0-9,]+) %$`, s.reputationMetrics)
	sc.Step(`^veo una cobertura del ([0-9,]+) % y un promedio de ([0-9,]+)$`, func(coverage, average string) error { return s.reputationMetrics(average, coverage) })
	sc.Step(`^la página contiene una reseña$`, func() error { return s.reputationPageCount(1) })
	sc.Step(`^los indicadores siguen considerando las (\d+) reseñas y muestran un promedio de ([0-9,]+)$`, s.reputationGlobalCountAndAverage)
	sc.Step(`^veo las reseñas de los trabajos C, B y A, en orden descendente de identificador$`, s.reputationNamedPageOrder)
	sc.Step(`^ese orden no depende del orden en que se registraron las reseñas$`, s.reputationOrderIsNotRegistrationOrder)
	sc.Step(`^cada reseña muestra el identificador del trabajo, la calificación y la descripción registrada$`, s.reputationPageMatchesPersistedReviews)
	sc.Step(`^la primera página contiene 20 reseñas y la siguiente contiene las 3 restantes$`, s.reputationDefaultPageSizes)
	sc.Step(`^recorro los (\d+) trabajos una sola vez en orden descendente de identificador$`, s.reputationTraversalMatchesOrders)
	sc.Step(`^los indicadores de ambas páginas muestran (\d+) reseñas y una cobertura del ([0-9,]+) %$`, s.reputationPagesKeepMetrics)
	sc.Step(`^la página contiene (\d+) reseñas$`, s.reputationPageCount)
	sc.Step(`^los indicadores consideran las (\d+) reseñas$`, s.reputationGlobalCount)
	sc.Step(`^se me informa que la consulta no es válida y no se muestran reseñas(?: ni indicadores)?$`, func() error { return s.reputationRejected(http.StatusBadRequest) })
	sc.Step(`^(se rechaza la consulta porque no inicié sesión|se rechaza la consulta porque mi sesión no es válida|se rechaza la consulta porque no soy prestador|se rechaza la consulta porque no tengo una cuenta en LoResuelvo) y no se muestran indicadores ni reseñas$`, s.reputationIdentityRejected)
}

func (s *testSuite) reputationConsumerExists(email string) error {
	s.reputationState()
	return s.thereIsRegisteredConsumerWithEmailNameAndSurname(email, "Consumer", "Reputation")
}
func (s *testSuite) reputationCreateOrders(count int, ratings []int, email string) error {
	if len(ratings) > count {
		return fmt.Errorf("more fixture reviews than eligible orders")
	}
	for index := 0; index < count; index++ {
		id, err := s.reputationCreateOrder(email, true)
		if err != nil {
			return err
		}
		if index < len(ratings) {
			if err := s.reputationAddReview(id, ratings[index], fmt.Sprintf("Registered review %d", id)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *testSuite) reputationCreateOrder(email string, paid bool) (int, error) {
	state := s.reputationState()
	state.nextFixture++
	label := fmt.Sprintf("reputation-order-%d", state.nextFixture)
	accepted := mustActivityDateTime("2026-09-01 10:00")
	reported := accepted.Add(48 * time.Hour)
	paidOn := reported.Add(time.Hour)
	if !paid {
		paidOn = time.Time{}
	}
	if err := s.createActivityOrder(label, "ana@example.com", email, 10000, 1000, accepted, reported, paidOn); err != nil {
		return 0, err
	}
	id, ok := s.operationInbox.orders[label]
	if !ok || id <= 0 {
		return 0, fmt.Errorf("reputation fixture order %q was not persisted", label)
	}
	if email == "juan@example.com" {
		state.orders = append(state.orders, id)
	}
	return id, nil
}
func (s *testSuite) reputationAddReview(id, rating int, description string) error {
	order, err := s.workOrderRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	found, err := s.userRepository.FindByAuthID(auth0IDForConsumerEmail("ana@example.com"))
	if err != nil {
		return err
	}
	reviewer, ok := found.(*consumer.Consumer)
	if !ok {
		return fmt.Errorf("reputation fixture reviewer is not a consumer")
	}
	review, err := workorder.NewReview(rating, description)
	if err != nil {
		return err
	}
	if err := order.AddReview(reviewer, review); err != nil {
		return err
	}
	if _, err := s.workOrderRepository.Save(s.scenarioContext, order); err != nil {
		return err
	}
	restored, err := s.workOrderRepository.FindByID(s.scenarioContext, id)
	if err != nil {
		return err
	}
	if restored.Review() == nil {
		return fmt.Errorf("review for order %d was not persisted", id)
	}
	state := s.reputationState()
	state.reviews[id] = reputationReviewResponse{WorkOrderID: id, Rating: restored.Review().Rating(), Description: restored.Review().Description()}
	state.registrationOrder = append(state.registrationOrder, id)
	return nil
}
func (s *testSuite) reputationPaidAndUnpaidOrders() error {
	if err := s.reputationCreateOrders(2, nil, "juan@example.com"); err != nil {
		return err
	}
	id, err := s.reputationCreateOrder("juan@example.com", false)
	if err == nil {
		s.reputationState().labels["unpaid"] = id
	}
	return err
}
func (s *testSuite) reputationNoPaidOrders() error {
	if len(s.reputationState().orders) != 0 {
		return fmt.Errorf("unexpected reputation order fixtures")
	}
	actor, err := s.userRepository.FindByAuthID(s.currentAuth0ID)
	if err != nil {
		return fmt.Errorf("finding reputation fixture actor: %w", err)
	}
	orders, err := s.workOrderRepository.FindByUserID(s.scenarioContext, actor.ID(), actor.Role())
	if err != nil {
		return fmt.Errorf("checking reputation fixture work orders: %w", err)
	}
	for _, order := range orders {
		if string(order.Status) == string(workorder.StatusPaid) {
			return fmt.Errorf("unexpected persisted paid work order %d", order.ID)
		}
	}
	return nil
}
func (s *testSuite) reputationOrderWithEmptyReview() error {
	id, err := s.reputationCreateOrder("juan@example.com", true)
	if err != nil {
		return err
	}
	return s.reputationAddReview(id, 4, "")
}
func (s *testSuite) reputationHalfAverageFixture() error {
	state := s.reputationState()
	if len(state.orders) != 32 || len(state.reviews) != 8 {
		return fmt.Errorf("half-average fixture has incorrect counts")
	}
	var counts [5]int
	for _, review := range state.reviews {
		counts[review.Rating-1]++
	}
	if counts != [5]int{1, 0, 0, 7, 0} {
		return fmt.Errorf("half-average fixture ratings are %v", counts)
	}
	return nil
}
func (s *testSuite) reputationNamedReviewedOrders() error {
	state := s.reputationState()
	for _, label := range []string{"A", "B", "C"} {
		id, err := s.reputationCreateOrder("juan@example.com", true)
		if err != nil {
			return err
		}
		state.labels[label] = id
	}
	for index, label := range []string{"C", "A", "B"} {
		if err := s.reputationAddReview(state.labels[label], index+3, "Registered review "+label); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) reputationNamedIDsIncrease() error {
	labels := s.reputationState().labels
	if !(labels["A"] > 0 && labels["A"] < labels["B"] && labels["B"] < labels["C"]) {
		return fmt.Errorf("named persisted IDs are not A < B < C: %v", labels)
	}
	return nil
}
func (s *testSuite) reputationRegistrationWasCAB() error {
	state := s.reputationState()
	want := []int{state.labels["C"], state.labels["A"], state.labels["B"]}
	if !reflect.DeepEqual(state.registrationOrder, want) {
		return fmt.Errorf("actual review registration order %v, want %v", state.registrationOrder, want)
	}
	return nil
}
func (s *testSuite) reputationAllOrdersReviewed(count int) error {
	ratings := make([]int, count)
	for index := range ratings {
		ratings[index] = index%5 + 1
	}
	return s.reputationCreateOrders(count, ratings, "juan@example.com")
}
func (s *testSuite) reputationBothProvidersReviewed(email string) error {
	if err := s.reputationCreateOrders(3, []int{1, 2, 3}, "juan@example.com"); err != nil {
		return err
	}
	return s.reputationCreateOrders(3, []int{3, 4, 5}, email)
}
func (s *testSuite) reputationQuery() error { return s.reputationRequest(url.Values{}) }
func (s *testSuite) reputationQueryWithLimit(limit int) error {
	return s.reputationRequest(url.Values{"limit": {strconv.Itoa(limit)}})
}
func (s *testSuite) reputationRequest(query url.Values) error {
	state := s.reputationState()
	state.response = reputationStatisticsResponse{}
	state.responseValid = false
	request, err := http.NewRequest(http.MethodGet, s.server.URL+reputationStatisticsPath+queryString(query), nil)
	if err != nil {
		return err
	}
	if s.invalidSession {
		request.Header.Set("Authorization", "Bearer invalid.reputation.token")
	} else if s.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+s.tokenBuilder.BuildToken(s.currentAuth0ID, s.currentPermissions))
	}
	state.authorization = request.Header.Get("Authorization")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	s.lastStatus = response.StatusCode
	s.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		return fmt.Errorf("reputation cache header is %q", response.Header.Get("Cache-Control"))
	}
	if response.StatusCode == http.StatusOK {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(s.lastBody, &raw); err != nil {
			return err
		}
		for _, key := range []string{"calculated_at", "average_rating", "review_count", "rating_distribution", "eligible_paid_orders", "reviewed_paid_orders", "coverage_percentage", "reviews", "next_cursor"} {
			if _, ok := raw[key]; !ok {
				return fmt.Errorf("reputation response lacks %q: %s", key, s.lastBody)
			}
		}
		var decoded reputationStatisticsResponse
		if err := json.Unmarshal(s.lastBody, &decoded); err != nil {
			return err
		}
		if decoded.CalculatedAt.IsZero() || decoded.Reviews == nil {
			return fmt.Errorf("reputation response lacks calculation instant or non-null reviews")
		}
		state.response = decoded
		state.responseValid = true
	}
	return nil
}
func (s *testSuite) reputationResponse() (reputationStatisticsResponse, error) {
	if s.lastStatus != http.StatusOK || !s.reputationState().responseValid {
		return reputationStatisticsResponse{}, fmt.Errorf("expected reputation 200, got %d: %s", s.lastStatus, s.lastBody)
	}
	return s.reputationState().response, nil
}
func (s *testSuite) reputationObtainCursor(email string) error {
	originalAuth, originalInvalid := s.currentAuth0ID, s.invalidSession
	defer func() { s.currentAuth0ID, s.invalidSession = originalAuth, originalInvalid }()
	if err := s.iAmAuthenticatedAsProvider(email); err != nil {
		return err
	}
	s.invalidSession = false
	if err := s.reputationQueryWithLimit(1); err != nil {
		return err
	}
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if len(response.Reviews) != 1 || response.NextCursor == nil || *response.NextCursor == "" {
		return fmt.Errorf("provider %q did not receive a real one-row continuation", email)
	}
	s.reputationState().cursors[email] = *response.NextCursor
	return nil
}
func (s *testSuite) reputationSelectCursor(email string) error {
	token, ok := s.reputationState().cursors[email]
	if !ok || token == "" {
		return fmt.Errorf("provider %q has no issued reputation cursor", email)
	}
	s.reputationState().selectedCursor = token
	return nil
}
func (s *testSuite) reputationAlterCursor() error {
	if err := s.reputationSelectCursor("juan@example.com"); err != nil {
		return err
	}
	state := s.reputationState()
	replacement := "A"
	if strings.HasPrefix(state.selectedCursor, "A") {
		replacement = "B"
	}
	state.selectedCursor = replacement + state.selectedCursor[1:]
	return nil
}
func (s *testSuite) reputationContinue(size string) error {
	limit := 1
	if size == "dos reseñas" {
		limit = 2
	}
	if s.reputationState().selectedCursor == "" {
		return fmt.Errorf("no continuation selected")
	}
	return s.reputationRequest(url.Values{"limit": {strconv.Itoa(limit)}, "cursor": {s.reputationState().selectedCursor}})
}
func (s *testSuite) reputationTraverseDefaultPages() error {
	state := s.reputationState()
	state.pages = nil
	query := url.Values{}
	seen := make(map[string]bool)
	for {
		if err := s.reputationRequest(query); err != nil {
			return err
		}
		response, err := s.reputationResponse()
		if err != nil {
			return err
		}
		state.pages = append(state.pages, response)
		if response.NextCursor == nil {
			return nil
		}
		token := *response.NextCursor
		if token == "" || seen[token] || len(state.pages) > len(state.orders)+1 {
			return fmt.Errorf("reputation pagination did not progress")
		}
		seen[token] = true
		query = url.Values{"cursor": {token}}
	}
}

func (s *testSuite) reputationCounts(eligible, reviewed int) error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if response.EligiblePaidOrders != int64(eligible) || response.ReviewedPaidOrders != int64(reviewed) {
		return fmt.Errorf("reputation paid counts = %d/%d, want %d/%d", response.EligiblePaidOrders, response.ReviewedPaidOrders, eligible, reviewed)
	}
	return nil
}
func (s *testSuite) reputationAllCounts(eligible, reviewed, reviews int) error {
	if err := s.reputationCounts(eligible, reviewed); err != nil {
		return err
	}
	return s.reputationGlobalCount(reviews)
}
func (s *testSuite) reputationGlobalCount(count int) error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if response.ReviewCount != int64(count) {
		return fmt.Errorf("reputation review_count=%d, want %d", response.ReviewCount, count)
	}
	return nil
}
func reputationFloatEquals(actual *float64, text string) error {
	expected, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", "."), 64)
	if err != nil {
		return err
	}
	if actual == nil || *actual != expected {
		return fmt.Errorf("reputation numeric value %s, want %s", activityFloatString(actual), text)
	}
	return nil
}
func (s *testSuite) reputationMetrics(average, coverage string) error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if err := reputationFloatEquals(response.AverageRating, average); err != nil {
		return fmt.Errorf("average: %w", err)
	}
	if err := reputationFloatEquals(response.CoveragePercentage, coverage); err != nil {
		return fmt.Errorf("coverage: %w", err)
	}
	return nil
}
func (s *testSuite) reputationReviewCountAndMetrics(count int, average, coverage string) error {
	if err := s.reputationGlobalCount(count); err != nil {
		return err
	}
	return s.reputationMetrics(average, coverage)
}
func (s *testSuite) reputationCountsAndMetrics(eligible, reviewed int, coverage, average string) error {
	if err := s.reputationCounts(eligible, reviewed); err != nil {
		return err
	}
	return s.reputationMetrics(average, coverage)
}
func (s *testSuite) reputationGlobalCountAndAverage(count int, average string) error {
	if err := s.reputationGlobalCount(count); err != nil {
		return err
	}
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	return reputationFloatEquals(response.AverageRating, average)
}
func (s *testSuite) reputationFiveRatings() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if len(response.RatingDistribution) != 5 {
		return fmt.Errorf("distribution has %d buckets, want 5", len(response.RatingDistribution))
	}
	for index, bucket := range response.RatingDistribution {
		if bucket.Rating != index+1 || bucket.Count < 0 {
			return fmt.Errorf("invalid distribution bucket %d: %+v", index, bucket)
		}
	}
	return nil
}
func (s *testSuite) reputationDistributionMatches(want [5]int64) error {
	if err := s.reputationFiveRatings(); err != nil {
		return err
	}
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	for index, bucket := range response.RatingDistribution {
		if bucket.Count != want[index] {
			return fmt.Errorf("rating %d count=%d, want %d", bucket.Rating, bucket.Count, want[index])
		}
	}
	return nil
}
func (s *testSuite) reputationDistributionAddsUp() error {
	if err := s.reputationFiveRatings(); err != nil {
		return err
	}
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	var total int64
	for _, bucket := range response.RatingDistribution {
		total += bucket.Count
	}
	if total != response.ReviewCount {
		return fmt.Errorf("distribution sum %d != review count %d", total, response.ReviewCount)
	}
	return nil
}
func (s *testSuite) reputationExcludesUnpaid() error {
	if err := s.reputationAllCounts(2, 1, 1); err != nil {
		return err
	}
	if err := s.reputationDistributionMatches([5]int64{0, 0, 0, 1, 0}); err != nil {
		return err
	}
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if len(response.Reviews) != 1 || response.Reviews[0].WorkOrderID != s.reputationState().orders[0] {
		return fmt.Errorf("page did not contain only the reviewed paid order: %+v", response.Reviews)
	}
	for _, review := range response.Reviews {
		if review.WorkOrderID == s.reputationState().labels["unpaid"] {
			return fmt.Errorf("unpaid order appears in reviews")
		}
	}
	return nil
}
func (s *testSuite) reputationEligibleWithoutReviews() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if err := reputationFloatEquals(response.CoveragePercentage, "0,00"); err != nil {
		return err
	}
	if response.AverageRating != nil || len(response.Reviews) != 0 {
		return fmt.Errorf("unreviewed eligible orders have an average or non-empty page")
	}
	return nil
}
func (s *testSuite) reputationNoNextPage() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if response.NextCursor != nil {
		return fmt.Errorf("terminal reputation page has a continuation")
	}
	return nil
}
func (s *testSuite) reputationNoActivityMetrics() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if response.AverageRating != nil || response.CoveragePercentage != nil || len(response.Reviews) != 0 {
		return fmt.Errorf("no-activity metrics are not null/empty")
	}
	return s.reputationNoNextPage()
}
func (s *testSuite) reputationEmptyDescription() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if len(response.Reviews) != 1 || response.Reviews[0].Rating != 4 || response.Reviews[0].Description != "" {
		return fmt.Errorf("empty-description review changed: %+v", response.Reviews)
	}
	return s.reputationPageMatchesPersistedReviews()
}
func (s *testSuite) reputationEmptyDescriptionContributes() error {
	if err := s.reputationAllCounts(1, 1, 1); err != nil {
		return err
	}
	if err := s.reputationMetrics("4,00", "100,00"); err != nil {
		return err
	}
	return s.reputationDistributionMatches([5]int64{0, 0, 0, 1, 0})
}
func (s *testSuite) reputationPageCount(count int) error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	if len(response.Reviews) != count {
		return fmt.Errorf("page has %d reviews, want %d", len(response.Reviews), count)
	}
	return nil
}
func (s *testSuite) reputationNamedPageOrder() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	labels := s.reputationState().labels
	want := []int{labels["C"], labels["B"], labels["A"]}
	actual := make([]int, len(response.Reviews))
	for index, review := range response.Reviews {
		actual[index] = review.WorkOrderID
	}
	if !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("page order %v, want %v", actual, want)
	}
	return nil
}
func (s *testSuite) reputationOrderIsNotRegistrationOrder() error {
	if err := s.reputationRegistrationWasCAB(); err != nil {
		return err
	}
	return s.reputationNamedPageOrder()
}
func (s *testSuite) reputationPageMatchesPersistedReviews() error {
	response, err := s.reputationResponse()
	if err != nil {
		return err
	}
	for _, review := range response.Reviews {
		expected, ok := s.reputationState().reviews[review.WorkOrderID]
		if !ok || review != expected {
			return fmt.Errorf("review %+v does not match persisted fixture %+v", review, expected)
		}
	}
	return nil
}
func (s *testSuite) reputationDefaultPageSizes() error {
	pages := s.reputationState().pages
	if len(pages) != 2 || len(pages[0].Reviews) != 20 || len(pages[1].Reviews) != 3 {
		return fmt.Errorf("default reputation pages have unexpected sizes")
	}
	if pages[0].NextCursor == nil || pages[1].NextCursor != nil {
		return fmt.Errorf("default pages have incorrect continuation markers")
	}
	return nil
}
func (s *testSuite) reputationTraversalMatchesOrders(count int) error {
	want := append([]int(nil), s.reputationState().orders...)
	sort.Sort(sort.Reverse(sort.IntSlice(want)))
	var actual []int
	for _, page := range s.reputationState().pages {
		for _, review := range page.Reviews {
			actual = append(actual, review.WorkOrderID)
		}
	}
	if len(want) != count || !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("traversed IDs %v, want %v", actual, want)
	}
	return nil
}
func (s *testSuite) reputationPagesKeepMetrics(count int, coverage string) error {
	pages := s.reputationState().pages
	if len(pages) != 2 {
		return fmt.Errorf("expected two real reputation pages")
	}
	for index, page := range pages {
		if page.ReviewCount != int64(count) || page.EligiblePaidOrders != int64(count) || page.ReviewedPaidOrders != int64(count) {
			return fmt.Errorf("page %d has page-local counts", index+1)
		}
		if err := reputationFloatEquals(page.CoveragePercentage, coverage); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(pages[0].RatingDistribution, pages[1].RatingDistribution) || !reflect.DeepEqual(pages[0].AverageRating, pages[1].AverageRating) {
		return fmt.Errorf("global reputation metrics changed across pages")
	}
	return nil
}
func (s *testSuite) reputationRejected(status int) error {
	if s.lastStatus != status {
		return fmt.Errorf("reputation status %d, want %d: %s", s.lastStatus, status, s.lastBody)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(s.lastBody, &body); err != nil {
		return err
	}
	var message string
	if err := json.Unmarshal(body["error"], &message); err != nil || message == "" {
		return fmt.Errorf("reputation rejection lacks controlled error: %s", s.lastBody)
	}
	for key, value := range body {
		if key == "error" {
			continue
		}
		if status == http.StatusUnauthorized && key == "message" {
			var detail string
			if err := json.Unmarshal(value, &detail); err != nil || detail == "" {
				return fmt.Errorf("authentication rejection has invalid message: %s", s.lastBody)
			}
			continue
		}
		return fmt.Errorf("reputation rejection exposes unexpected field %q: %s", key, s.lastBody)
	}
	if s.reputationState().responseValid {
		return fmt.Errorf("rejected response retained previous successful reputation data")
	}
	return nil
}
func (s *testSuite) reputationIdentityRejected(reason string) error {
	status := http.StatusUnauthorized
	switch reason {
	case "se rechaza la consulta porque no soy prestador":
		status = http.StatusForbidden
	case "se rechaza la consulta porque no tengo una cuenta en LoResuelvo":
		status = http.StatusNotFound
	case "se rechaza la consulta porque no inicié sesión":
		if s.reputationState().authorization != "" {
			return fmt.Errorf("no-session request unexpectedly sent Authorization")
		}
	case "se rechaza la consulta porque mi sesión no es válida":
		if s.reputationState().authorization != "Bearer invalid.reputation.token" {
			return fmt.Errorf("invalid-session request did not send the invalid token")
		}
	default:
		return fmt.Errorf("unsupported reputation rejection %q", reason)
	}
	return s.reputationRejected(status)
}
