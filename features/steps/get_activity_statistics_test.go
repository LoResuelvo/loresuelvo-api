package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const activityStatisticsPath = "/providers/me/statistics/activity"

type activityStatisticsState struct {
	requestByPair     map[string]string
	proposalByOrder   map[string]string
	nextFixture       int
	lastQuery         url.Values
	response          activityStatisticsResponse
	responseValid     bool
	pendingBooking    time.Time
	scheduledOverride time.Time
}

type activityStatisticsResponse struct {
	Period struct {
		From        time.Time `json:"from"`
		To          time.Time `json:"to"`
		Granularity string    `json:"granularity"`
		TimeZone    string    `json:"time_zone"`
	} `json:"period"`
	CalculatedAt time.Time                  `json:"calculated_at"`
	Results      activityStatisticsTotals   `json:"results"`
	Evolution    []activityStatisticsBucket `json:"evolution"`
	Pending      activityStatisticsPending  `json:"current_pending"`
	Comparison   *activityStatisticsCompare `json:"comparison,omitempty"`
}

type activityStatisticsTotals struct {
	ConfirmedBookings   int    `json:"confirmed_bookings"`
	ReportedCompletions int    `json:"reported_completions"`
	FullyPaidWorkOrders int    `json:"fully_paid_work_orders"`
	ClientsServed       int    `json:"clients_served"`
	NewClients          int    `json:"new_clients"`
	ReturningClients    int    `json:"returning_clients"`
	AgreedValueCents    int64  `json:"agreed_value_cents"`
	AverageValueCents   *int64 `json:"average_value_cents"`
	Currency            string `json:"currency"`
}

type activityStatisticsBucket struct {
	From                time.Time `json:"from"`
	To                  time.Time `json:"to"`
	ConfirmedBookings   int       `json:"confirmed_bookings"`
	ReportedCompletions int       `json:"reported_completions"`
	FullyPaidWorkOrders int       `json:"fully_paid_work_orders"`
}

type activityStatisticsPending struct {
	Requests              int `json:"requests"`
	ScheduledOrders       int `json:"scheduled_orders"`
	AwaitingPaymentOrders int `json:"awaiting_payment_orders"`
}

type activityStatisticsCompare struct {
	Period struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`
	Results activityStatisticsTotals            `json:"results"`
	Changes map[string]activityStatisticsChange `json:"changes"`
}

type activityStatisticsChange struct {
	Absolute   *float64 `json:"absolute"`
	Percentage *float64 `json:"percentage"`
}

func registerGetActivityStatisticsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen los consumidores (.+)$`, suite.activityConsumersExist)
	sc.Step(`^que existen los prestadores (.+)$`, suite.activityProvidersExist)
	sc.Step(`^que hoy es 29 de septiembre de 2026 a las 12:00 en Buenos Aires$`, suite.activityClockIsSet)
	sc.Step(`^que existen estos trabajos con sus acuerdos aceptados:$`, suite.activityJobsExist)
	sc.Step(`^que informé la finalización de un trabajo para "([^"]+)" antes de septiembre de 2026$`, suite.activityOneCompletionBeforeSeptember)
	sc.Step(`^que informé la finalización de un trabajo para "([^"]+)" entre el 1 y el 3 de septiembre de 2026$`, suite.activityOneCompletionBetweenSeptemberFirstAndThird)
	sc.Step(`^que informé la finalización de un trabajo para "([^"]+)", uno para "([^"]+)" y dos para "([^"]+)" entre el 1 y el 3 de septiembre de 2026$`, suite.activityFourCompletionsAcrossClients)
	sc.Step(`^que "([^"]+)" informó la finalización de un trabajo para "([^"]+)" antes de septiembre, pero yo no había informado ninguno para ella$`, suite.activityOtherProviderCompletion)
	sc.Step(`^que durante los primeros nueve días de septiembre de 2026 me contrataron, informé la finalización y cobré dos trabajos de ARS 100,01 cada uno para "([^"]+)"$`, suite.activityTwoPaidWorkOrders)
	sc.Step(`^que el primer trabajo tiene tres fotos, una reseña y un intento de pago fallido antes del aprobado$`, suite.activityFirstWorkHasNoisyHistory)
	sc.Step(`^que el segundo tiene dos fotos y un intento de pago fallido antes del aprobado$`, suite.activitySecondWorkHasNoisyHistory)
	sc.Step(`^que me contrataron para un trabajo el (.+)$`, suite.activityOneBookingAt)
	sc.Step(`^que informé su finalización el (.+)$`, suite.activityOneCompletionAt)
	sc.Step(`^que me contrataron para otro trabajo el (.+), informé su finalización el (.+) y lo cobré a partir del (.+)$`, suite.activityBookingCompletionAndPayment)
	sc.Step(`^que cobré un tercer trabajo el (.+), cuya contratación se confirmó y cuya finalización informé antes de septiembre$`, suite.activityThirdPaymentAt)
	sc.Step(`^que entre el 8 y el 9 de septiembre de 2026 me contrataron, informé la finalización y cobré un trabajo de ARS 100,00 para un cliente$`, suite.activityOnePaidComparisonBaseline)
	sc.Step(`^que entre el 10 y el 11 de septiembre de 2026 me contrataron, informé la finalización y cobré dos trabajos de ARS 100,00 para dos clientes distintos$`, suite.activityTwoPaidComparisonCurrent)
	sc.Step(`^que no tuve contrataciones, finalizaciones informadas ni cobros el 8 y 9 de septiembre de 2026$`, suite.activityNoComparisonBaseline)
	sc.Step(`^que el 10 y 11 de septiembre de 2026 me contrataron, informé la finalización y cobré un trabajo de ARS 100,01$`, suite.activityOnePaidNoBaseline)
	sc.Step(`^que tengo una solicitud pendiente de "([^"]+)" recibida antes del 20 de septiembre de 2026$`, suite.activityPendingRequestBeforePeriod)
	sc.Step(`^que tengo otra solicitud pendiente de "([^"]+)" recibida el 23 de septiembre de 2026$`, suite.activityPendingRequestOnSeptember23)
	sc.Step(`^que todavía figura por realizar un trabajo programado para el (.+) y otro programado para el (.+), ambos acordados antes del 20 de septiembre$`, suite.activityScheduledOrders)
	sc.Step(`^que informé la finalización de un trabajo antes del 20 de septiembre de 2026 que todavía espera el pago$`, suite.activityAwaitingPaymentOrder)
	sc.Step(`^que "([^"]+)" tiene otra solicitud pendiente y otro trabajo que espera el pago$`, suite.activityOtherProviderPendingAndPayment)
	sc.Step(`^que no tengo solicitudes ni trabajos$`, suite.activityNoRequestsOrOrders)

	sc.Step(`^consulto mis resultados desde el (.+) hasta antes del (.+)$`, suite.activityQueryDateRange)
	sc.Step(`^consulto mi evolución diaria desde el (.+) hasta antes del (.+)$`, suite.activityQueryDailyDateRange)
	sc.Step(`^consulto mi evolución semanal desde el (.+) hasta el (.+)$`, suite.activityQueryWeeklyDateRange)
	sc.Step(`^consulto mi evolución mensual desde el (.+) hasta el (.+)$`, suite.activityQueryMonthlyDateRange)
	sc.Step(`^comparo mis resultados del (.+) con el período anterior$`, suite.activityQueryWithComparison)
	sc.Step(`^consulto mis resultados sin elegir un período$`, suite.activityQueryDefaultPeriod)
	sc.Step(`^intento consultar mis resultados (.+)$`, suite.activityTryQueryInvalidPeriod)
	sc.Step(`^intento consultar mis resultados$`, suite.activityTryQuery)
	sc.Step(`^que no inicié sesión$`, suite.activityNoSession)
	sc.Step(`^que mi sesión no es válida$`, suite.activityInvalidSession)
	sc.Step(`^que inicié sesión con una identidad que no figura en LoResuelvo$`, suite.activityUnknownIdentity)
	sc.Step(`^veo (\d+|una?) contrataciones confirmadas, (\d+|una?) finalizaciones informadas, (\d+|una?) cobros completos y (\d+|un|una) clientes? atendidos?$`, suite.activityResultsCounts)
	sc.Step(`^veo un cliente nuevo y uno recurrente$`, suite.activityOneNewAndReturningClient)
	sc.Step(`^el valor pactado de los trabajos cuya finalización informé es ARS ([0-9,.]+) y su promedio es ARS ([0-9,.]+), sin sumar las comisiones$`, suite.activityValueAndAverage)
	sc.Step(`^el trabajo cobrado el (\d+) de septiembre cuenta como cobro del período, pero no como finalización informada en el período$`, suite.activityPaymentIndependentFromCompletion)
	sc.Step(`^no aparecen los resultados de "([^"]+)"$`, suite.activityResultsExcludeProvider)
	sc.Step(`^no veo una comparación con un período anterior$`, suite.activityHasNoComparison)
	sc.Step(`^veo (\d+) finalizaciones informadas y (\d+) clientes atendidos$`, suite.activityCompletionAndClientCounts)
	sc.Step(`^veo (\d+) clientes recurrentes y (\d+) nuevo, sin contar dos veces a quien tuvo más de un trabajo$`, suite.activityReturningAndNewClients)
	sc.Step(`^veo un valor pactado de ARS ([0-9,.]+) y un promedio de ARS ([0-9,.]+)$`, suite.activityValueAndAverage)
	sc.Step(`^veo exactamente estos días, en orden:$`, suite.activityEvolutionMatches)
	sc.Step(`^no veo cobros completos en el total de esos tres días$`, suite.activityNoPaymentsInPeriod)
	sc.Step(`^veo una contratación en el tramo desde (.+) hasta (.+)$`, suite.activityBookingInBucket)
	sc.Step(`^veo una finalización informada en el tramo desde (.+) hasta (.+)$`, suite.activityCompletionInBucket)
	sc.Step(`^no veo otros tramos en el período elegido$`, suite.activityNoOtherBuckets)
	sc.Step(`^la comparación corresponde al (.+)$`, suite.activityComparisonPeriodIs)
	sc.Step(`^veo estas diferencias entre el período elegido y el anterior:$`, suite.activityComparisonMatches)
	sc.Step(`^veo una contratación, una finalización informada, un cobro completo y un cliente atendido más que antes$`, suite.activityComparisonOneMore)
	sc.Step(`^veo ARS ([0-9,.]+) más de valor pactado que antes$`, suite.activityComparisonValueIncrease)
	sc.Step(`^no se calcula un cambio porcentual para ninguno de esos resultados porque antes eran cero$`, suite.activityZeroBaselinePercentages)
	sc.Step(`^el promedio anterior y sus diferencias se muestran sin valor, en lugar de inventar un promedio$`, suite.activityAverageUndefined)
	sc.Step(`^mis resultados de esos dos días son cero y el promedio no tiene valor$`, suite.activityPeriodIsEmpty)
	sc.Step(`^como no elegí otra forma de agruparlos, veo los dos días sin movimiento en mi evolución diaria$`, suite.activityTwoEmptyDailyBuckets)
	sc.Step(`^veo (\d+) solicitudes pendientes, (\d+) trabajos por realizar y (\d+) trabajo que espera el pago$`, suite.activityPendingCounts)
	sc.Step(`^no veo los pendientes de "([^"]+)"$`, suite.activityOtherProviderPendingExcluded)
	sc.Step(`^veo mis resultados desde el (.+) hasta el (.+)$`, suite.activityDefaultPeriodMatches)
	sc.Step(`^los resultados y pendientes son cero, mientras que el promedio no tiene valor$`, suite.activityEmptyResultsAndPending)
	sc.Step(`^como no elegí otra forma de agruparlos, veo (\d+) días calendario sin movimiento, incluidos los días parciales del comienzo y del final$`, suite.activityEmptyDayCount)
	sc.Step(`^se me informa que el período no es válido y no se muestran resultados$`, suite.activityPeriodRejected)
	sc.Step(`^(se rechaza la consulta porque no inicié sesión|se rechaza la consulta porque mi sesión no es válida|se rechaza la consulta porque no soy prestador|se rechaza la consulta porque no tengo una cuenta en LoResuelvo) y no se muestran resultados$`, suite.activityIdentityRejected)
}

