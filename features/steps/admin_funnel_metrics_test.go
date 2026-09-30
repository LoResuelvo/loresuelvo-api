package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

const adminFunnelPath = "/admin/metrics/funnel"

var errInjectedAdminFunnelRead = errors.New("injected administrative funnel read failure")

type adminFunnelTestCapture struct {
	failRead     bool
	readAttempts int
}

func (capture *adminFunnelTestCapture) reset() {
	capture.failRead = false
	capture.readAttempts = 0
}

type adminFunnelReaderDecorator struct {
	inner   operation.FunnelReader
	capture *adminFunnelTestCapture
}

func (reader adminFunnelReaderDecorator) Read(ctx context.Context, criteria operation.FunnelCriteria) (readmodel.FunnelSnapshot, error) {
	reader.capture.readAttempts++
	if reader.capture.failRead {
		return readmodel.FunnelSnapshot{}, errInjectedAdminFunnelRead
	}
	return reader.inner.Read(ctx, criteria)
}

type adminFunnelState struct {
	responseValid            bool
	response                 adminFunnelResponse
	query                    url.Values
	statusBefore             int
	assessmentAliases        map[string]int
	requestAliases           map[string]int
	proposalAliases          map[string]int
	proposalRequests         map[string]string
	orderAliases             map[string]int
	auditWatermark           int64
	businessCounts           map[string]int64
	requestSnapshot          map[int]*jobrequest.JobRequest
	entitySnapshot           *adminFunnelBusinessSnapshot
	chatbotRequestsBefore    int
	checkoutRequestsBefore   int
	externalSystemsOffline   bool
	readBaseline             *readmodel.FunnelSnapshot
	periodFrom               time.Time
	periodTo                 time.Time
	assessmentConversationID int
	assessmentMessageID      int
}

type adminFunnelBusinessSnapshot struct {
	Requests       map[int]*jobrequest.JobRequest
	Proposals      map[int]*serviceproposal.ServiceProposal
	Orders         map[int]*workorder.WorkOrder
	Conversations  map[int]conversation.Conversation
	CalendarEvents map[string]int
}

