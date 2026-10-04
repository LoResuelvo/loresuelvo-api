package steps_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

const conversionStatisticsPath = "/providers/me/statistics/conversion"

type conversionStages struct {
	Issued     int64 `json:"issued"`
	Contracted int64 `json:"contracted"`
	Reported   int64 `json:"reported"`
	Paid       int64 `json:"paid"`
}
type conversionRatio struct {
	Numerator   int64    `json:"numerator"`
	Denominator int64    `json:"denominator"`
	Percentage  *float64 `json:"percentage"`
}
type conversionStageRates struct {
	Cohort        conversionRatio `json:"cohort"`
	PreviousStage conversionRatio `json:"previous_stage"`
}
type conversionStatisticsResponse struct {
	Period struct {
		From     time.Time `json:"from"`
		To       time.Time `json:"to"`
		TimeZone string    `json:"time_zone"`
	} `json:"period"`
	ObservedAt time.Time `json:"observed_at"`
	Proposals  struct {
		Stages conversionStages `json:"stages"`
		Rates  struct {
			Contracted conversionStageRates `json:"contracted"`
			Reported   conversionStageRates `json:"reported"`
			Paid       conversionStageRates `json:"paid"`
		} `json:"rates"`
		Uncontracted int64 `json:"uncontracted"`
	} `json:"proposals"`
	Requests struct {
		Received       int64           `json:"received"`
		Accepted       int64           `json:"accepted"`
		Pending        int64           `json:"pending"`
		AcceptanceRate conversionRatio `json:"acceptance_rate"`
	} `json:"requests"`
}
type conversionProposalFixture struct {
	label, providerEmail, requestLabel string
	created, accepted, reported, paid  time.Time
	status                             serviceproposal.Status
}
type conversionStatisticsState struct {
	proposals                                          map[string]conversionProposalFixture
	requestByPair                                      map[string]string
	nextFixture                                        int
	query                                              url.Values
	response                                           conversionStatisticsResponse
	responseValid                                      bool
	authorization, expectedAuthorization, cacheControl string
	preserveRecords                                    bool
	before                                             map[string]any
}

func (s *testSuite) conversionState() *conversionStatisticsState {
	s.operationInbox.ensureMaps()
	if s.conversionStatistics == nil {
		s.conversionStatistics = &conversionStatisticsState{
			proposals: make(map[string]conversionProposalFixture), requestByPair: make(map[string]string),
		}
	}
	return s.conversionStatistics
}

func registerGetConversionStatisticsSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que tengo estas propuestas:$`, s.conversionProposalsTable)
	sc.Step(`^que tengo estas propuestas creadas antes del 29 de septiembre de 2026:$`, s.conversionProposalsTable)
	sc.Step(`^que tengo tres propuestas creadas entre el 1 y el 3 de septiembre:$`, s.conversionProposalsTable)
	sc.Step(`^que tengo cuatro propuestas creadas en agosto, contratadas entre el 2 y el 4 de septiembre$`, s.conversionPriorProposals)
	sc.Step(`^que "pedro@example.com" tiene una propuesta creada el 2 de septiembre, contratada el 3, finalizada el 4 y pagada el 5 de septiembre$`, s.conversionOtherProviderProposal)
	sc.Step(`^que tengo una propuesta creada (.+)$`, s.conversionBoundaryProposal)
	sc.Step(`^que tengo dos propuestas distintas para "ana@example.com" en una misma conversación, creadas el 2 y el 3 de septiembre$`, s.conversionSharedConversationProposals)
	sc.Step(`^que ambas propuestas fueron contratadas, tienen finalización informada y pago completo antes del 28 de septiembre$`, s.conversionBothProposalsAdvance)
	sc.Step(`^que el reporte de finalización de la orden vinculada a una propuesta tiene tres fotos$`, s.conversionThreeImages)
	sc.Step(`^que la orden de la otra propuesta tiene varios intentos de pago, uno rechazado y otro aprobado$`, s.conversionMultiplePaymentAttempts)
	sc.Step(`^que tengo 32 propuestas creadas entre el 1 y el 28 de septiembre, una creada el 1 fue contratada el 10 y las otras 31 no tienen orden$`, s.conversionHalfCentFixtures)
	sc.Step(`^que tengo 3 propuestas creadas entre el 1 y el 3 de septiembre y ninguna fue contratada$`, s.conversionUncontractedFixtures)
	sc.Step(`^que tengo estas solicitudes:$`, s.conversionRequestsTable)
	sc.Step(`^que la solicitud que recibí de "ana@example.com" fue aceptada después del final del período$`, s.conversionRequestAcceptedAfterPeriod)
	sc.Step(`^que acepté la solicitud de "carla@example.com" el 3 de septiembre antes de crear cuatro propuestas en nuestra conversación el 4 de septiembre; una fue contratada el 10 y las otras tres no tienen orden$`, s.conversionCarlaProposals)
	sc.Step(`^que no tengo propuestas creadas entre el 1 y el 4 de septiembre$`, s.conversionNoProposals)
	sc.Step(`^que recibí una solicitud pendiente el 3 de septiembre$`, s.conversionPendingRequest)
	sc.Step(`^que no tengo propuestas ni solicitudes recibidas entre el 30 de agosto y el 29 de septiembre de 2026$`, s.conversionNoRecords)
	sc.Step(`^que tengo propuestas creadas antes del 30 de agosto a las 12:00, exactamente a esa hora, antes del 29 de septiembre a las 12:00 y exactamente al final del período$`, s.conversionDefaultBoundaryFixtures)
	sc.Step(`^que tengo una propuesta pendiente y una solicitud aceptada, y en otra conversación una propuesta aceptada con su orden pagada y un mensaje$`, s.conversionPrivacyFixtures)
	sc.Step(`^que todos esos registros se crearon entre el 1 y el 28 de septiembre de 2026, y no tengo una cuenta de cobros conectada$`, s.conversionPrivacyFixtureEvidence)
	sc.Step(`^consulto la conversión desde el (.+) hasta antes del (.+)$`, s.conversionQueryPeriod)
	sc.Step(`^consulto la conversión sin elegir un período$`, s.conversionQueryDefault)
	sc.Step(`^intento consultar la conversión$`, s.conversionQueryDefault)
	sc.Step(`^intento consultar la conversión (.+)$`, s.conversionQueryInvalid)
	sc.Step(`^veo estas etapas de mi cohorte:$`, s.conversionStagesTable)
	sc.Step(`^P1 cuenta en las cuatro etapas aunque su trabajo ya esté pagado$`, s.conversionPaidProposalInAllStages)
	sc.Step(`^la contratación posterior de P1, aunque ocurrió después del período, cuenta en su cohorte$`, s.conversionLaterMilestoneCounts)
	sc.Step(`^las cuatro propuestas anteriores no cuentan aunque sus contrataciones ocurrieron dentro del período$`, s.conversionPriorProposalsExcluded)
	sc.Step(`^la conversión de mi cohorte es 50,00 %, no la tasa que resultaría de dividir los hitos del período por las propuestas emitidas$`, s.conversionCohortNotActivity)
	sc.Step(`^los resultados corresponden exclusivamente a mis propuestas, no a las de "pedro@example.com"$`, s.conversionOtherProviderExcluded)
	sc.Step(`^no veo comparaciones con otra cohorte, variaciones entre períodos ni espacios reservados para esos resultados$`, s.conversionNoComparison)
	sc.Step(`^veo (\d+) propuestas? emitidas?$`, s.conversionIssuedCount)
	sc.Step(`^veo (\d+) propuestas emitidas, (\d+) contratadas, (\d+) con finalización informada y (\d+) con pago completo$`, s.conversionStageCounts)
	sc.Step(`^veo estas tasas, calculadas con los conteos indicados:$`, s.conversionRatesTable)
	sc.Step(`^la tasa de contratación sobre la cohorte es 3,13 %, calculada como 1 propuesta contratada sobre 32 emitidas$`, s.conversionHalfCentPercentage)
	sc.Step(`^veo 0 propuestas contratadas, con finalización informada y con pago completo$`, s.conversionNoAdvances)
	sc.Step(`^las tres tasas sobre la cohorte son 0,00 %$`, s.conversionZeroCohortRates)
	sc.Step(`^la contratación respecto de las emitidas es 0,00 %$`, s.conversionZeroContractingRate)
	sc.Step(`^la finalización y el pago respecto de su etapa anterior se muestran sin valor porque esos denominadores son cero$`, s.conversionNullAdvanceRates)
	sc.Step(`^veo 2 propuestas sin contratación observada, calculadas como 3 emitidas menos 1 contratada$`, s.conversionUncontractedCount)
	sc.Step(`^las propuestas conservan sus estados aceptada, pendiente y rechazada$`, s.conversionProposalStatesPreserved)
	sc.Step(`^P2 no se presenta como rechazada por no tener una contratación observada$`, s.conversionPendingNotRejected)
	sc.Step(`^veo (\d+) solicitudes recibidas, (\d+) aceptadas y (\d+) pendiente$`, s.conversionRequestCounts)
	sc.Step(`^la aceptación de solicitudes es 66,67 %, calculada como 2 aceptadas sobre 3 recibidas$`, s.conversionAcceptancePercentage)
	sc.Step(`^la solicitud aceptada después del período cuenta según su estado actual$`, s.conversionCurrentRequestStatusCounts)
	sc.Step(`^mi conversión de propuestas es 25,00 %, calculada como 1 contratada sobre 4 emitidas, independiente del 66,67 % de solicitudes aceptadas$`, s.conversionIndependentUniverses)
	sc.Step(`^no se cuentan las solicitudes recibidas antes del inicio, en el final del período ni para "pedro@example.com"$`, s.conversionRequestsOutsidePeriodExcluded)
	sc.Step(`^veo cero propuestas en todas las etapas y porcentajes sin valor por denominador cero$`, s.conversionEmptyProposals)
	sc.Step(`^veo 1 solicitud recibida, 0 aceptadas, 1 pendiente y 0,00 % de aceptación$`, s.conversionPendingRequestOnly)
	sc.Step(`^veo cero propuestas y solicitudes en todos los conteos$`, s.conversionEmptyCounts)
	sc.Step(`^todos los porcentajes se muestran sin valor porque sus denominadores son cero$`, s.conversionAllRatesNull)
	sc.Step(`^veo 2 propuestas emitidas en el período desde el 30 de agosto de 2026 a las 12:00 hasta el 29 de septiembre de 2026 a las 12:00 en Buenos Aires$`, s.conversionDefaultPeriod)
	sc.Step(`^veo que los avances se observaron el 29 de septiembre de 2026 a las 12:00$`, s.conversionObservedAt)
	sc.Step(`^veo únicamente indicadores agregados propios, sin conversaciones, datos de clientes ni información financiera$`, s.conversionAggregatePrivacy)
	sc.Step(`^los resultados son privados y no se almacenan en caché$`, s.conversionPrivateCache)
	sc.Step(`^las propuestas, las solicitudes, la orden y el mensaje conservan su información$`, s.conversionRecordsPreserved)
}

func conversionDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return activityQueryDate(value)
}
func (s *testSuite) conversionRequest(label, consumer, provider string, created time.Time, status string) error {
	state := s.conversionState()
	pair := consumer + "\x00" + provider
	if _, exists := state.requestByPair[pair]; exists {
		return fmt.Errorf("conversion request pair already exists: %s", pair)
	}
	if err := s.createInboxJobRequest(label, consumer, provider, created, status); err != nil {
		return err
	}
	state.requestByPair[pair] = label
	return nil
}
func (s *testSuite) conversionEnsureRequest(consumer, provider string) (string, error) {
	state := s.conversionState()
	pair := consumer + "\x00" + provider
	if label, exists := state.requestByPair[pair]; exists {
		return label, nil
	}
	state.nextFixture++
	label := fmt.Sprintf("conversion-request-%d", state.nextFixture)
	if err := s.conversionRequest(label, consumer, provider, mustActivityDateTime("2026-08-01 00:00"), "accepted"); err != nil {
		return "", err
	}
	return label, nil
}
func (s *testSuite) conversionCreateProposal(label, consumer, provider string, created, accepted, reported, paid time.Time, status serviceproposal.Status, images int) error {
	requestLabel, err := s.conversionEnsureRequest(consumer, provider)
	if err != nil {
		return err
	}
	scheduled := created.Add(72 * time.Hour)
	if !accepted.IsZero() && scheduled.Before(accepted.Add(48*time.Hour)) {
		scheduled = accepted.Add(48 * time.Hour)
	}
	if !reported.IsZero() {
		scheduled = reported.Add(-time.Hour)
	}
	if scheduled.Sub(created) < 24*time.Hour || (!accepted.IsZero() && !accepted.Before(scheduled)) {
		return fmt.Errorf("invalid independent conversion proposal/milestone fixture %s", label)
	}
	row := map[string]string{
		"propuesta": label, "solicitud": requestLabel, "creada": created.Format(time.RFC3339Nano),
		"fecha programada": scheduled.Format(time.RFC3339Nano), "duración": "60", "precio total": "10000",
		"moneda": "ARS", "descripción": "Conversion private proposal " + label, "estado": "pending",
	}
	if err := s.createDetailProposal(row); err != nil {
		return err
	}
	s.conversionState().proposals[label] = conversionProposalFixture{label: label, providerEmail: provider, requestLabel: requestLabel, created: created, accepted: accepted, reported: reported, paid: paid, status: status}
	if !accepted.IsZero() {
		return s.conversionAdvanceProposal(label, accepted, reported, paid, images)
	}
	if status == serviceproposal.StatusRejected {
		repository := repositories.NewServiceProposalRepository(s.database)
		proposal, err := repository.FindByID(s.scenarioContext, s.operationInbox.proposals[label].id)
		if err != nil {
			return err
		}
		proposal.Status = serviceproposal.StatusRejected
		unit := repositories.NewPaymentUnitOfWork(s.database, s.paymentIntentRepository, s.paymentTransactionRepository, repository, s.workOrderRepository, s.notificationRepository)
		return unit.Execute(s.scenarioContext, func(store payment.TransactionalStore) error {
			return store.SaveServiceProposal(s.scenarioContext, proposal)
		})
	}
	if status != serviceproposal.StatusPending {
		return fmt.Errorf("proposal %s has unsupported fixture status %s without an order", label, status)
	}
	return nil
}
func (s *testSuite) conversionAdvanceProposal(label string, accepted, reported, paid time.Time, images int) error {
	fixture, exists := s.conversionState().proposals[label]
	if !exists {
		return fmt.Errorf("unknown conversion proposal %s", label)
	}
	status, reportValue, paidValue := string(workorder.StatusScheduled), "", ""
	if !reported.IsZero() {
		status, reportValue = string(workorder.StatusAwaitingPayment), reported.Format(time.RFC3339Nano)
	}
	if !paid.IsZero() {
		status, paidValue = string(workorder.StatusPaid), paid.Format(time.RFC3339Nano)
	}
	if err := s.createActivityWorkOrder("conversion-order-"+label, label, accepted, status, reportValue, paidValue, images); err != nil {
		return err
	}
	fixture.accepted, fixture.reported, fixture.paid, fixture.status = accepted, reported, paid, serviceproposal.StatusAccepted
	s.conversionState().proposals[label] = fixture
	return nil
}
func (s *testSuite) conversionProposalsTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	return s.withInboxFixtureClock(func() error {
		for _, row := range rows {
			created, err := conversionDate(row["creada el"])
			if err != nil {
				return err
			}
			accepted, err := conversionDate(row["contratación confirmada"])
			if err != nil {
				return err
			}
			reported, err := conversionDate(row["finalización informada"])
			if err != nil {
				return err
			}
			paid, err := conversionDate(row["pago completo"])
			if err != nil {
				return err
			}
			consumer := row["cliente"]
			if consumer == "" {
				consumer = "ana@example.com"
			}
			status := serviceproposal.StatusPending
			if !accepted.IsZero() {
				status = serviceproposal.StatusAccepted
			}
			if row["estado actual"] == "rechazada" {
				status = serviceproposal.StatusRejected
			}
			if err := s.conversionCreateProposal(row["propuesta"], consumer, "juan@example.com", created, accepted, reported, paid, status, 1); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionPriorProposals() error {
	return s.withInboxFixtureClock(func() error {
		for index := 0; index < 4; index++ {
			if err := s.conversionCreateProposal(fmt.Sprintf("prior-%d", index), "ana@example.com", "juan@example.com", mustActivityDateTime("2026-08-20 00:00").Add(time.Duration(index)*24*time.Hour), mustActivityDateTime("2026-09-02 00:00").Add(time.Duration(index%3)*24*time.Hour), time.Time{}, time.Time{}, serviceproposal.StatusAccepted, 1); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionOtherProviderProposal() error {
	return s.withInboxFixtureClock(func() error {
		return s.conversionCreateProposal("other", "ana@example.com", "pedro@example.com", mustActivityDateTime("2026-09-02 00:00"), mustActivityDateTime("2026-09-03 00:00"), mustActivityDateTime("2026-09-04 00:00"), mustActivityDateTime("2026-09-05 00:00"), serviceproposal.StatusAccepted, 1)
	})
}
func (s *testSuite) conversionBoundaryProposal(value string) error {
	at, err := conversionDate(value)
	if err != nil {
		return err
	}
	return s.withInboxFixtureClock(func() error {
		return s.conversionCreateProposal("boundary", "ana@example.com", "juan@example.com", at, time.Time{}, time.Time{}, time.Time{}, serviceproposal.StatusPending, 1)
	})
}
func (s *testSuite) conversionSharedConversationProposals() error {
	return s.withInboxFixtureClock(func() error {
		for index := 0; index < 2; index++ {
			if err := s.conversionCreateProposal(fmt.Sprintf("shared-%d", index), "ana@example.com", "juan@example.com", mustActivityDateTime("2026-09-02 00:00").Add(time.Duration(index)*24*time.Hour), time.Time{}, time.Time{}, time.Time{}, serviceproposal.StatusPending, 1); err != nil {
				return err
			}
		}
		first, second := s.operationInbox.proposals["shared-0"], s.operationInbox.proposals["shared-1"]
		if first.id == second.id || first.requestLabel != second.requestLabel {
			return fmt.Errorf("conversion proposals do not have distinct IDs in the same conversation")
		}
		return nil
	})
}
func (s *testSuite) conversionBothProposalsAdvance() error {
	return s.withInboxFixtureClock(func() error {
		for index, label := range []string{"shared-0", "shared-1"} {
			created := s.conversionState().proposals[label].created
			if err := s.conversionAdvanceProposal(label, created.Add(24*time.Hour), created.Add(96*time.Hour), created.Add(120*time.Hour), 3-index*2); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionThreeImages() error {
	order, err := s.workOrderRepository.FindByID(s.scenarioContext, s.operationInbox.orders["conversion-order-shared-0"])
	if err != nil {
		return err
	}
	if order.CompletionReport() == nil || len(order.CompletionReport().ImageFileIDs()) != 3 {
		return fmt.Errorf("conversion first order does not have three persisted completion images")
	}
	return nil
}
func (s *testSuite) conversionMultiplePaymentAttempts() error {
	return s.inboxProposalHadRejectedThenApprovedDeposit("shared-1")
}
func (s *testSuite) conversionHalfCentFixtures() error {
	return s.withInboxFixtureClock(func() error {
		for index := 0; index < 32; index++ {
			created := mustActivityDateTime("2026-09-01 00:00").Add(time.Duration(index%28) * 24 * time.Hour)
			accepted := time.Time{}
			status := serviceproposal.StatusPending
			if index == 0 {
				accepted = mustActivityDateTime("2026-09-10 00:00")
				status = serviceproposal.StatusAccepted
			}
			if err := s.conversionCreateProposal(fmt.Sprintf("rounding-%d", index), "ana@example.com", "juan@example.com", created, accepted, time.Time{}, time.Time{}, status, 1); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionUncontractedFixtures() error {
	return s.withInboxFixtureClock(func() error {
		for index := 0; index < 3; index++ {
			if err := s.conversionCreateProposal(fmt.Sprintf("no-order-%d", index), "ana@example.com", "juan@example.com", mustActivityDateTime("2026-09-01 00:00").Add(time.Duration(index)*24*time.Hour), time.Time{}, time.Time{}, time.Time{}, serviceproposal.StatusPending, 1); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionRequestsTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	return s.withInboxFixtureClock(func() error {
		for _, row := range rows {
			created, err := conversionDate(row["recibida el"])
			if err != nil {
				return err
			}
			status := "pending"
			if row["estado actual"] == "aceptada" {
				status = "accepted"
			}
			// Ana's acceptance is explicitly performed after the period in the next Given.
			if row["cliente"] == "ana@example.com" && row["prestador"] == "juan@example.com" {
				status = "pending"
			}
			label := "request-" + row["prestador"] + "-" + row["cliente"]
			if err := s.conversionRequest(label, row["cliente"], row["prestador"], created, status); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionRequestAcceptedAfterPeriod() error {
	return s.withInboxFixtureClock(func() error {
		if err := s.setInboxFixtureClock(mustActivityDateTime("2026-09-10 00:00")); err != nil {
			return err
		}
		return s.acceptInboxJobRequest(s.conversionState().requestByPair["ana@example.com\x00juan@example.com"])
	})
}
func (s *testSuite) conversionCarlaProposals() error {
	return s.withInboxFixtureClock(func() error {
		label := s.conversionState().requestByPair["carla@example.com\x00juan@example.com"]
		request, err := s.jobRequestRepository.FindByID(s.operationInbox.requests[label].id)
		if err != nil {
			return err
		}
		if request.Status != "accepted" || !request.CreatedOn.Equal(mustActivityDateTime("2026-09-03 00:00")) {
			return fmt.Errorf("Carla request is not the existing accepted September 3 request")
		}
		for index := 0; index < 4; index++ {
			accepted := time.Time{}
			status := serviceproposal.StatusPending
			if index == 0 {
				accepted = mustActivityDateTime("2026-09-10 00:00")
				status = serviceproposal.StatusAccepted
			}
			proposalLabel := fmt.Sprintf("carla-%d", index)
			if err := s.conversionCreateProposal(proposalLabel, "carla@example.com", "juan@example.com", mustActivityDateTime("2026-09-04 00:00"), accepted, time.Time{}, time.Time{}, status, 1); err != nil {
				return err
			}
			if s.operationInbox.proposals[proposalLabel].requestLabel != label {
				return fmt.Errorf("Carla proposal created an unrelated request")
			}
		}
		return nil
	})
}
func (s *testSuite) conversionNoProposals() error {
	s.conversionState()
	actor, err := s.userRepository.FindByAuthID(auth0IDForProviderEmail("juan@example.com"))
	if err != nil {
		return err
	}
	proposals, err := repositories.NewServiceProposalRepository(s.database).FindByUserID(s.scenarioContext, actor.ID())
	if err != nil {
		return err
	}
	for _, proposal := range proposals {
		if !proposal.CreatedOn.Before(mustActivityDateTime("2026-09-01 00:00")) && proposal.CreatedOn.Before(mustActivityDateTime("2026-09-05 00:00")) {
			return fmt.Errorf("unexpected proposal %d in empty conversion fixture", proposal.ID)
		}
	}
	return nil
}
func (s *testSuite) conversionPendingRequest() error {
	return s.withInboxFixtureClock(func() error {
		return s.conversionRequest("pending", "ana@example.com", "juan@example.com", mustActivityDateTime("2026-09-03 00:00"), "pending")
	})
}
func (s *testSuite) conversionNoRecords() error {
	s.conversionState()
	actor, err := s.userRepository.FindByAuthID(auth0IDForProviderEmail("juan@example.com"))
	if err != nil {
		return err
	}
	proposals, err := repositories.NewServiceProposalRepository(s.database).FindByUserID(s.scenarioContext, actor.ID())
	if err != nil {
		return err
	}
	from, to := mustActivityDateTime("2026-08-30 00:00"), mustActivityDateTime("2026-09-30 00:00")
	for _, proposal := range proposals {
		if !proposal.CreatedOn.Before(from) && proposal.CreatedOn.Before(to) {
			return fmt.Errorf("unexpected persisted proposal in empty conversion period")
		}
	}
	requests, err := s.jobRequestRepository.FindByUserAuthID(auth0IDForProviderEmail("juan@example.com"))
	if err != nil {
		return err
	}
	for _, summary := range requests {
		request, err := s.jobRequestRepository.FindByID(summary.ID)
		if err != nil {
			return err
		}
		if !request.CreatedOn.Before(from) && request.CreatedOn.Before(to) {
			return fmt.Errorf("unexpected persisted request in empty conversion period")
		}
	}
	return nil
}
func (s *testSuite) conversionDefaultBoundaryFixtures() error {
	return s.withInboxFixtureClock(func() error {
		from, to := mustActivityDateTime("2026-08-30 12:00"), mustActivityDateTime("2026-09-29 12:00")
		for index, at := range []time.Time{from.Add(-time.Microsecond), from, to.Add(-time.Microsecond), to} {
			if err := s.conversionCreateProposal(fmt.Sprintf("default-%d", index), "ana@example.com", "juan@example.com", at, time.Time{}, time.Time{}, time.Time{}, serviceproposal.StatusPending, 1); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *testSuite) conversionPrivacyFixtures() error {
	return s.withInboxFixtureClock(func() error {
		for _, consumer := range []string{"ana@example.com", "carla@example.com"} {
			if err := s.conversionRequest("privacy-request-"+consumer, consumer, "juan@example.com", mustActivityDateTime("2026-09-01 00:00"), "accepted"); err != nil {
				return err
			}
		}
		if err := s.conversionCreateProposal("privacy-pending", "ana@example.com", "juan@example.com", mustActivityDateTime("2026-09-02 00:00"), time.Time{}, time.Time{}, time.Time{}, serviceproposal.StatusPending, 1); err != nil {
			return err
		}
		if err := s.conversionCreateProposal("privacy-paid", "carla@example.com", "juan@example.com", mustActivityDateTime("2026-09-03 00:00"), mustActivityDateTime("2026-09-10 00:00"), mustActivityDateTime("2026-09-12 00:00"), mustActivityDateTime("2026-09-15 00:00"), serviceproposal.StatusAccepted, 1); err != nil {
			return err
		}
		messageConversationID := s.operationInbox.requests["privacy-request-carla@example.com"].conversationID
		if _, err := (testsupport.OperationChatFixture{DB: s.database}).AddMessage(s.scenarioContext, messageConversationID, conversation.SenderConsumer, "Conversion confidential customer message", mustActivityDateTime("2026-09-20 00:00")); err != nil {
			return err
		}
		s.conversionState().preserveRecords = true
		return nil
	})
}
func (s *testSuite) conversionPrivacyFixtureEvidence() error {
	from, to := mustActivityDateTime("2026-09-01 00:00"), mustActivityDateTime("2026-09-29 00:00")
	within := func(at time.Time) bool { return !at.Before(from) && at.Before(to) }
	for _, fixture := range s.operationInbox.requests {
		request, err := s.jobRequestRepository.FindByID(fixture.id)
		if err != nil {
			return err
		}
		if !within(request.CreatedOn) {
			return fmt.Errorf("privacy request created outside stipulated period")
		}
		conversation, err := s.conversationRepository.FindByID(s.scenarioContext, fixture.conversationID)
		if err != nil {
			return err
		}
		for _, message := range conversation.Messages() {
			if !within(message.CreatedOn) {
				return fmt.Errorf("privacy message created outside stipulated period")
			}
		}
	}
	for _, fixture := range s.conversionState().proposals {
		if !within(fixture.created) {
			return fmt.Errorf("privacy proposal created outside stipulated period")
		}
	}
	actor, err := s.userRepository.FindByAuthID(auth0IDForProviderEmail("juan@example.com"))
	if err != nil {
		return err
	}
	account, err := s.paymentAccountRepository.FindByProviderID(s.scenarioContext, actor.ID(), paymentaccount.PaymentProvider("mercadopago"))
	if !errors.Is(err, paymentaccount.ErrConnectionNotFound) || account != nil {
		return fmt.Errorf("privacy provider unexpectedly has a payment account: %v", err)
	}
	return nil
}

func (s *testSuite) conversionQueryPeriod(start, end string) error {
	from, err := conversionDate(start)
	if err != nil {
		return err
	}
	to, err := conversionDate(end)
	if err != nil {
		return err
	}
	return s.requestConversion(url.Values{"from": {from.Format(time.RFC3339Nano)}, "to": {to.Format(time.RFC3339Nano)}})
}
func (s *testSuite) conversionQueryDefault() error { return s.requestConversion(nil) }
func (s *testSuite) conversionQueryInvalid(choice string) error {
	from, to, now := mustActivityDateTime("2026-09-01 00:00"), mustActivityDateTime("2026-09-05 00:00"), mustActivityDateTime("2026-09-29 12:00")
	query := url.Values{"from": {from.Format(time.RFC3339Nano)}, "to": {to.Format(time.RFC3339Nano)}}
	switch choice {
	case "sin indicar el comienzo":
		query.Del("from")
	case "sin indicar el final":
		query.Del("to")
	case "con el mismo comienzo y final":
		query.Set("to", query.Get("from"))
	case "con un final anterior al comienzo":
		query.Set("from", to.Format(time.RFC3339Nano))
		query.Set("to", from.Format(time.RFC3339Nano))
	case "por más de 365 días":
		query.Set("from", to.Add(-366*24*time.Hour).Format(time.RFC3339Nano))
	case "con un final posterior al momento actual":
		query.Set("to", now.Add(time.Minute).Format(time.RFC3339Nano))
	case "con una fecha sin indicar su zona horaria":
		query.Set("from", "2026-09-01T00:00:00")
	case "con una fecha en un formato no admitido":
		query.Set("from", "01/09/2026")
	default:
		return fmt.Errorf("unsupported invalid conversion period %q", choice)
	}
	return s.requestConversion(query)
}
func (s *testSuite) requestConversion(query url.Values) error {
	state := s.conversionState()
	state.query, state.responseValid = query, false
	if err := s.requestTestClockMock("2026-09-29T12:00:00-03:00"); err != nil {
		return err
	}
	if state.preserveRecords {
		before, err := s.conversionPersistedRecords()
		if err != nil {
			return err
		}
		state.before = before
	}
	request, err := http.NewRequest(http.MethodGet, s.server.URL+conversionStatisticsPath+queryString(query), nil)
	if err != nil {
		return err
	}
	state.expectedAuthorization = ""
	if s.invalidSession {
		state.expectedAuthorization = "Bearer invalid.conversion.token"
	} else if s.currentAuth0ID != "" {
		state.expectedAuthorization = "Bearer " + s.tokenBuilder.BuildToken(s.currentAuth0ID, s.currentPermissions)
	}
	if state.expectedAuthorization != "" {
		request.Header.Set("Authorization", state.expectedAuthorization)
	}
	state.authorization = request.Header.Get("Authorization")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	state.cacheControl = response.Header.Get("Cache-Control")
	s.lastStatus = response.StatusCode
	s.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if state.cacheControl != "private, no-store" {
		return fmt.Errorf("conversion response cache header %q, want private, no-store", state.cacheControl)
	}
	if response.StatusCode == http.StatusOK {
		if err := conversionResponseShape(s.lastBody); err != nil {
			return err
		}
		if err := json.Unmarshal(s.lastBody, &state.response); err != nil {
			return err
		}
		state.responseValid = true
	}
	return nil
}
func conversionObject(raw json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil || len(object) != len(keys) {
		return nil, fmt.Errorf("conversion object has wrong keys: %s; want %v", raw, keys)
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return nil, fmt.Errorf("conversion response missing %s: %s", key, raw)
		}
	}
	return object, nil
}
func conversionResponseShape(raw []byte) error {
	root, err := conversionObject(raw, "period", "observed_at", "proposals", "requests")
	if err != nil {
		return err
	}
	period, err := conversionObject(root["period"], "from", "to", "time_zone")
	if err != nil {
		return err
	}
	for _, rawInstant := range []json.RawMessage{period["from"], period["to"], root["observed_at"]} {
		var value string
		if err := json.Unmarshal(rawInstant, &value); err != nil {
			return err
		}
		if !strings.HasSuffix(value, "Z") {
			return fmt.Errorf("conversion timestamp not normalized to UTC: %s", value)
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			return err
		}
	}
	proposals, err := conversionObject(root["proposals"], "stages", "rates", "uncontracted")
	if err != nil {
		return err
	}
	stages, err := conversionObject(proposals["stages"], "issued", "contracted", "reported", "paid")
	if err != nil {
		return err
	}
	checkCount := func(raw json.RawMessage) error {
		var count *int64
		if err := json.Unmarshal(raw, &count); err != nil {
			return err
		}
		if count == nil || *count < 0 {
			return fmt.Errorf("conversion count must be a nonnegative integer, got %s", raw)
		}
		return nil
	}
	for _, count := range stages {
		if err := checkCount(count); err != nil {
			return err
		}
	}
	if err := checkCount(proposals["uncontracted"]); err != nil {
		return err
	}
	rates, err := conversionObject(proposals["rates"], "contracted", "reported", "paid")
	if err != nil {
		return err
	}
	checkRatio := func(raw json.RawMessage) error {
		ratio, err := conversionObject(raw, "numerator", "denominator", "percentage")
		if err != nil {
			return err
		}
		for _, key := range []string{"numerator", "denominator"} {
			if err := checkCount(ratio[key]); err != nil {
				return err
			}
		}
		var percentage *float64
		return json.Unmarshal(ratio["percentage"], &percentage)
	}
	for _, stage := range []string{"contracted", "reported", "paid"} {
		stageRates, err := conversionObject(rates[stage], "cohort", "previous_stage")
		if err != nil {
			return err
		}
		for _, kind := range []string{"cohort", "previous_stage"} {
			if err := checkRatio(stageRates[kind]); err != nil {
				return err
			}
		}
	}
	requests, err := conversionObject(root["requests"], "received", "accepted", "pending", "acceptance_rate")
	if err != nil {
		return err
	}
	for _, key := range []string{"received", "accepted", "pending"} {
		if err := checkCount(requests[key]); err != nil {
			return err
		}
	}
	return checkRatio(requests["acceptance_rate"])
}
func (s *testSuite) conversionResponse() (conversionStatisticsResponse, error) {
	state := s.conversionState()
	if s.lastStatus != http.StatusOK || !state.responseValid {
		return conversionStatisticsResponse{}, fmt.Errorf("conversion expected 200 and decoded response, got %d: %s", s.lastStatus, s.lastBody)
	}
	if state.authorization == "" || state.authorization != state.expectedAuthorization {
		return conversionStatisticsResponse{}, fmt.Errorf("conversion successful request did not carry the actual expected JWT")
	}
	if state.response.Period.TimeZone != "America/Argentina/Buenos_Aires" {
		return conversionStatisticsResponse{}, fmt.Errorf("unexpected conversion timezone %s", state.response.Period.TimeZone)
	}
	return state.response, nil
}
func conversionAssertRatio(actual conversionRatio, numerator, denominator int64, percentage *float64) error {
	if actual.Numerator != numerator || actual.Denominator != denominator {
		return fmt.Errorf("conversion ratio counts %d/%d, want %d/%d", actual.Numerator, actual.Denominator, numerator, denominator)
	}
	if (actual.Percentage == nil) != (percentage == nil) || (percentage != nil && *actual.Percentage != *percentage) {
		return fmt.Errorf("conversion ratio percentage %s, want %s", activityFloatString(actual.Percentage), activityFloatString(percentage))
	}
	return nil
}
func conversionPercent(value float64) *float64 { return &value }
func (s *testSuite) conversionStageCounts(issued, contracted, reported, paid int64) error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	expected := conversionStages{Issued: issued, Contracted: contracted, Reported: reported, Paid: paid}
	if response.Proposals.Stages != expected {
		return fmt.Errorf("conversion stages %+v, want %+v", response.Proposals.Stages, expected)
	}
	return nil
}
func (s *testSuite) conversionStagesTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	actual := map[string]int64{"emitidas": response.Proposals.Stages.Issued, "contratadas": response.Proposals.Stages.Contracted, "con finalización informada": response.Proposals.Stages.Reported, "con pago completo": response.Proposals.Stages.Paid}
	if len(rows) != 4 {
		return fmt.Errorf("conversion stage table requires four stages")
	}
	for _, row := range rows {
		count, err := strconv.ParseInt(row["propuestas"], 10, 64)
		if err != nil {
			return err
		}
		value, ok := actual[row["etapa"]]
		if !ok || value != count {
			return fmt.Errorf("conversion stage %s is %d, want %d", row["etapa"], value, count)
		}
		delete(actual, row["etapa"])
	}
	if len(actual) != 0 {
		return fmt.Errorf("conversion stage table is incomplete or duplicated")
	}
	return nil
}
func (s *testSuite) conversionIssuedCount(count int64) error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if response.Proposals.Stages.Issued != count {
		return fmt.Errorf("issued proposals %d, want %d", response.Proposals.Stages.Issued, count)
	}
	return nil
}
func (s *testSuite) conversionPaidProposalInAllStages() error {
	order, err := s.workOrderRepository.FindByID(s.scenarioContext, s.operationInbox.orders["conversion-order-P1"])
	if err != nil {
		return err
	}
	if order.Status() != workorder.StatusPaid || order.CompletionReport() == nil || order.AcceptedOn().IsZero() || order.PaidOn().IsZero() {
		return fmt.Errorf("P1 lacks persisted evidence of all four stages")
	}
	return s.conversionStageCounts(4, 2, 1, 1)
}
func (s *testSuite) conversionLaterMilestoneCounts() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	order, err := s.workOrderRepository.FindByID(s.scenarioContext, s.operationInbox.orders["conversion-order-P1"])
	if err != nil {
		return err
	}
	if !order.AcceptedOn().After(response.Period.To) {
		return fmt.Errorf("P1 confirmation was not after the cohort period")
	}
	return conversionAssertRatio(response.Proposals.Rates.Contracted.Cohort, 2, 4, conversionPercent(50))
}
func (s *testSuite) conversionPriorProposalsExcluded() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	for index := 0; index < 4; index++ {
		label := fmt.Sprintf("prior-%d", index)
		proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(s.scenarioContext, s.operationInbox.proposals[label].id)
		if err != nil {
			return err
		}
		order, err := s.workOrderRepository.FindByID(s.scenarioContext, s.operationInbox.orders["conversion-order-"+label])
		if err != nil {
			return err
		}
		if !proposal.CreatedOn.Before(response.Period.From) || order.AcceptedOn().Before(response.Period.From) || !order.AcceptedOn().Before(response.Period.To) {
			return fmt.Errorf("prior proposal fixture does not contrast creation and milestone periods")
		}
	}
	return s.conversionStageCounts(4, 2, 1, 1)
}
func (s *testSuite) conversionCohortNotActivity() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if err := conversionAssertRatio(response.Proposals.Rates.Contracted.Cohort, 2, 4, conversionPercent(50)); err != nil {
		return err
	}
	activityConfirmations := 0
	for _, fixture := range s.conversionState().proposals {
		if fixture.providerEmail == "juan@example.com" && !fixture.accepted.IsZero() && !fixture.accepted.Before(response.Period.From) && fixture.accepted.Before(response.Period.To) {
			activityConfirmations++
		}
	}
	if activityConfirmations != 5 || *response.Proposals.Rates.Contracted.Cohort.Percentage == float64(activityConfirmations)*100/4 {
		return fmt.Errorf("fixture did not distinguish activity confirmations from cohort conversion")
	}
	return nil
}
func (s *testSuite) conversionOtherProviderExcluded() error {
	proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(s.scenarioContext, s.operationInbox.proposals["other"].id)
	if err != nil {
		return err
	}
	other, err := s.providerIDByEmail("pedro@example.com")
	if err != nil {
		return err
	}
	if proposal.Provider.ID() != other {
		return fmt.Errorf("other provider fixture is not owned by Pedro")
	}
	return s.conversionStageCounts(4, 2, 1, 1)
}
func (s *testSuite) conversionNoComparison() error {
	if _, err := s.conversionResponse(); err != nil {
		return err
	}
	return conversionResponseShape(s.lastBody)
}
func (s *testSuite) conversionRatesTable(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	actual := map[string]conversionRatio{
		"contratación sobre la cohorte": response.Proposals.Rates.Contracted.Cohort, "contratación sobre la etapa anterior": response.Proposals.Rates.Contracted.PreviousStage,
		"finalización sobre la cohorte": response.Proposals.Rates.Reported.Cohort, "finalización sobre la etapa anterior": response.Proposals.Rates.Reported.PreviousStage,
		"pago sobre la cohorte": response.Proposals.Rates.Paid.Cohort, "pago sobre la etapa anterior": response.Proposals.Rates.Paid.PreviousStage,
	}
	if len(rows) != 6 {
		return fmt.Errorf("conversion rate table requires six rates")
	}
	for _, row := range rows {
		numerator, err := strconv.ParseInt(row["numerador"], 10, 64)
		if err != nil {
			return err
		}
		text, _, _ := strings.Cut(row["denominador"], " ")
		denominator, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return err
		}
		percentage, err := activityPercentage(row["resultado"])
		if err != nil {
			return err
		}
		value, ok := actual[row["tasa"]]
		if !ok {
			return fmt.Errorf("unknown or duplicate rate %s", row["tasa"])
		}
		if err := conversionAssertRatio(value, numerator, denominator, percentage); err != nil {
			return fmt.Errorf("rate %s: %w", row["tasa"], err)
		}
		delete(actual, row["tasa"])
	}
	if len(actual) != 0 {
		return fmt.Errorf("conversion rate table omitted rates")
	}
	return nil
}
func (s *testSuite) conversionHalfCentPercentage() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if err := s.conversionStageCounts(32, 1, 0, 0); err != nil {
		return err
	}
	return conversionAssertRatio(response.Proposals.Rates.Contracted.Cohort, 1, 32, conversionPercent(3.13))
}
func (s *testSuite) conversionNoAdvances() error { return s.conversionStageCounts(3, 0, 0, 0) }
func (s *testSuite) conversionZeroCohortRates() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	for _, ratio := range []conversionRatio{response.Proposals.Rates.Contracted.Cohort, response.Proposals.Rates.Reported.Cohort, response.Proposals.Rates.Paid.Cohort} {
		if err := conversionAssertRatio(ratio, 0, 3, conversionPercent(0)); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) conversionZeroContractingRate() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	return conversionAssertRatio(response.Proposals.Rates.Contracted.PreviousStage, 0, 3, conversionPercent(0))
}
func (s *testSuite) conversionNullAdvanceRates() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	for _, ratio := range []conversionRatio{response.Proposals.Rates.Reported.PreviousStage, response.Proposals.Rates.Paid.PreviousStage} {
		if err := conversionAssertRatio(ratio, 0, 0, nil); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) conversionUncontractedCount() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if err := s.conversionStageCounts(3, 1, 0, 0); err != nil {
		return err
	}
	if response.Proposals.Uncontracted != 2 || response.Proposals.Uncontracted != response.Proposals.Stages.Issued-response.Proposals.Stages.Contracted {
		return fmt.Errorf("uncontracted count did not equal issued minus contracted")
	}
	return nil
}
func (s *testSuite) conversionProposalStatesPreserved() error {
	for label, want := range map[string]serviceproposal.Status{"P1": serviceproposal.StatusAccepted, "P2": serviceproposal.StatusPending, "P3": serviceproposal.StatusRejected} {
		proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(s.scenarioContext, s.operationInbox.proposals[label].id)
		if err != nil {
			return err
		}
		if proposal.Status != want {
			return fmt.Errorf("proposal %s changed state to %s, want %s", label, proposal.Status, want)
		}
	}
	return nil
}
func (s *testSuite) conversionPendingNotRejected() error {
	proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(s.scenarioContext, s.operationInbox.proposals["P2"].id)
	if err != nil {
		return err
	}
	if proposal.Status != serviceproposal.StatusPending {
		return fmt.Errorf("P2 was relabeled as %s", proposal.Status)
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if response.Proposals.Uncontracted != 2 {
		return fmt.Errorf("pending and rejected proposals were not both counted as uncontracted")
	}
	return conversionResponseShape(s.lastBody)
}
func (s *testSuite) conversionRequestCounts(received, accepted, pending int64) error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if response.Requests.Received != received || response.Requests.Accepted != accepted || response.Requests.Pending != pending {
		return fmt.Errorf("conversion request counts %d/%d/%d, want %d/%d/%d", response.Requests.Received, response.Requests.Accepted, response.Requests.Pending, received, accepted, pending)
	}
	return nil
}
func (s *testSuite) conversionAcceptancePercentage() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	return conversionAssertRatio(response.Requests.AcceptanceRate, 2, 3, conversionPercent(66.67))
}
func (s *testSuite) conversionCurrentRequestStatusCounts() error {
	label := s.conversionState().requestByPair["ana@example.com\x00juan@example.com"]
	request, err := s.jobRequestRepository.FindByID(s.operationInbox.requests[label].id)
	if err != nil {
		return err
	}
	if request.Status != "accepted" {
		return fmt.Errorf("Ana request was not persisted accepted")
	}
	if err := s.conversionRequestCounts(3, 2, 1); err != nil {
		return err
	}
	return s.conversionAcceptancePercentage()
}
func (s *testSuite) conversionIndependentUniverses() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if err := s.conversionStageCounts(4, 1, 0, 0); err != nil {
		return err
	}
	if err := conversionAssertRatio(response.Proposals.Rates.Contracted.Cohort, 1, 4, conversionPercent(25)); err != nil {
		return err
	}
	return conversionAssertRatio(response.Requests.AcceptanceRate, 2, 3, conversionPercent(66.67))
}
func (s *testSuite) conversionRequestsOutsidePeriodExcluded() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	outside, other := 0, 0
	for _, fixture := range s.operationInbox.requests {
		request, err := s.jobRequestRepository.FindByID(fixture.id)
		if err != nil {
			return err
		}
		if fixture.providerEmail == "pedro@example.com" {
			other++
			continue
		}
		if request.CreatedOn.Before(response.Period.From) || !request.CreatedOn.Before(response.Period.To) {
			outside++
		}
	}
	if outside != 2 || other != 2 {
		return fmt.Errorf("request boundary/isolation witnesses missing: outside=%d other=%d", outside, other)
	}
	return s.conversionRequestCounts(3, 2, 1)
}
func (s *testSuite) conversionEmptyProposals() error {
	if err := s.conversionStageCounts(0, 0, 0, 0); err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if response.Proposals.Uncontracted != 0 {
		return fmt.Errorf("empty cohort has uncontracted proposals")
	}
	for _, stage := range []conversionStageRates{response.Proposals.Rates.Contracted, response.Proposals.Rates.Reported, response.Proposals.Rates.Paid} {
		for _, ratio := range []conversionRatio{stage.Cohort, stage.PreviousStage} {
			if err := conversionAssertRatio(ratio, 0, 0, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *testSuite) conversionPendingRequestOnly() error {
	if err := s.conversionRequestCounts(1, 0, 1); err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	return conversionAssertRatio(response.Requests.AcceptanceRate, 0, 1, conversionPercent(0))
}
func (s *testSuite) conversionEmptyCounts() error {
	if err := s.conversionStageCounts(0, 0, 0, 0); err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if response.Proposals.Uncontracted != 0 {
		return fmt.Errorf("empty results contain uncontracted proposals")
	}
	return s.conversionRequestCounts(0, 0, 0)
}
func (s *testSuite) conversionAllRatesNull() error {
	if err := s.conversionEmptyProposals(); err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	return conversionAssertRatio(response.Requests.AcceptanceRate, 0, 0, nil)
}
func (s *testSuite) conversionDefaultPeriod() error {
	if err := s.conversionIssuedCount(2); err != nil {
		return err
	}
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if !response.Period.From.Equal(mustActivityDateTime("2026-08-30 12:00")) || !response.Period.To.Equal(mustActivityDateTime("2026-09-29 12:00")) {
		return fmt.Errorf("conversion default period %s--%s differs from exact last 30 days", response.Period.From, response.Period.To)
	}
	return nil
}
func (s *testSuite) conversionObservedAt() error {
	response, err := s.conversionResponse()
	if err != nil {
		return err
	}
	if !response.ObservedAt.Equal(mustActivityDateTime("2026-09-29 12:00")) {
		return fmt.Errorf("conversion observation time %s is not the controlled query time", response.ObservedAt)
	}
	return nil
}
func (s *testSuite) conversionRejected(status int) error {
	state := s.conversionState()
	if s.lastStatus != status {
		return fmt.Errorf("conversion rejection status %d, want %d: %s", s.lastStatus, status, s.lastBody)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(s.lastBody, &body); err != nil {
		return err
	}
	var message string
	if err := json.Unmarshal(body["error"], &message); err != nil || message == "" {
		return fmt.Errorf("conversion rejection lacks a controlled error: %s", s.lastBody)
	}
	for key, value := range body {
		if key == "error" {
			continue
		}
		if status == http.StatusUnauthorized && key == "message" {
			var detail string
			if err := json.Unmarshal(value, &detail); err != nil || detail == "" {
				return fmt.Errorf("auth rejection has invalid message")
			}
			continue
		}
		return fmt.Errorf("conversion rejection exposes unexpected field %s: %s", key, s.lastBody)
	}
	if state.responseValid {
		return fmt.Errorf("conversion rejection retained successful results")
	}
	if state.cacheControl != "private, no-store" {
		return fmt.Errorf("conversion rejection lacks private no-store cache header")
	}
	return nil
}
func (s *testSuite) conversionPeriodRejected() error {
	return s.conversionRejected(http.StatusBadRequest)
}
func (s *testSuite) conversionIdentityRejected(reason string) error {
	state := s.conversionState()
	status := http.StatusUnauthorized
	switch reason {
	case "se rechaza la consulta porque no soy prestador":
		status = http.StatusForbidden
	case "se rechaza la consulta porque no tengo una cuenta en LoResuelvo":
		status = http.StatusNotFound
	case "se rechaza la consulta porque no inicié sesión":
		if state.authorization != "" {
			return fmt.Errorf("conversion missing-session request sent Authorization")
		}
	case "se rechaza la consulta porque mi sesión no es válida":
		if state.authorization != "Bearer invalid.conversion.token" {
			return fmt.Errorf("conversion invalid-session request did not send invalid token")
		}
	default:
		return fmt.Errorf("unsupported conversion identity rejection %q", reason)
	}
	if status != http.StatusUnauthorized && (state.authorization == "" || state.authorization != state.expectedAuthorization) {
		return fmt.Errorf("conversion authenticated rejection omitted or changed its outgoing JWT")
	}
	return s.conversionRejected(status)
}
func (s *testSuite) conversionAggregatePrivacy() error {
	if err := s.conversionStageCounts(2, 1, 1, 1); err != nil {
		return err
	}
	if err := s.conversionRequestCounts(2, 2, 0); err != nil {
		return err
	}
	if err := conversionResponseShape(s.lastBody); err != nil {
		return err
	}
	for _, secret := range []string{"@example.com", "Conversion confidential customer message", "Conversion private proposal", "access_token", "refresh_token", "mp-", "https://checkout"} {
		if strings.Contains(string(s.lastBody), secret) {
			return fmt.Errorf("conversion aggregate leaked private marker %q", secret)
		}
	}
	return nil
}
func (s *testSuite) conversionPrivateCache() error {
	if _, err := s.conversionResponse(); err != nil {
		return err
	}
	if s.conversionState().cacheControl != "private, no-store" {
		return fmt.Errorf("conversion results cache header is %q", s.conversionState().cacheControl)
	}
	return nil
}
func (s *testSuite) conversionPersistedRecords() (map[string]any, error) {
	records := make(map[string]any)
	for label, fixture := range s.operationInbox.requests {
		request, err := s.jobRequestRepository.FindByID(fixture.id)
		if err != nil {
			return nil, err
		}
		records["request/"+label] = request
		conversation, err := s.conversationRepository.FindByID(s.scenarioContext, fixture.conversationID)
		if err != nil {
			return nil, err
		}
		records["conversation/"+label] = conversation
	}
	for label, fixture := range s.operationInbox.proposals {
		proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(s.scenarioContext, fixture.id)
		if err != nil {
			return nil, err
		}
		records["proposal/"+label] = proposal
	}
	for label, id := range s.operationInbox.orders {
		order, err := s.workOrderRepository.FindByID(s.scenarioContext, id)
		if err != nil {
			return nil, err
		}
		records["order/"+label] = order
	}
	return records, nil
}
func (s *testSuite) conversionRecordsPreserved() error {
	state := s.conversionState()
	if !state.preserveRecords || len(state.before) != 7 {
		return fmt.Errorf("privacy readback did not capture two requests, two conversations, two proposals and one order before When")
	}
	after, err := s.conversionPersistedRecords()
	if err != nil {
		return err
	}
	if len(after) != len(state.before) {
		return fmt.Errorf("conversion changed the persisted record set")
	}
	for key, before := range state.before {
		if !reflect.DeepEqual(before, after[key]) {
			return fmt.Errorf("conversion mutated persisted %s", key)
		}
	}
	conversation, err := s.conversationRepository.FindByID(s.scenarioContext, s.operationInbox.requests["privacy-request-carla@example.com"].conversationID)
	if err != nil {
		return err
	}
	found := false
	for _, message := range conversation.Messages() {
		if message.Content == "Conversion confidential customer message" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("conversion fixture message was not present on independent persisted readback")
	}
	return nil
}

func TestConversionResponseShapeRejectsMissingNullPercentage(t *testing.T) {
	valid := `{"period":{"from":"2026-09-01T00:00:00Z","to":"2026-09-02T00:00:00Z","time_zone":"America/Argentina/Buenos_Aires"},"observed_at":"2026-09-29T15:00:00Z","proposals":{"stages":{"issued":0,"contracted":0,"reported":0,"paid":0},"rates":{"contracted":{"cohort":{"numerator":0,"denominator":0,"percentage":null},"previous_stage":{"numerator":0,"denominator":0,"percentage":null}},"reported":{"cohort":{"numerator":0,"denominator":0,"percentage":null},"previous_stage":{"numerator":0,"denominator":0,"percentage":null}},"paid":{"cohort":{"numerator":0,"denominator":0,"percentage":null},"previous_stage":{"numerator":0,"denominator":0,"percentage":null}}},"uncontracted":0},"requests":{"received":0,"accepted":0,"pending":0,"acceptance_rate":{"numerator":0,"denominator":0,"percentage":null}}}`
	if err := conversionResponseShape([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{strings.Replace(valid, `,"percentage":null`, "", 1), strings.Replace(valid, `"observed_at":`, `"comparison":null,"observed_at":`, 1), strings.Replace(valid, `00:00:00Z`, `00:00:00-03:00`, 1), strings.Replace(valid, `"issued":0`, `"issued":null`, 1)} {
		if err := conversionResponseShape([]byte(invalid)); err == nil {
			t.Fatal("strict conversion schema accepted an omitted null, reserved field or unnormalized timestamp")
		}
	}
}