func (suite *testSuite) activityState() *activityStatisticsState {
	suite.operationInbox.ensureMaps()
	if suite.activityStatistics == nil {
		suite.activityStatistics = &activityStatisticsState{
			requestByPair: make(map[string]string), proposalByOrder: make(map[string]string),
		}
	}
	return suite.activityStatistics
}

func (suite *testSuite) activityConsumersExist(quotedEmails string) error {
	for _, email := range quotedEmailList(quotedEmails) {
		if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(email, "Consumidor", activityNamePart(email)); err != nil {
			return fmt.Errorf("registering activity consumer %q: %w", email, err)
		}
	}
	return nil
}

func (suite *testSuite) activityProvidersExist(quotedEmails string) error {
	if err := suite.thereIsCategoryNamed("Plomería"); err != nil {
		return fmt.Errorf("preparing provider activity category: %w", err)
	}
	for _, email := range quotedEmailList(quotedEmails) {
		if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(email, "Prestador", activityNamePart(email), "Plomería"); err != nil {
			return fmt.Errorf("registering activity provider %q: %w", email, err)
		}
	}
	return nil
}

func quotedEmailList(value string) []string {
	parts := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(value, -1)
	emails := make([]string, 0, len(parts))
	for _, part := range parts {
		emails = append(emails, part[1])
	}
	return emails
}

func activityNamePart(email string) string {
	local, _, _ := strings.Cut(email, "@")
	return strings.ToUpper(local[:1]) + local[1:]
}

func (suite *testSuite) activityClockIsSet() error {
	return suite.requestTestClockMock("2026-09-29T12:00:00-03:00")
}

func activityCurrencyToCents(value string) (int64, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "ARS "))
	value = strings.ReplaceAll(value, ".", "")
	parts := strings.Split(value, ",")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid Argentine peso amount %q", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing Argentine peso amount %q: %w", value, err)
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) != 2 {
		return 0, fmt.Errorf("amount %q must have two decimal places", value)
	}
	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil || cents > 99 {
		return 0, fmt.Errorf("invalid cents in amount %q", value)
	}
	return whole*100 + cents, nil
}

func activityDateTime(value string) (time.Time, error) {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.Time{}, fmt.Errorf("loading Buenos Aires time zone: %w", err)
	}
	formats := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range formats {
		if parsed, parseErr := time.ParseInLocation(layout, value, location); parseErr == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported activity fixture date %q", value)
}