type adminFunnelResponse struct {
	Period struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`
	TimeZone      string    `json:"timezone"`
	ObservedAt    time.Time `json:"observed_at"`
	CategoryID    *int      `json:"category_id"`
	Rounding      string    `json:"rounding"`
	DecimalPlaces int       `json:"decimal_places"`
	Cohorts       struct {
		AI     adminFunnelCohort `json:"ai"`
		Manual adminFunnelCohort `json:"manual"`
	} `json:"cohorts"`
}

type adminFunnelCohort struct {
	CategorySource string `json:"category_source"`
	Stages         []struct {
		Stage      string          `json:"stage"`
		Count      int64           `json:"count"`
		Conversion json.RawMessage `json:"conversion_percentage"`
	} `json:"stages"`
	GlobalConversion json.RawMessage `json:"global_completion_conversion_percentage"`
	Delays           map[string]struct {
		Observations int64           `json:"observations"`
		MeanSeconds  json.RawMessage `json:"mean_seconds"`
	} `json:"delays"`
}

func registerAdminFunnelSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el reloj del sistema indica "([^"]*)"$`, suite.adminFunnelSetClock)
	sc.Step(`^consulto el embudo administrativo sin filtros$`, suite.adminFunnelQueryDefault)
	sc.Step(`^consulto el embudo administrativo$`, suite.adminFunnelQueryDefault)
	sc.Step(`^consulto el embudo desde "([^"]*)" hasta "([^"]*)"$`, suite.adminFunnelQueryPeriod)
	sc.Step(`^consulto el embudo con el rubro con ID (\d+)$`, suite.adminFunnelQueryCategoryID)
	sc.Step(`^consulto el embudo (sin filtros|para el rubro "[^"]*")$`, suite.adminFunnelQueryChoice)
	sc.Step(`^consulto el embudo con los parámetros "([^"]*)"$`, suite.adminFunnelQueryRaw)
	sc.Step(`^que el reader administrativo falla al obtener la instantánea coherente de lectura$`, suite.adminFunnelFailRead)
	sc.Step(`^el sistema responde con estado (\d+) y no devuelve métricas$`, suite.adminFunnelStatus)
	sc.Step(`^la respuesta incluye "Cache-Control: private, no-store"$`, suite.adminFunnelCacheIsPrivate)
	sc.Step(`^el período efectivo comienza exactamente treinta días antes de "([^"]*)" y termina en ese instante$`, suite.adminFunnelDefaultPeriod)
	sc.Step(`^la respuesta informa la zona horaria "([^"]*)" y el campo "observed_at" con el instante "([^"]*)"$`, suite.adminFunnelMetadata)
	sc.Step(`^ambos recorridos informan conteos cero, porcentajes sin denominador como nulos y medias sin observaciones como nulas$`, suite.adminFunnelEmptyCohorts)
	sc.Step(`^ambos recorridos contienen conteos cero, porcentajes sin denominador nulos y medias nulas$`, suite.adminFunnelEmptyCohorts)
	sc.Step(`^no se inventa actividad ni se aplica un umbral mínimo de muestras$`, suite.adminFunnelNoActivityThreshold)
	sc.Step(`^el período efectivo incluye el origen creado exactamente en "([^"]*)" y excluye los creados exactamente al final$`, suite.adminFunnelPeriodIsHalfOpen)
	sc.Step(`^el recorrido IA cuenta (\d+) evaluaciones profesionales? y el recorrido manual (\d+) solicitudes?$`, suite.adminFunnelOriginCounts)
	sc.Step(`^el recorrido IA cuenta (una|cero) y selecciona por el rubro persistido en la evaluación de origen$`, suite.adminFunnelCategoryAI)
	sc.Step(`^el recorrido manual cuenta (una|cero) y selecciona por el rubro actual del prestador destinatario, sin presentarlo como histórico$`, suite.adminFunnelCategoryManual)
	sc.Step(`^no se devuelve un error de validación por no encontrar ese rubro$`, suite.adminFunnelUnknownCategory)
	sc.Step(`^cada versión profesional persistida se cuenta como unidad independiente$`, suite.adminFunnelAssessmentVersions)
	sc.Step(`^el recorrido IA informa (\d+) evaluaciones profesionales y (\d+) con solicitud$`, suite.adminFunnelAssessmentCounts)
	sc.Step(`^ninguna etapa cuenta mensajes como unidades ni combina las versiones persistidas$`, suite.adminFunnelAssessmentVersions)
	sc.Step(`^el recorrido IA cuenta una sola unidad para "([^"]*)"$`, suite.adminFunnelOneAssessment)
	sc.Step(`^los resultados "self_service" y "collecting_information" no se incluyen en la cohorte de contratación$`, suite.adminFunnelOnlyProfessionalOutcomes)
	sc.Step(`^los mensajes y la respuesta "unchanged" no incrementan el conteo de evaluaciones$`, suite.adminFunnelOneAssessmentVersion)
	sc.Step(`^la cohorte manual contiene únicamente la solicitud sin evaluación de origen$`, suite.adminFunnelManualOnlyUnlinked)
	sc.Step(`^la cohorte IA cuenta la evaluación profesional y solo considera la solicitud vinculada a su ID de origen$`, suite.adminFunnelAIOnlyLinked)
	sc.Step(`^las dos cohortes mantienen denominadores independientes y no se suman entre sí$`, suite.adminFunnelIndependentCohorts)
	sc.Step(`^el recorrido (IA|manual) informa estos conteos y conversiones consecutivas(?: por cohorte)?:$`, suite.adminFunnelStageTable)
	sc.Step(`^las órdenes vinculadas y sus hitos persistidos determinan las etapas; los intentos o transacciones de pago no multiplican etapas ni representan por sí solos el pago completo$`, suite.adminFunnelDistinctStages)
	sc.Step(`^la conversión global desde el origen hasta "([^"]*)" es (\d+\.\d+) por ciento$`, suite.adminFunnelGlobalAI)
	sc.Step(`^la conversión global desde solicitud hasta finalización informada es (\d+\.\d+) por ciento$`, suite.adminFunnelGlobalManual)
	sc.Step(`^la conversión global IA hasta finalización informada es (\d+\.\d+) por ciento con denominador tres$`, suite.adminFunnelGlobalAIConversion)
	sc.Step(`^la conversión IA de evaluación profesional a solicitud es (\d+\.\d+) por ciento con denominador tres$`, suite.adminFunnelConversion)
	sc.Step(`^las conversiones posteriores con denominador cero son nulas y conservan sus conteos cero$`, suite.adminFunnelLaterConversionsNull)
	sc.Step(`^las conversiones manuales son nulas porque su cohorte inicial no tiene unidades$`, suite.adminFunnelManualConversionsNull)
	sc.Step(`^el recorrido IA cuenta una unidad con finalización informada por el reporte de "([^"]*)"$`, suite.adminFunnelReportedBranch)
	sc.Step(`^el recorrido IA no cuenta una unidad con pago completo porque ningún mismo recorrido de orden tiene ambos hitos de finalización y pago$`, suite.adminFunnelNoPaidBranch)
	sc.Step(`^el reporte de "([^"]*)" no se combina con el pago de "([^"]*)" para fabricar una cadena completa$`, suite.adminFunnelNoCrossBranch)
	sc.Step(`^"([^"]*)" pertenece a la cohorte por su creación dentro del período$`, suite.adminFunnelOriginInCohort)
	sc.Step(`^el período no vuelve a filtrar las fechas de solicitudes, propuestas, órdenes, reportes, pagos ni reseñas$`, suite.adminFunnelDownstreamNotPeriodFiltered)
	sc.Step(`^las etapas anteriores siguen contándose cuando la unidad avanza a otra etapa$`, suite.adminFunnelStagesMonotonic)
	sc.Step(`^el recorrido manual informa cero unidades y conversiones nulas por tener denominador inicial cero$`, suite.adminFunnelManualConversionsNull)
	sc.Step(`^la demora solicitud→primera propuesta tiene dos observaciones y media (\d+)\.(\d+) segundos, redondeada a dos decimales por mitades hacia arriba$`, suite.adminFunnelDelayTwoParts)
	sc.Step(`^la demora propuesta→contratación confirmada tiene tres observaciones de las órdenes "([^"]*)", "([^"]*)" y "([^"]*)" y media (\d+)\.(\d+) segundos$`, suite.adminFunnelDelayThreeParts)
	sc.Step(`^la demora contratación→finalización tiene dos observaciones y media (\d+)\.(\d+) segundos usando los reportes de "([^"]*)" y "([^"]*)"$`, suite.adminFunnelHiringToCompletionParts)
	sc.Step(`^la demora finalización→pago tiene una observación y media (\d+)\.(\d+) segundos usando el pago de "([^"]*)"$`, suite.adminFunnelCompletionToPaymentParts)
	sc.Step(`^las demoras usan solo hitos persistidos y cronológicamente válidos de la misma orden$`, suite.adminFunnelDelaySamplesValid)
	sc.Step(`^los reportes y pagos posteriores al final del período se incluyen por pertenecer a la cohorte seleccionada por la fecha de sus solicitudes$`, suite.adminFunnelDownstreamNotPeriodFiltered)
	sc.Step(`^la solicitud y la orden sin reporte no aportan como duración el tiempo que llevan esperando$`, suite.adminFunnelPendingNotDuration)
	sc.Step(`^la demora solicitud→primera propuesta tiene una observación de "([^"]*)" y media (\d+\.\d+) segundos$`, suite.adminFunnelRequestProposalByExpected)
	sc.Step(`^no se sustituye la primera propuesta inválida por una posterior$`, suite.adminFunnelNoSubstitution)
	sc.Step(`^la demora propuesta→contratación confirmada conserva dos observaciones válidas y una media de (\d+)\.(\d+) segundos$`, suite.adminFunnelProposalHiringByExpected)
	sc.Step(`^el intervalo contratación→finalización inválido de la primera orden se excluye sin descartar la observación válida de "([^"]*)" de (\d+\.\d+) segundos$`, suite.adminFunnelHiringCompletionByExpected)
	sc.Step(`^la demora finalización→pago conserva una observación válida de "([^"]*)" y media (\d+\.\d+) segundos$`, suite.adminFunnelCompletionPaymentByExpected)
	sc.Step(`^cada media informa su cantidad de observaciones válidas$`, suite.adminFunnelDelaySampleCounts)
	sc.Step(`^solicitud→primera propuesta informa una observación válida y una media de cero segundos$`, suite.adminFunnelZeroDelay)
	sc.Step(`^las otras medias informan cero observaciones y valor nulo$`, suite.adminFunnelEmptyDelays)
	sc.Step(`^el período efectivo tiene exactamente 365 días y su final coincide con el instante de observación$`, suite.adminFunnelMaxPeriod)
	sc.Step(`^el error no se presenta como una respuesta vacía ni como conteos cero$`, suite.adminFunnelErrorNotZero)
	sc.Step(`^no se entrega un agregado parcial$`, suite.adminFunnelErrorNotZero)
	sc.Step(`^la respuesta contiene solo agregados y metadatos de cálculo, sin filas individuales, datos personales, conversaciones ni payloads financieros$`, suite.adminFunnelAggregateOnly)
	sc.Step(`^no se registra auditoría administrativa persistente ni se modifica el estado de negocio$`, suite.adminFunnelNoSideEffects)
	sc.Step(`^la consulta no depende de servicios externos$`, suite.adminFunnelNoExternalDependency)
	sc.Step(`^el recorrido IA cuenta (\d+) evaluaciones profesionales y selecciona por el rubro persistido en la evaluación de origen$`, suite.adminFunnelCategoryOriginCount)
	sc.Step(`^el recorrido manual cuenta (\d+) y selecciona por el rubro actual del prestador destinatario, sin presentarlo como histórico$`, suite.adminFunnelCategoryManualCount)
	// Common background fixtures are deliberately separate Given steps; they resolve through the existing repositories.
	sc.Step(`^que no existen evaluaciones profesionales ni solicitudes manuales en el período predeterminado$`, suite.adminFunnelNoOrigins)
	sc.Step(`^que existen evaluaciones profesionales creadas en "([^"]*)" y "([^"]*)"$`, suite.adminFunnelHistoricalAssessments)
	sc.Step(`^que existe una solicitud manual sin evaluación de origen creada en "([^"]*)"$`, suite.adminFunnelHistoricalManualRequest)
	sc.Step(`^que existe actividad asociada al rubro (\d+)$`, suite.adminFunnelUnknownActivity)
	sc.Step(`^que no hay actividad dentro del período$`, suite.adminFunnelNoActivity365Days)
	sc.Step(`^que estoy autenticado con un JWT válido de administrador sin el permiso "([^"]*)"$`, suite.adminFunnelAuthWithoutPermission)
	sc.Step(`^que el JWT incluye otro permiso administrativo$`, suite.adminFunnelAddOtherPermission)
	sc.Step(`^que existe actividad válida dentro del período$`, suite.adminFunnelValidActivity)
	sc.Step(`^que existen evaluaciones, solicitudes, propuestas y órdenes con actividad en el período$`, suite.adminFunnelCompleteActivity)
	sc.Step(`^que no existe un evento administrativo de auditoría para la consulta$`, suite.adminFunnelNoAudit)
	sc.Step(`^que existen prestadores registrados en los rubros indicados:$`, suite.adminFunnelProvidersByCategory)
	sc.Step(`^que existe una evaluación profesional de "([^"]*)" del rubro "([^"]*)" en el período$`, suite.adminFunnelAssessmentByConsumerCategory)
	sc.Step(`^que la conversación asistida de "([^"]*)" tiene las siguientes evaluaciones persistidas en el período:$`, suite.adminFunnelAssessmentTable)
	sc.Step(`^que "([^"]*)" es la evaluación actual de la conversación$`, suite.adminFunnelCurrentAssessment)
	sc.Step(`^que la solicitud "([^"]*)" se originó explícitamente en "([^"]*)" aunque "([^"]*)" sea la evaluación actual$`, suite.adminFunnelLinkAssessment)
	sc.Step(`^que existe una evaluación persistida "([^"]*)" con resultado "([^"]*)" en el período$`, suite.adminFunnelSimpleAssessment)
	sc.Step(`^que una respuesta posterior del asistente tiene acción "unchanged" y reutiliza el ID de "([^"]*)" sin persistir una nueva versión$`, suite.adminFunnelUnchangedAssessment)
	sc.Step(`^que existen evaluaciones del período con resultados "([^"]*)" y "([^"]*)"$`, suite.adminFunnelExcludedAssessments)
	sc.Step(`^que existe una solicitud manual de "([^"]*)" con "([^"]*)" cuyo source_assessment_id está ausente$`, suite.adminFunnelManualRequest)
	sc.Step(`^que existe otra solicitud de "([^"]*)" con "([^"]*)" originada en una evaluación profesional persistida$`, suite.adminFunnelLinkedRequest)

	sc.Step(`^el recorrido IA cuenta una evaluación profesional y el recorrido manual una solicitud$`, suite.adminFunnelOneEach)
	sc.Step(`^que existe una solicitud manual de "([^"]*)" dirigida a un prestador cuyo rubro actual es "([^"]*)" en el período$`, suite.adminFunnelManualRequestByCategory)
	sc.Step(`^que no existe actividad asociada al rubro (\d+)$`, suite.adminFunnelUnknownActivity)
	sc.Step(`^que existen tres evaluaciones profesionales del período: "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.adminFunnelThreeAssessments)
	sc.Step(`^que la evaluación profesional "([^"]*)" fue creada exactamente al inicio del período$`, suite.adminFunnelAssessmentAtPeriodStart)
	sc.Step(`^que su solicitud, propuesta, orden aceptada, reporte de finalización, pago completo y reseña asociada a esa misma orden pagada ocurrieron después del final del período y antes del instante de observación$`, suite.adminFunnelPostPeriodBranch)
	sc.Step(`^que existen tres evaluaciones profesionales en el período y ninguna tiene una solicitud$`, suite.adminFunnelThreeUnrequestedAssessments)
	sc.Step(`^que no existen solicitudes manuales en el período$`, suite.adminFunnelNoManualRequests)

	sc.Step(`^que "([^"]*)" se vinculó a dos solicitudes distintas para prestadores distintos y ambas tienen propuestas en sus propias conversaciones$`, suite.adminFunnelAIAssessmentHasTwoRequests)
	sc.Step(`^que la primera solicitud de "([^"]*)" tiene dos propuestas, cada una con su propia orden, y que la segunda tiene una propuesta y su orden$`, suite.adminFunnelAIBranchHasThreeProposalsAndOrders)
	sc.Step(`^que "([^"]*)" tiene una solicitud, una propuesta y una orden; "([^"]*)" no tiene solicitud$`, suite.adminFunnelOneAssessmentBranch)
	sc.Step(`^que una orden de "([^"]*)" tiene reporte persistido, está pagada y tiene una reseña; la orden de "([^"]*)" tiene reporte pero aún no tiene paid_on$`, suite.adminFunnelAIBranchMilestones)
	sc.Step(`^que una orden de "([^"]*)" tiene varios intentos de pago y transacciones, incluidos intentos no aprobados$`, suite.adminFunnelPaymentAttempts)
	sc.Step(`^el recorrido IA informa estos conteos y conversiones consecutivas por cohorte, sin multiplicar una unidad por sus relaciones 1:N:$`, suite.adminFunnelAIStageTable)
	sc.Step(`^la conversión global desde el origen hasta "([^"]*)" es (\d+\.\d+) por ciento aunque pago completo y reseña tengan otros conteos$`, suite.adminFunnelGlobalAI)
	sc.Step(`^la conversión global desde evaluación profesional hasta finalización informada es (\d+\.\d+) por ciento$`, suite.adminFunnelGlobalAIExpected)
	sc.Step(`^que existen tres solicitudes manuales del período: "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.adminFunnelThreeManualRequests)
	sc.Step(`^que "([^"]*)" tiene dos propuestas en su conversación de trabajo, "([^"]*)" una propuesta y "([^"]*)" una propuesta cuyo intento de pago de seña está en checkout_ready y no tiene orden$`, suite.adminFunnelManualProposals)
	sc.Step(`^que las dos propuestas de "([^"]*)" y la de "([^"]*)" tienen órdenes aceptadas distintas "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.adminFunnelManualOrders)
	sc.Step(`^"([^"]*)" y "([^"]*)" tienen reporte de finalización; solo "([^"]*)" tiene "paid_on" y una reseña asociada, mientras "([^"]*)" no tiene reporte$`, suite.adminFunnelManualMilestones)
	sc.Step(`^el checkout_ready de la propuesta de "([^"]*)" no cuenta como contratación confirmada sin una orden vinculada$`, suite.adminFunnelNoOrderForCheckout)
	sc.Step(`^que una evaluación profesional "([^"]*)" originó solicitudes para dos prestadores en conversaciones de trabajo distintas$`, suite.adminFunnelTwoBranches)
	sc.Step(`^que la orden "([^"]*)" de la primera rama tiene un reporte persistido y no tiene "paid_on"$`, suite.adminFunnelBranchReported)
	sc.Step(`^que la orden "([^"]*)" de la segunda rama tiene "paid_on" persistido pero no tiene reporte de finalización$`, suite.adminFunnelBranchPaidWithoutReport)
	sc.Step(`^que estos datos incompatibles con el flujo normal son un fixture histórico persistido explícitamente, no una secuencia creada por el flujo de aceptación$`, suite.adminFunnelHistoricalAnomaly)
	sc.Step(`^que la solicitud "([^"]*)" fue creada a las "([^"]*)" y sus propuestas persistidas se crearon a las "([^"]*)" y "([^"]*)"$`, suite.adminFunnelAnomalousRequest)
	sc.Step(`^que la orden histórica de la primera propuesta fue aceptada a las "([^"]*)" y tiene un reporte anterior a su aceptación$`, suite.adminFunnelAnomalousReport)
	sc.Step(`^que la orden histórica de la primera propuesta no tiene "paid_on"$`, suite.adminFunnelNoPaymentForFirstAnomaly)
	sc.Step(`^que la solicitud "([^"]*)" fue creada a las "([^"]*)", su propuesta a las "([^"]*)", su orden aceptada a las "([^"]*)", programada para "([^"]*)", reportada a las "([^"]*)" y pagada a las "([^"]*)"$`, suite.adminFunnelValidSecondBranch)
	sc.Step(`^que las fechas anómalas de "([^"]*)" provienen de un fixture histórico persistido explícitamente, no del flujo normal de contratación$`, suite.adminFunnelChronologyAnomaly)
	sc.Step(`^que la única solicitud manual del período fue creada a las "([^"]*)" y su primera propuesta persistida se creó en ese mismo instante, sin orden$`, suite.adminFunnelZeroIntervalFixture)
	sc.Step(`^que existen estas solicitudes manuales del período:$`, suite.adminFunnelRequestsTable)
	sc.Step(`^que las propuestas están vinculadas a sus propias solicitudes y tienen estas fechas:$`, suite.adminFunnelProposalsTable)
	sc.Step(`^las órdenes aceptadas, reportes y pagos están vinculados a sus propias propuestas:$`, suite.adminFunnelOrdersTable)
	sc.Step(`^"([^"]*)" no tiene propuesta y "([^"]*)" no tiene reporte$`, suite.adminFunnelMissingProposalAndReport)
	sc.Step(`^las dos propuestas de "([^"]*)" no multiplican el conteo de solicitudes ni de etapas$`, suite.adminFunnelManualRequestNotMultiplied)

}