func (suite *testSuite) activityJobsExist(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	for _, row := range rows {
		accepted, err := activityDateTime(row["contratación confirmada"])
		if err != nil {
			return err
		}
		reported := time.Time{}
		if row["finalización informada"] != "" {
			reported, err = activityDateTime(row["finalización informada"])
			if err != nil {
				return err
			}
		}
		paid := time.Time{}
		if row["cobrado"] != "" {
			paid, err = activityDateTime(row["cobrado"])
			if err != nil {
				return err
			}
		}
		amount, err := activityCurrencyToCents(row["precio"])
		if err != nil {
			return err
		}
		fee, err := activityCurrencyToCents(row["comisión"])
		if err != nil {
			return err
		}
		if err := suite.createActivityOrder(row["trabajo"], row["cliente"], row["prestador"], amount, fee, accepted, reported, paid); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) createActivityOrder(label, consumerEmail, providerEmail string, amount, fee int64, acceptedOn, reportedOn, paidOn time.Time) error {
	return suite.createActivityOrderWithImageCount(label, consumerEmail, providerEmail, amount, fee, acceptedOn, reportedOn, paidOn, 1)
}

func (suite *testSuite) createActivityOrderWithImageCount(label, consumerEmail, providerEmail string, amount, fee int64, acceptedOn, reportedOn, paidOn time.Time, imageCount int) error {
	auth0ID, permissions := suite.currentAuth0ID, suite.currentPermissions
	defer func() {
		suite.currentAuth0ID, suite.currentPermissions = auth0ID, permissions
	}()
	state := suite.activityState()
	requestKey := consumerEmail + "\x00" + providerEmail
	requestLabel, exists := state.requestByPair[requestKey]
	if !exists {
		state.nextFixture++
		requestLabel = fmt.Sprintf("activity-request-%d", state.nextFixture)
		requestCreated := acceptedOn.AddDate(0, 0, -367)
		if err := suite.createInboxJobRequest(requestLabel, consumerEmail, providerEmail, requestCreated, "pending"); err != nil {
			return err
		}
		request := suite.operationInbox.requests[requestLabel]
		if err := suite.setInboxFixtureClock(acceptedOn.AddDate(0, 0, -365)); err != nil {
			return err
		}
		suite.lastJobRequestID = request.id
		suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
		if err := suite.requestAcceptPendingJobRequest(); err != nil {
			return err
		}
		if suite.lastStatus != http.StatusOK {
			return fmt.Errorf("accepting activity request %q returned %d: %s", requestLabel, suite.lastStatus, suite.lastBody)
		}
		state.requestByPair[requestKey] = requestLabel
	}

	state.nextFixture++
	proposalLabel := fmt.Sprintf("activity-proposal-%d", state.nextFixture)
	orderLabel := label
	scheduledOn := acceptedOn.Add(48 * time.Hour)
	if !reportedOn.IsZero() {
		scheduledOn = acceptedOn.Add(time.Hour)
	}
	if !state.scheduledOverride.IsZero() {
		scheduledOn = state.scheduledOverride
		state.scheduledOverride = time.Time{}
	}
	if !acceptedOn.Before(scheduledOn) {
		return fmt.Errorf("activity fixture %q is accepted at or after its scheduled time", label)
	}
	if !reportedOn.IsZero() && reportedOn.After(scheduledOn) {
		scheduledOn = reportedOn.Add(-time.Hour)
	}
	if !reportedOn.IsZero() && scheduledOn.After(reportedOn) {
		return fmt.Errorf("activity fixture %q reports completion before its scheduled time", label)
	}
	proposalCreated := acceptedOn.Add(-48 * time.Hour)
	if proposalCreated.After(scheduledOn.Add(-24*time.Hour)) || proposalCreated.Equal(scheduledOn.Add(-24*time.Hour)) {
		proposalCreated = scheduledOn.Add(-48 * time.Hour)
	}
	request := suite.operationInbox.requests[requestLabel]
	row := map[string]string{
		"propuesta": proposalLabel, "solicitud": requestLabel,
		"creada": proposalCreated.UTC().Format(time.RFC3339), "fecha programada": scheduledOn.UTC().Format(time.RFC3339),
		"duración": "60", "moneda": "ARS", "precio total": strconv.FormatInt(amount, 10),
		"seña":             strconv.FormatInt(maxActivityInt64(1, amount/2), 10),
		"comisión total":   strconv.FormatInt(fee, 10),
		"comisión inicial": strconv.FormatInt(fee/2, 10),
		"saldo servicio":   strconv.FormatInt(amount-maxActivityInt64(1, amount/2), 10),
		"saldo comisión":   strconv.FormatInt(fee-fee/2, 10),
		"descripción":      "Trabajo para estadísticas", "estado": "pending",
	}
	if proposalCreated.Before(request.createdOn) {
		return fmt.Errorf("activity proposal %q predates its shared request", label)
	}
	if err := suite.createDetailProposal(row); err != nil {
		return err
	}
	state.proposalByOrder[orderLabel] = proposalLabel
	status, reportedValue, paidValue := string(workorder.StatusScheduled), "", ""
	if !reportedOn.IsZero() {
		status, reportedValue = string(workorder.StatusAwaitingPayment), reportedOn.Format(time.RFC3339)
	}
	if !paidOn.IsZero() {
		status, paidValue = string(workorder.StatusPaid), paidOn.Format(time.RFC3339)
	}
	return suite.createActivityWorkOrder(orderLabel, proposalLabel, acceptedOn, status, reportedValue, paidValue, imageCount)
}

func (suite *testSuite) createActivityWorkOrder(label, proposalLabel string, acceptedOn time.Time, status, reportedOnValue, paidOnValue string, imageCount int) error {
	fixture, ok := suite.operationInbox.proposals[proposalLabel]
	if !ok {
		return fmt.Errorf("unknown activity proposal %q", proposalLabel)
	}
	ctx := context.Background()
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(ctx, fixture.id)
	if err != nil {
		return fmt.Errorf("finding activity proposal %q: %w", proposalLabel, err)
	}
	acceptedUTC := acceptedOn.UTC()
	if err := proposal.Accept(proposal.Consumer.ID(), acceptedUTC); err != nil {
		return fmt.Errorf("accepting activity proposal %q: %w", proposalLabel, err)
	}
	order, err := workorder.New(proposal, acceptedUTC)
	if err != nil {
		return fmt.Errorf("creating activity work order %q: %w", label, err)
	}
	proposalRepository := repositories.NewServiceProposalRepository(suite.database)
	unitOfWork := repositories.NewPaymentUnitOfWork(suite.database, suite.paymentIntentRepository, suite.paymentTransactionRepository, proposalRepository, suite.workOrderRepository, suite.notificationRepository)
	if err := unitOfWork.Execute(ctx, func(store payment.TransactionalStore) error {
		if err := store.SaveServiceProposal(ctx, proposal); err != nil {
			return err
		}
		return store.SaveWorkOrder(ctx, order)
	}); err != nil {
		return fmt.Errorf("saving activity work order %q: %w", label, err)
	}
	persisted, err := suite.workOrderRepository.FindByServiceProposalID(ctx, fixture.id)
	if err != nil {
		return fmt.Errorf("finding saved activity work order %q: %w", label, err)
	}
	suite.operationInbox.orders[label] = persisted.ID()
	if status == string(workorder.StatusScheduled) {
		return nil
	}
	reportedOn, err := parseInboxInstant(reportedOnValue)
	if err != nil {
		return fmt.Errorf("activity work order %q needs its completion time: %w", label, err)
	}
	if err := suite.reportInboxWorkOrderCompletion(label, persisted, reportedOn, imageCount); err != nil {
		return err
	}
	if status != string(workorder.StatusPaid) {
		return nil
	}
	paidOn, err := parseInboxInstant(paidOnValue)
	if err != nil {
		return fmt.Errorf("activity work order %q needs its payment time: %w", label, err)
	}
	paidOrder, err := suite.workOrderRepository.FindByID(ctx, persisted.ID())
	if err != nil {
		return err
	}
	if err := paidOrder.RegisterApprovedBalancePayment(paidOn); err != nil {
		return fmt.Errorf("registering activity payment for %q: %w", label, err)
	}
	_, err = suite.workOrderRepository.Save(ctx, paidOrder)
	return err
}

func maxActivityInt64(minimum, value int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func (suite *testSuite) activityOneCompletionBeforeSeptember(email string) error {
	return suite.createActivityOrder("activity-prior-"+email, email, "juan@example.com", 10000, 1000,
		mustActivityDateTime("2026-08-25 10:00"), mustActivityDateTime("2026-08-28 10:00"), time.Time{})
}

func (suite *testSuite) activityOneCompletionBetweenSeptemberFirstAndThird(email string) error {
	return suite.createActivityOrder("activity-current-"+email, email, "juan@example.com", 10000, 1000,
		mustActivityDateTime("2026-09-01 10:00"), mustActivityDateTime("2026-09-02 10:00"), time.Time{})
}

func (suite *testSuite) activityFourCompletionsAcrossClients(firstEmail, secondEmail, repeatEmail string) error {
	for _, email := range []string{firstEmail, secondEmail} {
		if err := suite.activityOneCompletionBetweenSeptemberFirstAndThird(email); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		if err := suite.createActivityOrder(fmt.Sprintf("activity-repeat-%d-%s", i, repeatEmail), repeatEmail, "juan@example.com", 10000, 1000,
			mustActivityDateTime(fmt.Sprintf("2026-09-01 %02d:00", 11+i)), mustActivityDateTime(fmt.Sprintf("2026-09-02 %02d:00", 11+i)), time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) activityOtherProviderCompletion(providerEmail, consumerEmail string) error {
	return suite.createActivityOrder("activity-other-provider-"+consumerEmail, consumerEmail, providerEmail, 10000, 1000,
		mustActivityDateTime("2026-08-25 10:00"), mustActivityDateTime("2026-08-28 10:00"), time.Time{})
}

func (suite *testSuite) activityTwoPaidWorkOrders(email string) error {
	for i, imageCount := range []int{3, 2} {
		if err := suite.createActivityOrderWithImageCount(fmt.Sprintf("activity-paid-%d", i), email, "juan@example.com", 10001, 1000,
			mustActivityDateTime(fmt.Sprintf("2026-09-02 %02d:00", 10+i)), mustActivityDateTime(fmt.Sprintf("2026-09-04 %02d:00", 10+i)), mustActivityDateTime(fmt.Sprintf("2026-09-07 %02d:00", 10+i)), imageCount); err != nil {
			return err
		}
	}
	return nil
}

func mustActivityDateTime(value string) time.Time {
	parsed, err := activityDateTime(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func (suite *testSuite) activityFirstWorkHasNoisyHistory() error {
	auth0ID, permissions := suite.currentAuth0ID, suite.currentPermissions
	defer func() {
		suite.currentAuth0ID, suite.currentPermissions = auth0ID, permissions
	}()
	proposal := suite.activityState().proposalByOrder["activity-paid-0"]
	if proposal == "" {
		return fmt.Errorf("activity first paid work fixture does not exist")
	}
	if err := suite.inboxProposalHadRejectedThenApprovedDeposit(proposal); err != nil {
		return err
	}
	suite.lastServiceProposalID = suite.operationInbox.proposals[proposal].id
	suite.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	if err := suite.requestReview(5, "Trabajo completado correctamente."); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating activity review fixture returned %d: %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) activitySecondWorkHasNoisyHistory() error {
	proposal := suite.activityState().proposalByOrder["activity-paid-1"]
	if proposal == "" {
		return fmt.Errorf("activity second paid work fixture does not exist")
	}
	return suite.inboxProposalHadRejectedThenApprovedDeposit(proposal)
}

func (suite *testSuite) activityOneBookingAt(value string) error {
	bookingText, completionText, hasCompletion := strings.Cut(value, " e informé su finalización el")
	instant, err := activityQueryDate(bookingText)
	if err != nil {
		return err
	}
	if hasCompletion {
		reported, err := activityQueryDate(completionText)
		if err != nil {
			return err
		}
		return suite.createActivityOrder("activity-boundary-"+instant.Format("200601021504"), "ana@example.com", "juan@example.com", 10000, 1000, instant, reported, time.Time{})
	}
	suite.activityState().pendingBooking = instant
	return nil
}

func (suite *testSuite) activityOneCompletionAt(value string) error {
	reported, err := activityQueryDate(value)
	if err != nil {
		return err
	}
	accepted := suite.activityState().pendingBooking
	if accepted.IsZero() {
		return fmt.Errorf("completion fixture has no preceding booking")
	}
	suite.activityState().pendingBooking = time.Time{}
	return suite.createActivityOrder("activity-boundary-"+reported.Format("200601021504"), "ana@example.com", "juan@example.com", 10000, 1000, accepted, reported, time.Time{})
}

func (suite *testSuite) activityBookingCompletionAndPayment(booking, completion, paid string) error {
	accepted, err := activityQueryDate(booking)
	if err != nil {
		return err
	}
	reported, err := activityQueryDate(completion)
	if err != nil {
		return err
	}
	paidOn, err := activityQueryDate(paid)
	if err != nil {
		return err
	}
	return suite.createActivityOrder("activity-boundary-paid", "carla@example.com", "juan@example.com", 10000, 1000, accepted, reported, paidOn)
}

func (suite *testSuite) activityThirdPaymentAt(value string) error {
	paidOn, err := activityQueryDate(value)
	if err != nil {
		return err
	}
	return suite.createActivityOrder("activity-boundary-third-paid", "beatriz@example.com", "juan@example.com", 10000, 1000,
		mustActivityDateTime("2026-08-25 10:00"), mustActivityDateTime("2026-08-28 10:00"), paidOn)
}

func (suite *testSuite) activityOnePaidComparisonBaseline() error {
	return suite.createActivityOrder("activity-comparison-base", "ana@example.com", "juan@example.com", 10000, 1000,
		mustActivityDateTime("2026-09-08 10:00"), mustActivityDateTime("2026-09-09 10:00"), mustActivityDateTime("2026-09-09 11:00"))
}

func (suite *testSuite) activityTwoPaidComparisonCurrent() error {
	for i, email := range []string{"carla@example.com", "beatriz@example.com"} {
		if err := suite.createActivityOrder(fmt.Sprintf("activity-comparison-current-%d", i), email, "juan@example.com", 10000, 1000,
			mustActivityDateTime(fmt.Sprintf("2026-09-10 %02d:00", 10+i)), mustActivityDateTime(fmt.Sprintf("2026-09-11 %02d:00", 10+i)), mustActivityDateTime(fmt.Sprintf("2026-09-11 %02d:00", 11+i))); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) activityNoComparisonBaseline() error { return nil }

func (suite *testSuite) activityOnePaidNoBaseline() error {
	return suite.createActivityOrder("activity-comparison-only", "ana@example.com", "juan@example.com", 10001, 1000,
		mustActivityDateTime("2026-09-10 10:00"), mustActivityDateTime("2026-09-11 10:00"), mustActivityDateTime("2026-09-11 11:00"))
}

func (suite *testSuite) activityPendingRequestBeforePeriod(email string) error {
	return suite.activityCreatePendingRequest("pending-before-"+email, email, "juan@example.com", mustActivityDateTime("2026-09-10 10:00"))
}

func (suite *testSuite) activityPendingRequestOnSeptember23(email string) error {
	return suite.activityCreatePendingRequest("pending-after-"+email, email, "juan@example.com", mustActivityDateTime("2026-09-23 10:00"))
}

func (suite *testSuite) activityCreatePendingRequest(label, consumerEmail, providerEmail string, createdOn time.Time) error {
	auth0ID, permissions := suite.currentAuth0ID, suite.currentPermissions
	defer func() {
		suite.currentAuth0ID, suite.currentPermissions = auth0ID, permissions
	}()
	suite.activityState().nextFixture++
	label = fmt.Sprintf("activity-%s-%d", label, suite.activityState().nextFixture)
	return suite.createInboxJobRequest(label, consumerEmail, providerEmail, createdOn, "pending")
}

func (suite *testSuite) activityScheduledOrders(first, second string) error {
	for i, value := range []string{first, second} {
		scheduled, err := activityQueryDate(value)
		if err != nil {
			return err
		}
		accepted := scheduled.Add(-72 * time.Hour)
		suite.activityState().scheduledOverride = scheduled
		if err := suite.createActivityOrder(fmt.Sprintf("activity-scheduled-%d", i), "ana@example.com", "juan@example.com", 10000, 1000, accepted, time.Time{}, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) activityAwaitingPaymentOrder() error {
	return suite.createActivityOrder("activity-awaiting-payment", "ana@example.com", "juan@example.com", 10000, 1000,
		mustActivityDateTime("2026-09-14 10:00"), mustActivityDateTime("2026-09-17 10:00"), time.Time{})
}

func (suite *testSuite) activityOtherProviderPendingAndPayment(providerEmail string) error {
	if err := suite.activityCreatePendingRequest("other-provider-pending", "beatriz@example.com", providerEmail, mustActivityDateTime("2026-09-18 10:00")); err != nil {
		return err
	}
	return suite.createActivityOrder("activity-other-awaiting-payment", "ana@example.com", providerEmail, 10000, 1000,
		mustActivityDateTime("2026-09-14 10:00"), mustActivityDateTime("2026-09-17 10:00"), time.Time{})
}

func (suite *testSuite) activityNoRequestsOrOrders() error { return nil }

func (suite *testSuite) activityQueryDateRange(start, end string) error {
	return suite.requestActivityStatistics(start, end, "day", false)
}

func (suite *testSuite) activityQueryDailyDateRange(start, end string) error {
	return suite.requestActivityStatistics(start, end, "day", false)
}

func (suite *testSuite) activityQueryWeeklyDateRange(start, end string) error {
	return suite.requestActivityStatistics(start, end, "week", false)
}

func (suite *testSuite) activityQueryMonthlyDateRange(start, end string) error {
	return suite.requestActivityStatistics(start, end, "month", false)
}

func (suite *testSuite) activityQueryWithComparison(period string) error {
	return suite.requestActivityStatistics(period, "", "day", true)
}

func (suite *testSuite) activityQueryDefaultPeriod() error {
	return suite.requestActivityStatistics("", "", "day", false)
}

func (suite *testSuite) activityTryQueryInvalidPeriod(choice string) error {
	now := mustActivityDateTime("2026-09-29 12:00")
	from, to := mustActivityDateTime("2026-09-20 00:00"), mustActivityDateTime("2026-09-22 00:00")
	switch strings.TrimSpace(choice) {
	case "sin indicar el comienzo":
		return suite.requestActivityRaw(url.Values{"to": {to.Format(time.RFC3339)}})
	case "sin indicar el final":
		return suite.requestActivityRaw(url.Values{"from": {from.Format(time.RFC3339)}})
	case "con el mismo comienzo y final":
		return suite.requestActivityRaw(url.Values{"from": {from.Format(time.RFC3339)}, "to": {from.Format(time.RFC3339)}})
	case "con un final anterior al comienzo":
		return suite.requestActivityRaw(url.Values{"from": {to.Format(time.RFC3339)}, "to": {from.Format(time.RFC3339)}})
	case "por más de 365 días":
		return suite.requestActivityRaw(url.Values{"from": {mustActivityDateTime("2025-08-01 00:00").Format(time.RFC3339)}, "to": {mustActivityDateTime("2026-09-01 00:00").Format(time.RFC3339)}})
	case "con un final posterior al momento actual":
		return suite.requestActivityRaw(url.Values{"from": {from.Format(time.RFC3339)}, "to": {now.Add(time.Minute).Format(time.RFC3339)}})
	default:
		return fmt.Errorf("unsupported invalid activity period %q", choice)
	}
}

func (suite *testSuite) activityTryQuery() error {
	return suite.requestActivityStatistics("", "", "day", false)
}

func (suite *testSuite) activityNoSession() error {
	suite.currentAuth0ID = ""
	suite.invalidSession = false
	return nil
}

func (suite *testSuite) activityInvalidSession() error {
	suite.invalidSession = true
	return nil
}

func (suite *testSuite) activityUnknownIdentity() error {
	suite.currentAuth0ID = "auth0|unknown-activity-user"
	suite.invalidSession = false
	return nil
}

func (suite *testSuite) requestActivityStatistics(start, end, granularity string, compare bool) error {
	query := make(url.Values)
	if compare && end == "" {
		var from, to time.Time
		var err error
		from, to, err = activitySelectedPeriod(start)
		if err != nil {
			return err
		}
		query.Set("from", from.Format(time.RFC3339))
		query.Set("to", to.Format(time.RFC3339))
		start, end = "", ""
	}
	if strings.TrimSpace(start) != "" {
		from, err := activityQueryDate(start)
		if err != nil {
			return err
		}
		query.Set("from", from.Format(time.RFC3339))
	}
	if strings.TrimSpace(end) != "" {
		to, err := activityQueryDate(end)
		if err != nil {
			return err
		}
		query.Set("to", to.Format(time.RFC3339))
	}
	if granularity != "day" {
		query.Set("granularity", granularity)
	}
	if compare {
		query.Set("compare_previous", "true")
	}
	suite.activityState().lastQuery = query
	suite.activityState().responseValid = false
	return suite.requestActivityRaw(query)
}

func (suite *testSuite) requestActivityRaw(query url.Values) error {
	if err := suite.requestTestClockMock("2026-09-29T12:00:00-03:00"); err != nil {
		return fmt.Errorf("restoring activity statistics reference time: %w", err)
	}
	suite.activityState().lastQuery = query
	suite.activityState().responseValid = false
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+activityStatisticsPath+queryString(query), nil)
	if err != nil {
		return fmt.Errorf("creating activity statistics request: %w", err)
	}
	if suite.invalidSession {
		request.Header.Set("Authorization", "Bearer invalid.activity.token")
	} else if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting provider activity statistics: %w", err)
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading provider activity statistics: %w", err)
	}
	if response.StatusCode == http.StatusOK {
		var decoded activityStatisticsResponse
		if err := json.Unmarshal(suite.lastBody, &decoded); err != nil {
			return fmt.Errorf("decoding provider activity statistics: %w", err)
		}
		suite.activityState().response, suite.activityState().responseValid = decoded, true
	}
	return nil
}

func (suite *testSuite) activityResponse() (activityStatisticsResponse, error) {
	state := suite.activityState()
	if suite.lastStatus != http.StatusOK {
		return activityStatisticsResponse{}, fmt.Errorf("expected activity statistics status 200, got %d for query %v: %s", suite.lastStatus, state.lastQuery, suite.lastBody)
	}
	if !state.responseValid {
		return activityStatisticsResponse{}, fmt.Errorf("activity statistics response was not decoded")
	}
	return state.response, nil
}

func (suite *testSuite) activityResultsCounts(bookings, completions, payments, clients string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	expected := []int{mustActivityInt(bookings), mustActivityInt(completions), mustActivityInt(payments), mustActivityInt(clients)}
	found := []int{response.Results.ConfirmedBookings, response.Results.ReportedCompletions, response.Results.FullyPaidWorkOrders, response.Results.ClientsServed}
	if !equalActivityInts(found, expected) {
		return fmt.Errorf("activity totals are %v, want %v", found, expected)
	}
	return nil
}

func mustActivityInt(value string) int {
	if value == "un" || value == "una" {
		return 1
	}
	n, _ := strconv.Atoi(value)
	return n
}

func equalActivityInts(actual, expected []int) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func (suite *testSuite) activityOneNewAndReturningClient() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.NewClients != 1 || response.Results.ReturningClients != 1 {
		return fmt.Errorf("new/returning client counts are %d/%d, want 1/1", response.Results.NewClients, response.Results.ReturningClients)
	}
	return nil
}

func (suite *testSuite) activityValueAndAverage(totalValue, averageValue string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	expectedTotal, err := activityCurrencyToCents(totalValue)
	if err != nil {
		return err
	}
	expectedAverage, err := activityCurrencyToCents(averageValue)
	if err != nil {
		return err
	}
	if response.Results.AgreedValueCents != expectedTotal {
		return fmt.Errorf("agreed value is %d cents, want %d", response.Results.AgreedValueCents, expectedTotal)
	}
	if response.Results.AverageValueCents == nil || *response.Results.AverageValueCents != expectedAverage {
		return fmt.Errorf("average value is %v cents, want %d", response.Results.AverageValueCents, expectedAverage)
	}
	if response.Results.Currency != "ARS" {
		return fmt.Errorf("activity currency is %q, want ARS", response.Results.Currency)
	}
	return nil
}

func (suite *testSuite) activityPaymentIndependentFromCompletion(day string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ReportedCompletions != 2 || response.Results.FullyPaidWorkOrders != 2 {
		return fmt.Errorf("September has %d reported completions and %d paid orders, want 2/2", response.Results.ReportedCompletions, response.Results.FullyPaidWorkOrders)
	}
	instant, err := activityQueryDate(day + " de septiembre de 2026")
	if err != nil {
		return err
	}
	for _, bucket := range response.Evolution {
		if !bucket.From.After(instant) && bucket.To.After(instant) {
			if bucket.ReportedCompletions == 0 && bucket.FullyPaidWorkOrders == 1 {
				return nil
			}
			return fmt.Errorf("day %s bucket reports %d completions and %d paid orders, want 0/1", day, bucket.ReportedCompletions, bucket.FullyPaidWorkOrders)
		}
	}
	return fmt.Errorf("no evolution bucket contains September %s", day)
}

func (suite *testSuite) activityResultsExcludeProvider(email string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ConfirmedBookings != 2 || response.Results.ReportedCompletions != 2 || response.Results.FullyPaidWorkOrders != 2 || response.Results.ClientsServed != 2 || response.Results.AgreedValueCents != 30003 {
		return fmt.Errorf("results include or omit work from %q: got %+v", email, response.Results)
	}
	wantDay := mustActivityDateTime("2026-09-03 00:00")
	for _, bucket := range response.Evolution {
		if bucket.From.Equal(wantDay) {
			if bucket.ConfirmedBookings != 0 || bucket.ReportedCompletions != 0 || bucket.FullyPaidWorkOrders != 0 {
				return fmt.Errorf("results include %q activity on September 3: %+v", email, bucket)
			}
			return nil
		}
	}
	return fmt.Errorf("activity response has no September 3 bucket to verify exclusion of %q", email)
}

func (suite *testSuite) activityHasNoComparison() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison != nil {
		return fmt.Errorf("activity response unexpectedly includes comparison")
	}
	return nil
}

func (suite *testSuite) activityCompletionAndClientCounts(completions, clients string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ReportedCompletions != mustActivityInt(completions) || response.Results.ClientsServed != mustActivityInt(clients) {
		return fmt.Errorf("activity completion/client totals are %d/%d, want %s/%s", response.Results.ReportedCompletions, response.Results.ClientsServed, completions, clients)
	}
	return nil
}

func (suite *testSuite) activityReturningAndNewClients(returning, newClients string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ReturningClients != mustActivityInt(returning) || response.Results.NewClients != mustActivityInt(newClients) {
		return fmt.Errorf("activity returning/new client totals are %d/%d, want %s/%s", response.Results.ReturningClients, response.Results.NewClients, returning, newClients)
	}
	return nil
}

func (suite *testSuite) activityEvolutionMatches(table *godog.Table) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != len(response.Evolution) {
		return fmt.Errorf("activity evolution has %d days, want %d", len(response.Evolution), len(rows))
	}
	for index, row := range rows {
		bucket := response.Evolution[index]
		wantDay, err := activityQueryDate(row["día"])
		if err != nil {
			return err
		}
		if !bucket.From.Equal(wantDay) {
			return fmt.Errorf("evolution bucket %d starts at %s, want day %s", index+1, bucket.From, row["día"])
		}
		if bucket.ConfirmedBookings != mustActivityInt(row["contrataciones"]) || bucket.ReportedCompletions != mustActivityInt(row["finalizaciones informadas"]) || bucket.FullyPaidWorkOrders != mustActivityInt(row["cobros completos"]) {
			return fmt.Errorf("evolution bucket %d is %+v, want %v", index+1, bucket, row)
		}
	}
	return nil
}

func (suite *testSuite) activityNoPaymentsInPeriod() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.FullyPaidWorkOrders != 0 {
		return fmt.Errorf("activity period has %d paid orders, want zero", response.Results.FullyPaidWorkOrders)
	}
	return nil
}

func (suite *testSuite) activityBookingInBucket(start, end string) error {
	return suite.activityBucketHas(start, end, "booking")
}

func (suite *testSuite) activityCompletionInBucket(start, end string) error {
	return suite.activityBucketHas(start, end, "completion")
}

func (suite *testSuite) activityBucketHas(start, end, metric string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	wantFrom, err := activityQueryDate(start)
	if err != nil {
		return err
	}
	wantTo, err := activityQueryDate(end)
	if err != nil {
		return err
	}
	for _, bucket := range response.Evolution {
		if bucket.From.Equal(wantFrom) && bucket.To.Equal(wantTo) {
			count := bucket.ConfirmedBookings
			if metric == "completion" {
				count = bucket.ReportedCompletions
			}
			if count != 1 {
				return fmt.Errorf("activity bucket %s–%s has %d %s, want one", start, end, count, metric)
			}
			return nil
		}
	}
	return fmt.Errorf("activity evolution has no bucket from %s to %s", start, end)
}

func (suite *testSuite) activityNoOtherBuckets() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if len(response.Evolution) != 2 {
		return fmt.Errorf("activity evolution has %d buckets, want exactly 2", len(response.Evolution))
	}
	return nil
}

func (suite *testSuite) activityComparisonPeriodIs(period string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("activity response is missing its requested comparison")
	}
	wantFrom, wantTo, err := activitySelectedPeriod(period)
	if err != nil {
		return err
	}
	if !response.Comparison.Period.From.Equal(wantFrom) || !response.Comparison.Period.To.Equal(wantTo) {
		return fmt.Errorf("comparison period is %s–%s, want %s–%s", response.Comparison.Period.From, response.Comparison.Period.To, wantFrom, wantTo)
	}
	return nil
}

func (suite *testSuite) activityComparisonMatches(table *godog.Table) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("activity response is missing its requested comparison")
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	metricNames := map[string]string{"contrataciones": "confirmed_bookings", "finalizaciones informadas": "reported_completions", "cobros completos": "fully_paid_work_orders", "clientes atendidos": "clients_served", "valor pactado": "agreed_value_cents", "promedio": "average_value_cents"}
	for _, row := range rows {
		metric, exists := metricNames[row["resultado"]]
		if !exists {
			return fmt.Errorf("unsupported comparison result %q", row["resultado"])
		}
		for label, wantText := range map[string]string{"actual": row["actual"], "anterior": row["anterior"]} {
			want, err := activityComparisonAmount(wantText)
			if err != nil {
				return err
			}
			var actual *float64
			if label == "actual" {
				actual = activityMetricValue(response.Results, metric)
			} else {
				actual = activityMetricValue(response.Comparison.Results, metric)
			}
			if (actual == nil) != (want == nil) || actual != nil && *actual != *want {
				return fmt.Errorf("comparison %s %s value is %s, want %s (request query %v)", metric, label, activityFloatString(actual), activityFloatString(want), suite.activityState().lastQuery)
			}
		}
		change, exists := response.Comparison.Changes[metric]
		if !exists {
			return fmt.Errorf("comparison response is missing metric %q", metric)
		}
		wantAbsolute, err := activityComparisonAmount(row["diferencia"])
		if err != nil {
			return err
		}
		if (change.Absolute == nil) != (wantAbsolute == nil) || change.Absolute != nil && *change.Absolute != *wantAbsolute {
			return fmt.Errorf("comparison %s absolute change is %s, want %s (request query %v)", metric, activityFloatString(change.Absolute), activityFloatString(wantAbsolute), suite.activityState().lastQuery)
		}
		wantPercentage, err := activityPercentage(row["cambio porcentual"])
		if err != nil {
			return err
		}
		if (change.Percentage == nil) != (wantPercentage == nil) || change.Percentage != nil && *change.Percentage != *wantPercentage {
			return fmt.Errorf("comparison %s percentage is %s, want %s (request query %v)", metric, activityFloatString(change.Percentage), activityFloatString(wantPercentage), suite.activityState().lastQuery)
		}
	}
	return nil
}

func activityFloatString(value *float64) string {
	if value == nil {
		return "null"
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func activityMetricValue(totals activityStatisticsTotals, metric string) *float64 {
	var value float64
	switch metric {
	case "confirmed_bookings":
		value = float64(totals.ConfirmedBookings)
	case "reported_completions":
		value = float64(totals.ReportedCompletions)
	case "fully_paid_work_orders":
		value = float64(totals.FullyPaidWorkOrders)
	case "clients_served":
		value = float64(totals.ClientsServed)
	case "agreed_value_cents":
		value = float64(totals.AgreedValueCents)
	case "average_value_cents":
		if totals.AverageValueCents == nil {
			return nil
		}
		value = float64(*totals.AverageValueCents)
	}
	return &value
}

func activityComparisonAmount(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "sin valor" {
		return nil, nil
	}
	if strings.HasPrefix(value, "ARS ") {
		cents, err := activityCurrencyToCents(value)
		if err != nil {
			return nil, err
		}
		v := float64(cents)
		return &v, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing comparison amount %q: %w", value, err)
	}
	return &parsed, nil
}

func activityPercentage(value string) (*float64, error) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" || value == "sin valor" {
		return nil, nil
	}
	value = strings.ReplaceAll(value, ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing comparison percentage %q: %w", value, err)
	}
	return &parsed, nil
}