func (suite *testSuite) adminFunnelSetClock(value string) error {
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return fmt.Errorf("parsing funnel test clock: %w", err)
	}
	suite.clock.SetTime(instant)
	return nil
}

func (suite *testSuite) adminFunnelRequest(query url.Values) error {
	suite.adminFunnel.query = query
	suite.adminFunnel.statusBefore = suite.lastStatus
	suite.adminRequest.omitBearer = false
	suite.adminRequest.invalidBearer = false
	if suite.invalidSession {
		suite.adminRequest.invalidBearer = true
	} else if suite.currentAuth0ID == "" {
		suite.adminRequest.omitBearer = true
	}
	if err := suite.sendAdminGet(adminFunnelPath, query, ""); err != nil {
		return err
	}
	suite.adminFunnel.response = adminFunnelResponse{}
	var envelope map[string]json.RawMessage
	suite.adminFunnel.responseValid = json.Unmarshal(suite.lastBody, &envelope) == nil && envelope["period"] != nil && envelope["cohorts"] != nil && validateFunnelResponseShape(suite.lastBody) == nil
	if suite.adminFunnel.responseValid {
		if err := json.Unmarshal(suite.lastBody, &suite.adminFunnel.response); err != nil {
			return fmt.Errorf("decoding funnel metrics response: %w", err)
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelQueryDefault() error {
	return suite.adminFunnelRequest(url.Values{})
}
func (suite *testSuite) adminFunnelQueryPeriod(from, to string) error {
	return suite.adminFunnelRequest(url.Values{"from": {from}, "to": {to}})
}
func (suite *testSuite) adminFunnelQueryCategory(name string) error {
	id, ok := suite.categoryIDsByName[name]
	if !ok {
		return fmt.Errorf("category %q has no resolved fixture ID", name)
	}
	return suite.adminFunnelRequest(url.Values{"category_id": {strconv.Itoa(id)}})
}
func (suite *testSuite) adminFunnelQueryCategoryID(id string) error {
	return suite.adminFunnelRequest(url.Values{"category_id": {id}})
}
func (suite *testSuite) adminFunnelQueryChoice(choice string) error {
	if choice == "sin filtros" {
		return suite.adminFunnelQueryDefault()
	}
	name := strings.Trim(strings.TrimPrefix(choice, `para el rubro `), `"`)
	return suite.adminFunnelQueryCategory(name)
}
func (suite *testSuite) adminFunnelQueryRaw(raw string) error {
	query, err := url.ParseQuery(raw)
	if err != nil {
		return fmt.Errorf("parsing funnel query fixture: %w", err)
	}
	return suite.adminFunnelRequest(query)
}
func (suite *testSuite) adminFunnelFailRead() error {
	suite.adminFunnelCapture.failRead = true
	return nil
}
func (suite *testSuite) adminFunnelStatus(expected string) error {
	code, err := strconv.Atoi(expected)
	if err != nil {
		return err
	}
	if suite.lastStatus != code {
		return fmt.Errorf("expected funnel HTTP status %d, got %d: %s", code, suite.lastStatus, suite.lastBody)
	}
	if code == http.StatusUnauthorized {
		got := suite.adminRequest.sentHeaders.Get("Authorization")
		if suite.invalidSession {
			if got != "Bearer invalid-token" {
				return fmt.Errorf("invalid-session request sent Authorization %q, expected invalid Bearer token", got)
			}
		} else if suite.currentAuth0ID == "" && got != "" {
			return fmt.Errorf("unauthenticated request unexpectedly sent Authorization %q", got)
		}
	}
	if code == http.StatusOK && !suite.adminFunnel.responseValid {
		return fmt.Errorf("successful funnel response did not contain a complete valid aggregate")
	}
	if code != http.StatusOK && len(suite.lastBody) > 0 && json.Valid(suite.lastBody) {
		var payload map[string]json.RawMessage
		if json.Unmarshal(suite.lastBody, &payload) == nil && payload["cohorts"] != nil {
			return fmt.Errorf("error response unexpectedly contains metrics")
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelCacheIsPrivate() error {
	if got := suite.adminRequest.headers.Get("Cache-Control"); got != "private, no-store" {
		return fmt.Errorf("expected Cache-Control private, no-store, got %q", got)
	}
	return nil
}
func (suite *testSuite) adminFunnelDefaultPeriod(observed string) error {
	instant, err := time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return err
	}
	return suite.checkFunnelPeriod(instant.Add(-30*24*time.Hour), instant)
}
func (suite *testSuite) adminFunnelMetadata(zone, observed string) error {
	instant, err := time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return err
	}
	r := suite.adminFunnel.response
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("funnel response is not valid JSON")
	}
	if r.TimeZone != zone || !r.ObservedAt.Equal(instant) || r.Rounding != "half_up" || r.DecimalPlaces != 2 {
		return fmt.Errorf("unexpected funnel metadata: timezone=%q observed_at=%s rounding=%q decimal_places=%d", r.TimeZone, r.ObservedAt, r.Rounding, r.DecimalPlaces)
	}
	return nil
}
func (suite *testSuite) checkFunnelPeriod(from, to time.Time) error {
	r := suite.adminFunnel.response
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("funnel response is not valid JSON")
	}
	if !r.Period.From.Equal(from) || !r.Period.To.Equal(to) {
		return fmt.Errorf("unexpected effective period [%s,%s), want [%s,%s)", r.Period.From, r.Period.To, from, to)
	}
	return nil
}
func (suite *testSuite) adminFunnelPeriodIsHalfOpen(start string) error {
	from, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return err
	}
	if !suite.adminFunnel.response.Period.From.Equal(from) {
		return fmt.Errorf("period start=%s, expected inclusive origin boundary %s", suite.adminFunnel.response.Period.From, from)
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1")
}
func (suite *testSuite) adminFunnelMaxPeriod() error {
	if err := suite.checkFunnelPeriod(time.Date(2025, 9, 30, 15, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)); err != nil {
		return err
	}
	if !suite.adminFunnel.response.ObservedAt.Equal(suite.adminFunnel.response.Period.To) {
		return fmt.Errorf("maximum period end does not match observed_at")
	}
	return suite.adminFunnelEmptyCohorts()
}
func (suite *testSuite) adminFunnelEmptyCohorts() error {
	for name, cohort := range map[string]adminFunnelCohort{"ai": suite.adminFunnel.response.Cohorts.AI, "manual": suite.adminFunnel.response.Cohorts.Manual} {
		for _, stage := range cohort.Stages {
			if stage.Count != 0 {
				return fmt.Errorf("%s stage %s unexpectedly counts %d", name, stage.Stage, stage.Count)
			}
			if !isJSONNull(stage.Conversion) {
				return fmt.Errorf("%s stage %s conversion should be null without denominator", name, stage.Stage)
			}
		}
		if !isJSONNull(cohort.GlobalConversion) {
			return fmt.Errorf("%s global conversion should be null for empty origin cohort", name)
		}
		for key, delay := range cohort.Delays {
			if delay.Observations != 0 || !isJSONNull(delay.MeanSeconds) {
				return fmt.Errorf("%s delay %s should have no observations and null mean", name, key)
			}
		}
	}
	return nil
}
func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
func (suite *testSuite) adminFunnelNoActivityThreshold() error {
	return suite.adminFunnelEmptyCohorts()
}
func (suite *testSuite) adminFunnelOriginCounts(ai, manual string) error {
	if err := suite.requireFunnelCount(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", ai); err != nil {
		return err
	}
	return suite.requireFunnelCount(suite.adminFunnel.response.Cohorts.Manual, "request", manual)
}
func (suite *testSuite) adminFunnelCountNumber(cohort adminFunnelCohort, stage, expected string) error {
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("funnel metrics are unavailable or malformed")
	}
	for _, metric := range cohort.Stages {
		if metric.Stage == stage {
			if strconv.FormatInt(metric.Count, 10) != expected {
				return fmt.Errorf("%s count=%d, expected %s", stage, metric.Count, expected)
			}
			return nil
		}
	}
	return fmt.Errorf("stage %q is absent", stage)
}
func (suite *testSuite) requireFunnelCount(cohort adminFunnelCohort, stage, expected string) error {
	if expected == "una" {
		expected = "1"
	}
	if expected == "cero" {
		expected = "0"
	}
	return suite.adminFunnelCountNumber(cohort, stage, expected)
}
func (suite *testSuite) adminFunnelCategoryAI(expected string) error {
	if suite.adminFunnel.response.Cohorts.AI.CategorySource != "assessment.problem_category_id" {
		return fmt.Errorf("AI category source=%q", suite.adminFunnel.response.Cohorts.AI.CategorySource)
	}
	return suite.requireFunnelCount(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", expected)
}
func (suite *testSuite) adminFunnelCategoryManual(expected string) error {
	if suite.adminFunnel.response.Cohorts.Manual.CategorySource != "provider.current_category_id" {
		return fmt.Errorf("manual category source=%q", suite.adminFunnel.response.Cohorts.Manual.CategorySource)
	}
	return suite.requireFunnelCount(suite.adminFunnel.response.Cohorts.Manual, "request", expected)
}
func (suite *testSuite) adminFunnelAssessmentCounts(origins, requested string) error {
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", origins); err != nil {
		return err
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "request", requested)
}
func (suite *testSuite) adminFunnelStageTable(name string, table *godog.Table) error {
	cohort := suite.adminFunnel.response.Cohorts.AI
	if name == "manual" {
		cohort = suite.adminFunnel.response.Cohorts.Manual
	}
	if err := requireTableHeaders(table, "etapa", "unidades", "conversión"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		stage, count := row.Cells[0].Value, row.Cells[1].Value
		if err := suite.adminFunnelCountNumber(cohort, funnelStageForSpanish(stage), count); err != nil {
			return err
		}
		metric := funnelMetric(cohort, funnelStageForSpanish(stage))
		if metric == nil {
			return fmt.Errorf("missing conversion stage %s", stage)
		}
		if row.Cells[2].Value == "" {
			if !isJSONNull(metric.Conversion) {
				return fmt.Errorf("%s conversion should be null for the first stage", stage)
			}
		} else {
			var got string
			if err := json.Unmarshal(metric.Conversion, &got); err != nil {
				got = strings.Trim(string(metric.Conversion), `"`)
			}
			if got != strings.TrimSuffix(row.Cells[2].Value, " %") {
				return fmt.Errorf("%s conversion=%s, expected %s", stage, got, row.Cells[2].Value)
			}
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelAIStageTable(table *godog.Table) error {
	return suite.adminFunnelStageTable("IA", table)
}
func funnelStageForSpanish(stage string) string {
	switch stage {
	case "evaluación profesional":
		return "professional_assessment"
	case "solicitud", "con solicitud":
		return "request"
	case "con propuesta":
		return "proposal"
	case "con contratación confirmada":
		return "confirmed_hiring"
	case "con finalización informada":
		return "reported_completion"
	case "con pago completo":
		return "full_payment"
	case "con reseña":
		return "review"
	}
	return stage
}
func funnelMetric(cohort adminFunnelCohort, name string) *struct {
	Stage      string          `json:"stage"`
	Count      int64           `json:"count"`
	Conversion json.RawMessage `json:"conversion_percentage"`
} {
	for i := range cohort.Stages {
		if cohort.Stages[i].Stage == name {
			return &cohort.Stages[i]
		}
	}
	return nil
}

func (suite *testSuite) adminFunnelAssessmentVersions() error {
	expected := 2
	if raw := suite.adminFunnel.query.Get("category_id"); raw != "" {
		plumbing := suite.categoryIDsByName["Plomería"]
		electricity := suite.categoryIDsByName["Electricidad"]
		if raw == strconv.Itoa(plumbing) || raw == strconv.Itoa(electricity) {
			expected = 1
		} else {
			expected = 0
		}
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", strconv.Itoa(expected))
}
func (suite *testSuite) adminFunnelOneAssessmentVersion() error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1")
}
func (suite *testSuite) adminFunnelOneAssessment(alias string) error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1")
}
func (suite *testSuite) adminFunnelOnlyProfessionalOutcomes() error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1")
}
func (suite *testSuite) adminFunnelManualOnlyUnlinked() error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "request", "1")
}
func (suite *testSuite) adminFunnelAIOnlyLinked() error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "request", "1")
}
func (suite *testSuite) adminFunnelIndependentCohorts() error {
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1"); err != nil {
		return err
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "request", "1")
}
func (suite *testSuite) adminFunnelDistinctStages() error { return suite.adminFunnelStagesMonotonic() }
func (suite *testSuite) adminFunnelManualRequestNotMultiplied(alias string) error {
	if suite.adminFunnel.requestAliases[alias] == 0 {
		return fmt.Errorf("manual request %q is not persisted", alias)
	}
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "request", "3"); err != nil {
		return err
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "proposal", "3")
}
func (suite *testSuite) adminFunnelGlobalManual(expected string) error {
	return suite.adminFunnelComparePercent(suite.adminFunnel.response.Cohorts.Manual.GlobalConversion, expected)
}
func (suite *testSuite) adminFunnelGlobalAI(_, expected string) error {
	return suite.adminFunnelComparePercent(suite.adminFunnel.response.Cohorts.AI.GlobalConversion, expected)
}
func (suite *testSuite) adminFunnelGlobalAIExpected(expected string) error {
	return suite.adminFunnelComparePercent(suite.adminFunnel.response.Cohorts.AI.GlobalConversion, expected)
}
func (suite *testSuite) adminFunnelGlobalAIConversion(expected string) error {
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "3"); err != nil {
		return err
	}
	return suite.adminFunnelComparePercent(suite.adminFunnel.response.Cohorts.AI.GlobalConversion, expected)
}
func (suite *testSuite) adminFunnelConversion(expected string) error {
	metric := funnelMetric(suite.adminFunnel.response.Cohorts.AI, "request")
	if metric == nil {
		return fmt.Errorf("AI request stage is missing")
	}
	return suite.adminFunnelComparePercent(metric.Conversion, expected)
}
func (suite *testSuite) adminFunnelComparePercent(value json.RawMessage, expected string) error {
	var got string
	if len(value) == 0 || string(value) == "null" {
		got = "null"
	} else if err := json.Unmarshal(value, &got); err != nil {
		got = string(value)
	}
	want := strings.TrimSpace(expected)
	if got != want {
		return fmt.Errorf("percentage=%s, expected %s", got, want)
	}
	return nil
}
func (suite *testSuite) adminFunnelLaterConversionsNull() error {
	for _, cohort := range []adminFunnelCohort{suite.adminFunnel.response.Cohorts.AI, suite.adminFunnel.response.Cohorts.Manual} {
		for i, stage := range cohort.Stages {
			if i == 0 {
				continue
			}
			previous := cohort.Stages[i-1].Count
			if previous == 0 && !isJSONNull(stage.Conversion) {
				return fmt.Errorf("stage %s conversion should be null with zero denominator", stage.Stage)
			}
			if previous > 0 && isJSONNull(stage.Conversion) {
				return fmt.Errorf("stage %s conversion should be numeric with denominator %d", stage.Stage, previous)
			}
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelManualConversionsNull() error {
	for _, stage := range suite.adminFunnel.response.Cohorts.Manual.Stages {
		if stage.Count != 0 {
			return fmt.Errorf("manual stage %s unexpectedly has count %d", stage.Stage, stage.Count)
		}
		if !isJSONNull(stage.Conversion) {
			return fmt.Errorf("manual stage %s conversion should be null", stage.Stage)
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelReportedBranch(alias string) error {
	if suite.adminFunnel.orderAliases[alias] == 0 {
		return fmt.Errorf("reported branch order %q was not persisted", alias)
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "reported_completion", "1")
}
func (suite *testSuite) adminFunnelNoPaidBranch() error {
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "full_payment", "0")
}
func (suite *testSuite) adminFunnelOriginInCohort(alias string) error {
	if suite.adminFunnel.assessmentAliases[alias] == 0 {
		return fmt.Errorf("assessment %q was not persisted", alias)
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", "1")
}
func (suite *testSuite) adminFunnelNoCrossBranch(_ string, _ string) error {
	if err := suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "reported_completion", "1"); err != nil {
		return err
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "full_payment", "0")
}
func (suite *testSuite) adminFunnelDownstreamNotPeriodFiltered() error {
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("funnel metrics are unavailable")
	}
	for _, cohort := range []adminFunnelCohort{suite.adminFunnel.response.Cohorts.AI, suite.adminFunnel.response.Cohorts.Manual} {
		if len(cohort.Stages) > 1 && cohort.Stages[0].Count > 0 && cohort.Stages[1].Count == 0 {
			return fmt.Errorf("downstream activity for an origin was filtered out")
		}
	}
	return suite.adminFunnelStagesMonotonic()
}
func (suite *testSuite) adminFunnelStagesMonotonic() error {
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("funnel metrics are unavailable")
	}
	for _, cohort := range []adminFunnelCohort{suite.adminFunnel.response.Cohorts.AI, suite.adminFunnel.response.Cohorts.Manual} {
		for i := 1; i < len(cohort.Stages); i++ {
			if cohort.Stages[i].Count > cohort.Stages[i-1].Count {
				return fmt.Errorf("funnel stages are not monotonic: %s=%d after %s=%d", cohort.Stages[i].Stage, cohort.Stages[i].Count, cohort.Stages[i-1].Stage, cohort.Stages[i-1].Count)
			}
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelDelayTwoParts(whole, fraction string) error {
	return suite.adminFunnelCheckDelay("request_to_first_proposal", "2", whole+"."+fraction)
}
func (suite *testSuite) adminFunnelDelayThreeParts(order1, order2, order3, whole, fraction string) error {
	for _, alias := range []string{order1, order2, order3} {
		if suite.adminFunnel.orderAliases[alias] == 0 {
			return fmt.Errorf("delay order %q is not persisted", alias)
		}
	}
	return suite.adminFunnelCheckDelay("proposal_to_confirmed_hiring", "3", whole+"."+fraction)
}
func (suite *testSuite) adminFunnelHiringToCompletionParts(whole, fraction, order1, order2 string) error {
	for _, alias := range []string{order1, order2} {
		if suite.adminFunnel.orderAliases[alias] == 0 {
			return fmt.Errorf("completion-delay order %q is not persisted", alias)
		}
	}
	return suite.adminFunnelCheckDelay("confirmed_hiring_to_reported_completion", "2", whole+"."+fraction)
}
func (suite *testSuite) adminFunnelCompletionToPaymentParts(whole, fraction, alias string) error {
	if suite.adminFunnel.orderAliases[alias] == 0 {
		return fmt.Errorf("payment-delay order %q is not persisted", alias)
	}
	return suite.adminFunnelCheckDelay("reported_completion_to_full_payment", "1", whole+"."+fraction)
}
func (suite *testSuite) adminFunnelRequestProposalByExpected(_, mean string) error {
	return suite.adminFunnelCheckDelay("request_to_first_proposal", "1", mean)
}
func (suite *testSuite) adminFunnelProposalHiringByExpected(whole, fraction string) error {
	return suite.adminFunnelCheckDelay("proposal_to_confirmed_hiring", "2", whole+"."+fraction)
}
func (suite *testSuite) adminFunnelHiringCompletionByExpected(_, mean string) error {
	return suite.adminFunnelCheckDelay("confirmed_hiring_to_reported_completion", "1", mean)
}
func (suite *testSuite) adminFunnelCompletionPaymentByExpected(_, mean string) error {
	return suite.adminFunnelCheckDelay("reported_completion_to_full_payment", "1", mean)
}
func (suite *testSuite) adminFunnelCheckDelay(name, observations, mean string) error {
	delay, ok := suite.adminFunnel.response.Cohorts.Manual.Delays[name]
	if !ok {
		return fmt.Errorf("manual delay %q is absent", name)
	}
	if strconv.FormatInt(delay.Observations, 10) != observations {
		return fmt.Errorf("manual delay %s has %d observations, expected %s", name, delay.Observations, observations)
	}
	return suite.adminFunnelCheckDelayValue(delay, mean)
}
func (suite *testSuite) adminFunnelCheckDelayValue(delay struct {
	Observations int64           `json:"observations"`
	MeanSeconds  json.RawMessage `json:"mean_seconds"`
}, mean string) error {
	if delay.Observations == 0 {
		return fmt.Errorf("delay has no observations")
	}
	return suite.adminFunnelComparePercent(delay.MeanSeconds, mean)
}
func (suite *testSuite) adminFunnelNoSubstitution() error {
	if err := suite.adminFunnelCheckDelay("request_to_first_proposal", "1", "60.00"); err != nil {
		return err
	}
	return suite.adminFunnelRequireDelayCounts(map[string]int64{
		"request_to_first_proposal": 1, "proposal_to_confirmed_hiring": 2,
		"confirmed_hiring_to_reported_completion": 1, "reported_completion_to_full_payment": 1,
	})
}
func (suite *testSuite) adminFunnelDelaySamplesValid() error {
	for _, cohort := range []adminFunnelCohort{suite.adminFunnel.response.Cohorts.AI, suite.adminFunnel.response.Cohorts.Manual} {
		for name, delay := range cohort.Delays {
			if delay.Observations < 0 {
				return fmt.Errorf("delay %s has negative observations", name)
			}
			if delay.Observations == 0 && !isJSONNull(delay.MeanSeconds) {
				return fmt.Errorf("delay %s has a mean without observations", name)
			}
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelPendingNotDuration() error {
	if err := suite.adminFunnelCheckDelay("request_to_first_proposal", "2", "33.34"); err != nil {
		return err
	}
	if err := suite.adminFunnelCheckDelay("confirmed_hiring_to_reported_completion", "2", "172820.00"); err != nil {
		return err
	}
	if err := suite.adminFunnelCheckDelay("reported_completion_to_full_payment", "1", "40.00"); err != nil {
		return err
	}
	return suite.adminFunnelRequireDelayCounts(map[string]int64{
		"request_to_first_proposal": 2, "proposal_to_confirmed_hiring": 3,
		"confirmed_hiring_to_reported_completion": 2, "reported_completion_to_full_payment": 1,
	})
}
func (suite *testSuite) adminFunnelDelaySampleCounts() error {
	return suite.adminFunnelRequireDelayCounts(map[string]int64{
		"request_to_first_proposal": 1, "proposal_to_confirmed_hiring": 2,
		"confirmed_hiring_to_reported_completion": 1, "reported_completion_to_full_payment": 1,
	})
}
func (suite *testSuite) adminFunnelRequireDelayCounts(expected map[string]int64) error {
	for name, want := range expected {
		delay, ok := suite.adminFunnel.response.Cohorts.Manual.Delays[name]
		if !ok || delay.Observations != want {
			return fmt.Errorf("manual delay %s observations=%d present=%t, want %d", name, delay.Observations, ok, want)
		}
		if want == 0 && !isJSONNull(delay.MeanSeconds) {
			return fmt.Errorf("manual delay %s has mean %s without observations", name, delay.MeanSeconds)
		}
		if want > 0 && !isJSONNumber(delay.MeanSeconds) {
			return fmt.Errorf("manual delay %s mean %s is not a JSON number", name, delay.MeanSeconds)
		}
	}
	for name, delay := range suite.adminFunnel.response.Cohorts.AI.Delays {
		if delay.Observations != 0 || !isJSONNull(delay.MeanSeconds) {
			return fmt.Errorf("AI delay %s should have zero observations and null mean, got %d/%s", name, delay.Observations, delay.MeanSeconds)
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelZeroDelay() error {
	delay := suite.adminFunnel.response.Cohorts.Manual.Delays["request_to_first_proposal"]
	if delay.Observations != 1 || string(delay.MeanSeconds) != "0.00" {
		return fmt.Errorf("expected one zero-second request/proposal observation, got n=%d mean=%s", delay.Observations, delay.MeanSeconds)
	}
	return nil
}
func (suite *testSuite) adminFunnelEmptyDelays() error {
	for key, d := range suite.adminFunnel.response.Cohorts.Manual.Delays {
		if key == "request_to_first_proposal" {
			continue
		}
		if d.Observations != 0 || !isJSONNull(d.MeanSeconds) {
			return fmt.Errorf("delay %s should be null with no samples", key)
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelUnknownCategory() error { return suite.adminFunnelStatus("200") }
func (suite *testSuite) adminFunnelAggregateOnly() error {
	if !suite.adminFunnel.responseValid {
		return fmt.Errorf("aggregate response is not valid JSON")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &payload); err != nil {
		return err
	}
	if err := requireJSONKeys(payload, "period", "timezone", "observed_at", "category_id", "rounding", "decimal_places", "cohorts"); err != nil {
		return err
	}
	var period, cohorts map[string]json.RawMessage
	if err := json.Unmarshal(payload["period"], &period); err != nil {
		return err
	}
	if err := requireJSONKeys(period, "from", "to"); err != nil {
		return err
	}
	if err := json.Unmarshal(payload["cohorts"], &cohorts); err != nil {
		return err
	}
	if err := requireJSONKeys(cohorts, "ai", "manual"); err != nil {
		return err
	}
	for name, raw := range cohorts {
		var cohort map[string]json.RawMessage
		if err := json.Unmarshal(raw, &cohort); err != nil {
			return fmt.Errorf("%s cohort: %w", name, err)
		}
		if err := requireJSONKeys(cohort, "category_source", "stages", "global_completion_conversion_percentage", "delays"); err != nil {
			return err
		}
		var stages []map[string]json.RawMessage
		if err := json.Unmarshal(cohort["stages"], &stages); err != nil {
			return err
		}
		for _, stage := range stages {
			if err := requireJSONKeys(stage, "stage", "count", "conversion_percentage"); err != nil {
				return err
			}
		}
		if !isJSONNumberOrNull(cohort["global_completion_conversion_percentage"]) {
			return fmt.Errorf("%s global completion conversion must be a JSON number or null", name)
		}
		var delays map[string]json.RawMessage
		if err := json.Unmarshal(cohort["delays"], &delays); err != nil {
			return err
		}
		if err := requireJSONKeys(delays, "request_to_first_proposal", "proposal_to_confirmed_hiring", "confirmed_hiring_to_reported_completion", "reported_completion_to_full_payment"); err != nil {
			return err
		}
		for _, delayRaw := range delays {
			var delay map[string]json.RawMessage
			if err := json.Unmarshal(delayRaw, &delay); err != nil {
				return err
			}
			if err := requireJSONKeys(delay, "observations", "mean_seconds"); err != nil {
				return err
			}
			var observations int64
			if err := json.Unmarshal(delay["observations"], &observations); err != nil || observations < 0 {
				return fmt.Errorf("%s delay observations must be a nonnegative integer", name)
			}
			if !isJSONNumberOrNull(delay["mean_seconds"]) {
				return fmt.Errorf("%s delay mean_seconds must be a JSON number or null", name)
			}
		}
	}
	return nil
}

func requireJSONKeys(object map[string]json.RawMessage, expected ...string) error {
	if len(object) != len(expected) {
		return fmt.Errorf("unexpected JSON fields: got %d keys, expected %d", len(object), len(expected))
	}
	for _, key := range expected {
		if _, ok := object[key]; !ok {
			return fmt.Errorf("aggregate response is missing allowed field %q", key)
		}
	}
	return nil
}

func validateFunnelResponseShape(body []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if err := requireJSONKeys(payload, "period", "timezone", "observed_at", "category_id", "rounding", "decimal_places", "cohorts"); err != nil {
		return err
	}
	var period, cohorts map[string]json.RawMessage
	if err := json.Unmarshal(payload["period"], &period); err != nil {
		return err
	}
	if err := requireJSONKeys(period, "from", "to"); err != nil {
		return err
	}
	if err := json.Unmarshal(payload["cohorts"], &cohorts); err != nil {
		return err
	}
	if err := requireJSONKeys(cohorts, "ai", "manual"); err != nil {
		return err
	}
	expectedStages := map[string][]string{
		"ai":     {"professional_assessment", "request", "proposal", "confirmed_hiring", "reported_completion", "full_payment", "review"},
		"manual": {"request", "proposal", "confirmed_hiring", "reported_completion", "full_payment", "review"},
	}
	for _, name := range []string{"ai", "manual"} {
		var cohort map[string]json.RawMessage
		if err := json.Unmarshal(cohorts[name], &cohort); err != nil {
			return err
		}
		if err := requireJSONKeys(cohort, "category_source", "stages", "global_completion_conversion_percentage", "delays"); err != nil {
			return err
		}
		var stages []map[string]json.RawMessage
		if err := json.Unmarshal(cohort["stages"], &stages); err != nil {
			return err
		}
		if len(stages) != len(expectedStages[name]) {
			return fmt.Errorf("%s cohort has %d stages, expected %d", name, len(stages), len(expectedStages[name]))
		}
		for i, stage := range stages {
			if err := requireJSONKeys(stage, "stage", "count", "conversion_percentage"); err != nil {
				return err
			}
			var stageName string
			if err := json.Unmarshal(stage["stage"], &stageName); err != nil {
				return err
			}
			if stageName != expectedStages[name][i] {
				return fmt.Errorf("%s stage %d is %q, expected %q", name, i, stageName, expectedStages[name][i])
			}
			var count int64
			if err := json.Unmarshal(stage["count"], &count); err != nil {
				return err
			}
			if !isJSONNumberOrNull(stage["conversion_percentage"]) {
				return fmt.Errorf("%s stage %s conversion_percentage must be a JSON number or null", name, stageName)
			}
		}
		if !isJSONNumberOrNull(cohort["global_completion_conversion_percentage"]) {
			return fmt.Errorf("%s global completion conversion must be a JSON number or null", name)
		}
		var delays map[string]json.RawMessage
		if err := json.Unmarshal(cohort["delays"], &delays); err != nil {
			return err
		}
		if err := requireJSONKeys(delays, "request_to_first_proposal", "proposal_to_confirmed_hiring", "confirmed_hiring_to_reported_completion", "reported_completion_to_full_payment"); err != nil {
			return err
		}
		for _, raw := range delays {
			var delay map[string]json.RawMessage
			if err := json.Unmarshal(raw, &delay); err != nil {
				return err
			}
			if err := requireJSONKeys(delay, "observations", "mean_seconds"); err != nil {
				return err
			}
			var observations int64
			if err := json.Unmarshal(delay["observations"], &observations); err != nil || observations < 0 {
				return fmt.Errorf("%s delay observations must be a nonnegative integer", name)
			}
			if !isJSONNumberOrNull(delay["mean_seconds"]) {
				return fmt.Errorf("%s delay mean_seconds must be a JSON number or null", name)
			}
		}
	}
	return nil
}
func (suite *testSuite) adminFunnelNoSideEffects() error {
	if suite.adminFunnelCapture.readAttempts != 1 {
		return fmt.Errorf("expected exactly one internal read and no write-side operation, got %d reads", suite.adminFunnelCapture.readAttempts)
	}
	if suite.adminFunnel.businessCounts == nil || suite.adminFunnel.readBaseline == nil {
		return fmt.Errorf("read-only business and aggregate baselines were not established")
	}
	counts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(suite.adminFunnel.businessCounts, counts) {
		return fmt.Errorf("funnel query changed persisted business row counts")
	}
	criteria := operation.FunnelCriteria{Period: operation.TimeWindow{From: suite.adminFunnel.periodFrom, To: suite.adminFunnel.periodTo}, CategoryID: suite.adminFunnel.response.CategoryID}
	after, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, criteria)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(*suite.adminFunnel.readBaseline, after) {
		return fmt.Errorf("funnel query changed persisted stage or delay facts")
	}
	if suite.adminFunnel.entitySnapshot == nil {
		return fmt.Errorf("business entity snapshot was not established")
	}
	entities, err := suite.adminFunnelSnapshotEntities()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(suite.adminFunnel.entitySnapshot, entities) {
		return fmt.Errorf("funnel query changed a persisted request, proposal, order, assessment conversation, or message")
	}
	watermark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	if watermark != suite.adminFunnel.auditWatermark {
		return fmt.Errorf("funnel query appended an administrative audit event")
	}
	return nil
}
func (suite *testSuite) adminFunnelNoExternalDependency() error {
	if suite.adminFunnelCapture.readAttempts != 1 {
		return fmt.Errorf("expected exactly one local reader invocation, got %d", suite.adminFunnelCapture.readAttempts)
	}
	if suite.chatbot.RequestCount() != suite.adminFunnel.chatbotRequestsBefore {
		return fmt.Errorf("funnel query invoked chatbot: before=%d after=%d", suite.adminFunnel.chatbotRequestsBefore, suite.chatbot.RequestCount())
	}
	if suite.checkoutClient.RequestCount() != suite.adminFunnel.checkoutRequestsBefore {
		return fmt.Errorf("funnel query initiated external checkout: before=%d after=%d", suite.adminFunnel.checkoutRequestsBefore, suite.checkoutClient.RequestCount())
	}
	if !suite.adminFunnel.externalSystemsOffline {
		return fmt.Errorf("external dependency availability was not disabled before the funnel query")
	}
	return nil
}
func (suite *testSuite) adminFunnelErrorNotZero() error {
	if suite.lastStatus != http.StatusInternalServerError {
		return fmt.Errorf("expected HTTP 500 for funnel reader failure, got %d", suite.lastStatus)
	}
	if suite.adminFunnelCapture.readAttempts != 1 {
		return fmt.Errorf("expected exactly one failed reader invocation, got %d", suite.adminFunnelCapture.readAttempts)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &payload); err != nil {
		return fmt.Errorf("invalid funnel error JSON: %w", err)
	}
	if err := requireJSONKeys(payload, "error"); err != nil {
		return fmt.Errorf("error response must not contain partial funnel metrics: %w", err)
	}
	var message string
	if err := json.Unmarshal(payload["error"], &message); err != nil {
		return fmt.Errorf("invalid funnel error message: %w", err)
	}
	if message != "internal server error" {
		return fmt.Errorf("unexpected funnel error message %q", message)
	}
	return nil
}

func (suite *testSuite) adminFunnelCategoryOriginCount(expected string) error {
	if suite.adminFunnel.response.Cohorts.AI.CategorySource != "assessment.problem_category_id" {
		return fmt.Errorf("AI category source=%q", suite.adminFunnel.response.Cohorts.AI.CategorySource)
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.AI, "professional_assessment", expected)
}
func (suite *testSuite) adminFunnelCategoryManualCount(expected string) error {
	if suite.adminFunnel.response.Cohorts.Manual.CategorySource != "provider.current_category_id" {
		return fmt.Errorf("manual category source=%q", suite.adminFunnel.response.Cohorts.Manual.CategorySource)
	}
	return suite.adminFunnelCountNumber(suite.adminFunnel.response.Cohorts.Manual, "request", expected)
}
func (suite *testSuite) adminFunnelNoOrigins() error {
	from, to := suite.clock.Now().Add(-30*24*time.Hour), suite.clock.Now()
	snapshot, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, operation.FunnelCriteria{Period: operation.TimeWindow{From: from, To: to}})
	if err != nil {
		return fmt.Errorf("checking empty activity precondition: %w", err)
	}
	if snapshot.AI.Counts.Origins != 0 || snapshot.Manual.Counts.Origins != 0 {
		return fmt.Errorf("expected no funnel origins before request, got AI=%d manual=%d", snapshot.AI.Counts.Origins, snapshot.Manual.Counts.Origins)
	}
	return nil
}
func (suite *testSuite) adminFunnelNoActivity365Days() error {
	from, to := suite.clock.Now().Add(-365*24*time.Hour), suite.clock.Now()
	snapshot, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, operation.FunnelCriteria{Period: operation.TimeWindow{From: from, To: to}})
	if err != nil {
		return fmt.Errorf("checking empty 365-day activity precondition: %w", err)
	}
	if snapshot.AI.Counts.Origins != 0 || snapshot.Manual.Counts.Origins != 0 {
		return fmt.Errorf("expected no funnel origins in 365-day range, got AI=%d manual=%d", snapshot.AI.Counts.Origins, snapshot.Manual.Counts.Origins)
	}
	return nil
}

func (suite *testSuite) adminFunnelUnknownActivity(raw string) error {
	id, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}
	from, to := suite.clock.Now().Add(-30*24*time.Hour), suite.clock.Now()
	snapshot, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, operation.FunnelCriteria{Period: operation.TimeWindow{From: from, To: to}, CategoryID: &id})
	if err != nil {
		return err
	}
	if snapshot.AI.Counts.Origins != 0 || snapshot.Manual.Counts.Origins != 0 {
		return fmt.Errorf("unexpected activity for category ID %d", id)
	}
	return nil
}
func (suite *testSuite) adminFunnelAuthWithoutPermission(permission string) error {
	suite.currentAuth0ID = auth0IDForAdminEmail("operador@example.com")
	suite.currentPermissions = []string{"write:admin_metrics"}
	if permission == "" {
		return fmt.Errorf("missing permission fixture")
	}
	return nil
}
func (suite *testSuite) adminFunnelAddOtherPermission() error {
	suite.currentPermissions = append(suite.currentPermissions, "read:admin_users")
	return nil
}
func (suite *testSuite) adminFunnelNoAudit() error {
	watermark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	from, to := suite.clock.Now().Add(-30*24*time.Hour), suite.clock.Now()
	baseline, err := suite.dependencies.Persistence.OperationFunnelReader.Read(suite.scenarioContext, operation.FunnelCriteria{Period: operation.TimeWindow{From: from, To: to}})
	if err != nil {
		return err
	}
	counts, err := testsupport.OperationDetailBusinessCounts(suite.scenarioContext, suite.database)
	if err != nil {
		return err
	}
	entities, err := suite.adminFunnelSnapshotEntities()
	if err != nil {
		return err
	}
	suite.adminFunnel.auditWatermark, suite.adminFunnel.businessCounts = watermark, counts
	suite.adminFunnel.entitySnapshot = entities
	suite.adminFunnel.requestSnapshot = entities.Requests
	suite.adminFunnel.chatbotRequestsBefore = suite.chatbot.RequestCount()
	suite.adminFunnel.checkoutRequestsBefore = suite.checkoutClient.RequestCount()
	// Success while side-effecting adapters are unavailable proves this read does not depend on them.
	suite.consumerAddressResolver.SetAvailable(false)
	if suite.calendarAvailability != nil {
		suite.calendarAvailability.SetAvailable(false)
	}
	suite.identityVerifier.SetAvailable(false)
	suite.adminFunnel.externalSystemsOffline = true
	suite.adminFunnel.periodFrom, suite.adminFunnel.periodTo = from, to
	suite.adminFunnel.readBaseline = &baseline
	return nil
}

func (suite *testSuite) adminFunnelSnapshotEntities() (*adminFunnelBusinessSnapshot, error) {
	snapshot := &adminFunnelBusinessSnapshot{
		Requests:       make(map[int]*jobrequest.JobRequest),
		Proposals:      make(map[int]*serviceproposal.ServiceProposal),
		Orders:         make(map[int]*workorder.WorkOrder),
		Conversations:  make(map[int]conversation.Conversation),
		CalendarEvents: make(map[string]int),
	}
	for _, id := range suite.adminFunnel.requestAliases {
		if id == 0 {
			continue
		}
		request, err := suite.jobRequestRepository.FindByID(id)
		if err != nil {
			return nil, fmt.Errorf("snapshotting request %d: %w", id, err)
		}
		snapshot.Requests[id] = request
		if request.ConversationID > 0 {
			if _, ok := snapshot.Conversations[request.ConversationID]; !ok {
				c, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.ConversationID)
				if err != nil {
					return nil, err
				}
				snapshot.Conversations[request.ConversationID] = c
			}
		}
	}
	for _, id := range suite.adminFunnel.proposalAliases {
		if id == 0 {
			continue
		}
		proposal, err := suite.dependencies.Persistence.ServiceProposalRepository.FindByID(suite.scenarioContext, id)
		if err != nil {
			return nil, fmt.Errorf("snapshotting proposal %d: %w", id, err)
		}
		snapshot.Proposals[id] = proposal
	}
	for _, id := range suite.adminFunnel.orderAliases {
		if id == 0 {
			continue
		}
		order, err := suite.workOrderRepository.FindByID(suite.scenarioContext, id)
		if err != nil {
			return nil, fmt.Errorf("snapshotting order %d: %w", id, err)
		}
		snapshot.Orders[id] = order
		if observer, ok := suite.calendarEventDetailsObserver.(calendarEventCountObserver); ok {
			for _, participantID := range []int{order.ConsumerID(), order.ProviderID()} {
				count, err := observer.EventCountForUser(suite.scenarioContext, participantID, id)
				if err != nil {
					return nil, fmt.Errorf("snapshotting calendar event count for order %d: %w", id, err)
				}
				snapshot.CalendarEvents[fmt.Sprintf("%d:%d", participantID, id)] = count
			}
		}
	}
	if id := suite.adminFunnel.assessmentConversationID; id > 0 {
		if _, ok := snapshot.Conversations[id]; !ok {
			c, err := suite.conversationRepository.FindByID(suite.scenarioContext, id)
			if err != nil {
				return nil, fmt.Errorf("snapshotting assessment conversation %d: %w", id, err)
			}
			snapshot.Conversations[id] = c
		}
	}
	return snapshot, nil
}

func isJSONNumber(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	var value float64
	return json.Unmarshal(raw, &value) == nil
}

func isJSONNumberOrNull(raw json.RawMessage) bool {
	return isJSONNull(raw) || isJSONNumber(raw)
}