func (suite *testSuite) activityComparisonOneMore() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("activity response is missing its requested comparison")
	}
	for _, metric := range []string{"confirmed_bookings", "reported_completions", "fully_paid_work_orders", "clients_served"} {
		change, exists := response.Comparison.Changes[metric]
		if !exists || change.Absolute == nil || *change.Absolute != 1 {
			return fmt.Errorf("comparison change for %s is absolute=%s percentage=%s, want +1 (request query %v)", metric, activityFloatString(change.Absolute), activityFloatString(change.Percentage), suite.activityState().lastQuery)
		}
	}
	return nil
}

func (suite *testSuite) activityComparisonValueIncrease(value string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("activity response is missing its requested comparison")
	}
	wantCents, err := activityCurrencyToCents(value)
	if err != nil {
		return err
	}
	change := response.Comparison.Changes["agreed_value_cents"]
	if change.Absolute == nil || *change.Absolute != float64(wantCents) {
		return fmt.Errorf("agreed value comparison is absolute=%s percentage=%s, want %d cents (request query %v)", activityFloatString(change.Absolute), activityFloatString(change.Percentage), wantCents, suite.activityState().lastQuery)
	}
	return nil
}

func (suite *testSuite) activityZeroBaselinePercentages() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("activity response is missing its requested comparison")
	}
	for metric, change := range response.Comparison.Changes {
		if change.Percentage != nil {
			return fmt.Errorf("comparison percentage for %s is %v, want null for a zero baseline", metric, *change.Percentage)
		}
	}
	return nil
}

func (suite *testSuite) activityAverageUndefined() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.AverageValueCents == nil || *response.Results.AverageValueCents != 10001 {
		return fmt.Errorf("current average is %v cents, want 10001", response.Results.AverageValueCents)
	}
	if response.Comparison == nil || response.Comparison.Results.AverageValueCents != nil {
		return fmt.Errorf("previous average is %v, want null", response.Comparison.Results.AverageValueCents)
	}
	change := response.Comparison.Changes["average_value_cents"]
	if change.Absolute != nil || change.Percentage != nil {
		return fmt.Errorf("undefined previous average should have null absolute and percentage changes, got absolute=%s percentage=%s", activityFloatString(change.Absolute), activityFloatString(change.Percentage))
	}
	return nil
}

func (suite *testSuite) activityPeriodIsEmpty() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ConfirmedBookings != 0 || response.Results.ReportedCompletions != 0 || response.Results.FullyPaidWorkOrders != 0 || response.Results.AgreedValueCents != 0 || response.Results.AverageValueCents != nil {
		return fmt.Errorf("activity results are not empty: %+v", response.Results)
	}
	return nil
}

func (suite *testSuite) activityTwoEmptyDailyBuckets() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if len(response.Evolution) != 2 {
		return fmt.Errorf("activity evolution has %d buckets, want 2", len(response.Evolution))
	}
	for _, bucket := range response.Evolution {
		if bucket.ConfirmedBookings != 0 || bucket.ReportedCompletions != 0 || bucket.FullyPaidWorkOrders != 0 {
			return fmt.Errorf("expected an empty daily bucket, got %+v", bucket)
		}
	}
	return nil
}

func (suite *testSuite) activityPendingCounts(requests, scheduled, awaiting string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	want := activityStatisticsPending{Requests: mustActivityInt(requests), ScheduledOrders: mustActivityInt(scheduled), AwaitingPaymentOrders: mustActivityInt(awaiting)}
	if response.Pending != want {
		return fmt.Errorf("current pending totals are %+v, want %+v", response.Pending, want)
	}
	return nil
}

func (suite *testSuite) activityOtherProviderPendingExcluded(email string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	want := activityStatisticsPending{Requests: 2, ScheduledOrders: 2, AwaitingPaymentOrders: 1}
	if response.Pending != want {
		return fmt.Errorf("pending results include or omit work from %q: got %+v, want %+v", email, response.Pending, want)
	}
	return nil
}

func (suite *testSuite) activityDefaultPeriodMatches(start, end string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	wantFrom, err := activityQueryDate(start)
	if err != nil {
		return err
	}
	wantTo, err := activityQueryDate(end)
	if err != nil {
		return err
	}
	if !response.Period.From.Equal(wantFrom) || !response.Period.To.Equal(wantTo) {
		return fmt.Errorf("default period is %s–%s, want %s–%s", response.Period.From, response.Period.To, wantFrom, wantTo)
	}
	return nil
}

func (suite *testSuite) activityEmptyResultsAndPending() error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if response.Results.ConfirmedBookings != 0 || response.Results.ReportedCompletions != 0 || response.Results.FullyPaidWorkOrders != 0 || response.Results.ClientsServed != 0 || response.Results.NewClients != 0 || response.Results.ReturningClients != 0 || response.Results.AgreedValueCents != 0 || response.Results.AverageValueCents != nil || response.Pending != (activityStatisticsPending{}) {
		return fmt.Errorf("empty activity response contains totals or pending work: results=%+v pending=%+v", response.Results, response.Pending)
	}
	return nil
}

func (suite *testSuite) activityEmptyDayCount(dayCount string) error {
	response, err := suite.activityResponse()
	if err != nil {
		return err
	}
	if len(response.Evolution) != mustActivityInt(dayCount) {
		return fmt.Errorf("default evolution has %d buckets, want %s", len(response.Evolution), dayCount)
	}
	for _, bucket := range response.Evolution {
		if bucket.ConfirmedBookings != 0 || bucket.ReportedCompletions != 0 || bucket.FullyPaidWorkOrders != 0 {
			return fmt.Errorf("default evolution contains activity: %+v", bucket)
		}
	}
	return nil
}

func (suite *testSuite) activityPeriodRejected() error {
	if suite.lastStatus != http.StatusBadRequest {
		return fmt.Errorf("invalid activity period returned %d, want 400: %s", suite.lastStatus, suite.lastBody)
	}
	if bytes.Contains(suite.lastBody, []byte(`"results"`)) {
		return fmt.Errorf("invalid activity period response includes results: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) activityIdentityRejected(message string) error {
	wantStatus := http.StatusUnauthorized
	if strings.Contains(message, "no soy prestador") {
		wantStatus = http.StatusForbidden
	}
	if strings.Contains(message, "no tengo una cuenta") {
		wantStatus = http.StatusNotFound
	}
	if suite.invalidSession && suite.currentAuth0ID == "" {
		wantStatus = http.StatusUnauthorized
	}
	if suite.lastStatus != wantStatus {
		return fmt.Errorf("activity identity request returned %d, want %d: %s", suite.lastStatus, wantStatus, suite.lastBody)
	}
	if bytes.Contains(suite.lastBody, []byte(`"results"`)) {
		return fmt.Errorf("rejected activity response includes results: %s", suite.lastBody)
	}
	return nil
}

func activitySelectedPeriod(value string) (time.Time, time.Time, error) {
	clean := strings.TrimSpace(value)
	datePart := clean
	datePart = regexp.MustCompile(`\b20\d{2}\b`).ReplaceAllString(datePart, "")
	datePart = regexp.MustCompile(`(?:a las|las)\s+\d{1,2}:\d{2}(?::\d{2})?`).ReplaceAllString(datePart, "")
	days := regexp.MustCompile(`\b(\d{1,2})\b`).FindAllStringSubmatch(datePart, -1)
	if len(days) < 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("expected a two-day activity period in %q", value)
	}
	month := time.September
	for name, candidate := range activityMonthNumbers {
		if strings.Contains(strings.ToLower(clean), name) {
			month = candidate
			break
		}
	}
	firstDay, _ := strconv.Atoi(days[0][1])
	lastDay, _ := strconv.Atoi(days[1][1])
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from := time.Date(2026, month, firstDay, 0, 0, 0, 0, location)
	to := time.Date(2026, month, lastDay+1, 0, 0, 0, 0, location)
	return from, to, nil
}

func activityQueryDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := activityDateTime(value); err == nil {
		return parsed, nil
	}
	return parseNaturalActivityDate(value, time.Time{})
}

func queryString(query url.Values) string {
	encoded := query.Encode()
	if encoded == "" {
		return ""
	}
	return "?" + encoded
}

var activityMonthNumbers = map[string]time.Month{
	"enero": time.January, "febrero": time.February, "marzo": time.March,
	"abril": time.April, "mayo": time.May, "junio": time.June,
	"julio": time.July, "agosto": time.August, "septiembre": time.September,
	"octubre": time.October, "noviembre": time.November, "diciembre": time.December,
}

func parseNaturalActivityDate(value string, fallback time.Time) (time.Time, error) {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.Time{}, fmt.Errorf("loading Buenos Aires time zone: %w", err)
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	dateWithoutTime := regexp.MustCompile(`\b(?:a las|las)\s+\d{1,2}:\d{2}(?::\d{2})?\b`).ReplaceAllString(lower, "")
	dateWithoutYear := regexp.MustCompile(`\b20\d{2}\b`).ReplaceAllString(dateWithoutTime, "")
	dayMatches := regexp.MustCompile(`\b(\d{1,2})\b`).FindAllStringSubmatch(dateWithoutYear, -1)
	if len(dayMatches) == 0 {
		return time.Time{}, fmt.Errorf("unsupported activity query period %q", value)
	}
	day, err := strconv.Atoi(dayMatches[0][1])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid day in activity query period %q", value)
	}
	month, year := fallback.Month(), fallback.Year()
	if fallback.IsZero() {
		month, year = time.September, 2026
	}
	for name, candidate := range activityMonthNumbers {
		if strings.Contains(lower, name) {
			month = candidate
			break
		}
	}
	if match := regexp.MustCompile(`\b(20\d{2})\b`).FindStringSubmatch(lower); len(match) == 2 {
		year, err = strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing year in activity query period %q: %w", value, err)
		}
	}
	hour, minute, second := 0, 0, 0
	if match := regexp.MustCompile(`(?:a las|las)\s+(\d{1,2}):(\d{2})(?::(\d{2}))?`).FindStringSubmatch(lower); len(match) == 4 {
		hour, _ = strconv.Atoi(match[1])
		minute, _ = strconv.Atoi(match[2])
		if match[3] != "" {
			second, _ = strconv.Atoi(match[3])
		}
	}
	if hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, fmt.Errorf("invalid clock time in activity query period %q", value)
	}
	instant := time.Date(year, month, day, hour, minute, second, 0, location)
	if instant.Day() != day || instant.Month() != month {
		return time.Time{}, fmt.Errorf("invalid date in activity query period %q", value)
	}
	return instant, nil
}

func TestParseNaturalActivityDatePreservesSeconds(t *testing.T) {
	parsed, err := parseNaturalActivityDate("3 de septiembre de 2026 a las 23:59:59", time.Time{})
	if err != nil {
		t.Fatalf("parse date with seconds: %v", err)
	}
	if parsed.Hour() != 23 || parsed.Minute() != 59 || parsed.Second() != 59 {
		t.Fatalf("parsed time is %s, want 23:59:59", parsed.Format("15:04:05"))
	}
}
