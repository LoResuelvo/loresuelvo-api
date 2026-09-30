package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type adminPaymentState struct {
	proposals      map[string]int
	orders         map[string]int
	intents        map[string]*payment.Intent
	firstPage      []string
	continuedPage  []string
	firstCursor    string
	pageLimit      int
	correlations   []string
	transactionIDs []string
	baseline       string
	before         map[string]string
	readFailure    bool
}

type adminPaymentsResponse struct {
	Payments []adminPaymentRow `json:"payments"`
	Page     struct {
		Limit      int     `json:"limit"`
		NextCursor *string `json:"next_cursor"`
	} `json:"page"`
}
type adminPaymentRow struct {
	ID                string                    `json:"id"`
	ServiceProposalID int                       `json:"service_proposal_id"`
	WorkOrderID       *int                      `json:"work_order_id"`
	ConsumerID        int                       `json:"consumer_id"`
	ProviderID        int                       `json:"provider_id"`
	Purpose           string                    `json:"purpose"`
	IntentStatus      string                    `json:"intent_status"`
	Currency          string                    `json:"currency"`
	SellerAmountCents int64                     `json:"seller_amount_cents"`
	PlatformFeeCents  int64                     `json:"platform_fee_cents"`
	TotalAmountCents  int64                     `json:"total_amount_cents"`
	CreatedOn         time.Time                 `json:"created_on"`
	Transactions      []adminPaymentTransaction `json:"transactions"`
	Breakdown         struct {
		Currency                     string `json:"currency"`
		ServiceTotalCents            int64  `json:"service_total_cents"`
		DepositCents                 int64  `json:"deposit_cents"`
		PlatformFeeTotalCents        int64  `json:"platform_fee_total_cents"`
		PlatformFeeDueNowCents       int64  `json:"platform_fee_due_now_cents"`
		AmountDueNowCents            *int64 `json:"amount_due_now_cents"`
		RemainingServiceBalanceCents *int64 `json:"remaining_service_balance_cents"`
		RemainingPlatformFeeCents    *int64 `json:"remaining_platform_fee_cents"`
		RemainingAmountDueCents      *int64 `json:"remaining_amount_due_cents"`
	} `json:"breakdown"`
	Summary struct {
		ApprovedAmounts []struct {
			Currency    string `json:"currency"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"approved_amounts"`
		PendingAmount *struct {
			Currency    string `json:"currency"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"pending_amount"`
		Anomalies []string `json:"anomalies"`
	} `json:"summary"`
	Anomalies []string `json:"anomalies"`
}
type adminPaymentTransaction struct {
	ID                 int64      `json:"id"`
	ExternalPaymentID  string     `json:"external_payment_id"`
	Status             string     `json:"status"`
	Currency           string     `json:"currency"`
	AmountCents        int64      `json:"amount_cents"`
	VerifiedOn         *time.Time `json:"verified_on"`
	ProcessorFeeCents  *int64     `json:"processor_fee_cents"`
	NetSettlementCents *int64     `json:"net_settlement_cents"`
}

func registerAdminPaymentSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que la propuesta "([^"]*)" de "([^"]*)" con "([^"]*)" conserva estos términos en centavos:$`, s.adminPaymentProposalWithTerms)
	sc.Step(`^que la propuesta "([^"]*)" de "([^"]*)" con "([^"]*)" conserva términos de contratación válidos$`, s.adminPaymentDefaultProposal)
	sc.Step(`^que la propuesta "([^"]*)" de "([^"]*)" con "([^"]*)" conserva términos contractuales en ARS$`, s.adminPaymentDefaultProposal)
	sc.Step(`^que la propuesta "([^"]*)" de "([^"]*)" con "([^"]*)" tiene el intento pagado "([^"]*)"$`, s.adminPaymentPaidIntent)
	sc.Step(`^que la orden "([^"]*)" corresponde a la propuesta "([^"]*)"$`, s.adminPaymentCreateOrder)
	sc.Step(`^que la orden existente "([^"]*)" corresponde a "([^"]*)", está en estado "([^"]*)" y tiene la seña pagada "([^"]*)" por (\d+) centavos (\w+) con exactamente una transacción externa aprobada y coincidente$`, s.adminPaymentOrderWithDeposit)
	sc.Step(`^que la orden existente "([^"]*)" corresponde a "([^"]*)" en estado "([^"]*)", con la seña pagada "([^"]*)" y el intento de saldo "([^"]*)"$`, s.adminPaymentOrderAndBalance)
	sc.Step(`^que el intento de seña "([^"]*)" de "([^"]*)" tiene el estado "([^"]*)" y estos importes persistidos:$`, s.adminPaymentIntentWithAmounts)
	sc.Step(`^que el intento de saldo "([^"]*)" de "([^"]*)" tiene el estado "([^"]*)", su sesión de checkout está vigente y estos importes persistidos:$`, s.adminPaymentBalanceWithAmounts)
	sc.Step(`^que el intento de seña "([^"]*)" de "([^"]*)" está pagado$`, s.adminPaymentIntentPaid)
	sc.Step(`^que el intento de seña "([^"]*)" de "([^"]*)" fue rechazado antes de iniciar nuevos intentos y no tiene transacción externa$`, s.adminPaymentIntentRejected)
	sc.Step(`^que "([^"]*)" fue creado el "([^"]*)" y quedó (rechazado|pagado) antes de crear el siguiente intento$`, s.adminPaymentIntentCreatedStatus)
	sc.Step(`^que "([^"]*)" fue creado el "([^"]*)" y quedó pagado el "([^"]*)"$`, s.adminPaymentIntentCreatedPaid)
	sc.Step(`^que "([^"]*)" es un intento "([^"]*)" en estado "([^"]*)" con una transacción persistida aprobada, ID de Mercado Pago "([^"]*)"; "([^"]*)" es "([^"]*)", está en "([^"]*)" y no tiene transacción externa$`, s.adminPaymentTwoSearchIntents)
	sc.Step(`^"([^"]*)" es un intento "([^"]*)" en estado "([^"]*)" con una transacción persistida aprobada, ID de Mercado Pago "([^"]*)"; "([^"]*)" es "([^"]*)", está en "([^"]*)" y no tiene transacción externa$`, s.adminPaymentTwoSearchIntents)
	sc.Step(`^que "([^"]*)" tiene la transacción aprobada "([^"]*)" por el importe total de su intento$`, s.adminPaymentApprovedTransaction)
	sc.Step(`^que "([^"]*)" tiene una transacción externa persistida con el ID de Mercado Pago "([^"]*)", estado "([^"]*)", importe (\d+) (\w+)$`, s.adminPaymentTransaction)
	sc.Step(`^que "([^"]*)" tiene las siguientes transacciones externas persistidas:$`, s.adminPaymentTransactionsTable)
	sc.Step(`^que "([^"]*)" tiene una transacción externa persistida con el ID de Mercado Pago "([^"]*)", estado "([^"]*)", importe (\d+) (\w+), y conserva una URL de checkout y una preferencia externa$`, s.adminPaymentTransactionWithCheckout)
	sc.Step(`^que la propuesta "([^"]*)" tiene la seña rechazada "([^"]*)" y la propuesta "([^"]*)" tiene la orden existente "([^"]*)" en estado "([^"]*)", con la seña pagada "([^"]*)" y el intento de saldo "([^"]*)"$`, s.adminPaymentSearchFixture)
	sc.Step(`^que en secuencia "([^"]*)" de seña expiró sin transacción externa, "([^"]*)" de seña fue rechazado y "([^"]*)" de seña está en "([^"]*)"; cada intento anterior quedó terminal antes de crear el siguiente$`, s.adminPaymentSequence)
	sc.Step(`^que las dos transacciones aprobadas siguientes son una anomalía histórica preexistente persistida directamente, no el resultado del flujo normal de notificaciones$`, func() error { return nil })
	sc.Step(`^que existe una inconsistencia histórica persistida directamente:.*$`, s.adminPaymentMismatchFixture)
	sc.Step(`^que durante la preparación del fixture se envía dos veces la misma notificación aprobada de Mercado Pago con ID "([^"]*)"$`, s.adminPaymentDuplicateNotification)
	sc.Step(`^que antes de la consulta el repositorio conserva una sola transacción con el ID externo "([^"]*)" asociada a "([^"]*)"$`, s.adminPaymentAssertOneSavedTransaction)
	sc.Step(`^que el UUID interno de "([^"]*)" es el valor canónico de "external_reference" al crear un checkout, distinto del ID externo "([^"]*)"; el valor devuelto por Mercado Pago no se afirma como persistido$`, s.adminPaymentExternalReference)
	sc.Step(`^el UUID interno de "([^"]*)" es el valor canónico de "external_reference" al crear un checkout; el valor devuelto por Mercado Pago no se persiste en la transacción$`, s.adminPaymentCanonicalExternalReference)
	sc.Step(`^que existen intentos de pago para "([^"]*)" con "([^"]*)", para "([^"]*)" con "([^"]*)" y para "([^"]*)" con "([^"]*)"$`, s.adminPaymentEmailSearchFixture)
	sc.Step(`^que hay intentos de seña pagados creados en "([^"]*)" y "([^"]*)", un intento de seña rechazado creado en "([^"]*)" y un intento de saldo pagado creado en "([^"]*)"$`, s.adminPaymentDateFilterFixture)
	sc.Step(`^que no hay intentos de pago que coincidan con la consulta$`, func() error { return nil })
	sc.Step(`^que existen tres intentos con la misma fecha de creación y los IDs persistidos ordenados "([^"]*)", "([^"]*)" e "([^"]*)" de mayor a menor$`, s.adminPaymentPaginationFixture)
	sc.Step(`^que obtuve la primera página de pagos con límite (\d+), correlación "([^"]*)" y guardé su cursor$`, s.adminPaymentFirstPage)
	sc.Step(`^que existen 101 intentos de pago que coinciden con la consulta$`, s.adminPayment101Fixture)
	sc.Step(`^que hay varios intentos para las propuestas "([^"]*)" y "([^"]*)"$`, s.adminPaymentTwoProposalFixture)
	sc.Step(`^que obtuve una página y un cursor válido al filtrar por la propuesta "([^"]*)"$`, s.adminPaymentFirstProposalPage)
	sc.Step(`^que existen dos intentos de pago administrativos y no hay evento con la correlación "([^"]*)"$`, s.adminPaymentTwoAuditFixture)
	sc.Step(`^que existe al menos un intento de pago que coincide con la consulta$`, s.adminPaymentOneFixture)
	sc.Step(`^que falla el almacenamiento del evento de auditoría de la consulta de pagos$`, s.adminPaymentFailAudit)
	sc.Step(`^que falla la lectura persistida de los pagos que coinciden con la consulta$`, s.adminPaymentFailRead)
	sc.Step(`^consulto los pagos administrativos con correlación "([^"]*)"$`, s.adminPaymentQueryCorrelation)
	sc.Step(`^consulto los pagos administrativos sin indicar un límite y con correlación "([^"]*)"$`, s.adminPaymentQueryCorrelation)
	sc.Step(`^consulto los pagos administrativos$`, s.adminPaymentQuery)
	sc.Step(`^consulto los pagos administrativos por la propuesta "([^"]*)"$`, s.adminPaymentQueryProposal)
	sc.Step(`^consulto los pagos administrativos por el ID exacto de "([^"]*)"$`, s.adminPaymentQueryIntent)
	sc.Step(`^consulto los pagos administrativos con el parámetro "([^"]*)" igual a "([^"]*)"$`, s.adminPaymentQueryParam)
	sc.Step(`^consulto los pagos administrativos con la consulta "([^"]*)"$`, s.adminPaymentQueryRaw)
	sc.Step(`^consulto los pagos administrativos con los filtros de correo "([^"]*)" para consumidor y "([^"]*)" para prestador$`, s.adminPaymentQueryEmails)
	sc.Step(`^consulto los pagos administrativos con propósito "([^"]*)", estado del intento "([^"]*)", desde "([^"]*)" hasta "([^"]*)"$`, s.adminPaymentQueryDateRange)
	sc.Step(`^consulto los pagos administrativos con límite (\d+)$`, s.adminPaymentQueryLimit)
	sc.Step(`^continúo los pagos administrativos con el cursor guardado y correlación "([^"]*)"$`, s.adminPaymentContinue)
	sc.Step(`^continúo la consulta con ese cursor y el filtro de propuesta "([^"]*)"$`, s.adminPaymentContinueWrongFilter)
	sc.Step(`^intento consultar los pagos administrativos$`, s.adminPaymentQuery)
	registerAdminPaymentAssertions(sc, s)
}

func registerAdminPaymentAssertions(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^la colección contiene el intento "([^"]*)" una sola vez, vinculado a "([^"]*)", a la orden "([^"]*)", al consumidor "([^"]*)" y al prestador "([^"]*)" mediante sus IDs internos$`, s.adminPaymentLinkedRow)
	sc.Step(`^la colección contiene el intento "([^"]*)" una sola vez, vinculado a "([^"]*)", a la orden "([^"]*)", al consumidor "([^"]*)" y al prestador "([^"]*)"$`, s.adminPaymentLinkedRow)
	sc.Step(`^el intento informa el propósito "([^"]*)", su estado "([^"]*)", moneda "([^"]*)" y creación persistida$`, s.adminPaymentIntentFields)
	sc.Step(`^el importe aprobado bruto es exactamente (\d+) centavos (\w+) porque no hay transacciones aprobadas persistidas$`, s.adminPaymentApprovedZero)
	sc.Step(`^el importe aprobado bruto es exactamente (\d+) centavos (\w+), no representa liquidación ni ingreso neto y está respaldado por la transacción aprobada coincidente de "([^"]*)"$`, s.adminPaymentApprovedAmount)
	sc.Step(`^el importe aprobado bruto es (\d+) centavos (\w+), sumando una vez cada ID externo distinto del procesador, sin afirmar liquidación ni ingreso neto$`, s.adminPaymentApprovedAmountForSingleRow)
	sc.Step(`^el importe aprobado bruto es (\d+) centavos (\w+), separado por moneda, sin esconder la discrepancia ni convertirlo o restarlo del saldo contractual en ARS$`, s.adminPaymentApprovedAmountForSingleRow)
	sc.Step(`^la página contiene exactamente el conjunto de intentos "([^"]*)", sin asumir un orden no definido para esta consulta$`, s.adminPaymentExpectedIDs)
	sc.Step(`^la página contiene únicamente el intento asociado a "([^"]*)" y "([^"]*)"$`, s.adminPaymentOnlyEmailPair)
	sc.Step(`^la página contiene únicamente el intento pagado creado exactamente al inicio del rango$`, s.adminPaymentOneRow)
	sc.Step(`^la página contiene la colección "payments" vacía y no nula, el límite predeterminado (\d+) y el cursor siguiente nulo$`, s.adminPaymentDefaultEmptyPage)
	sc.Step(`^la colección de transacciones externas de "([^"]*)" está vacía y no se presenta la sesión de checkout como evidencia de cobro$`, s.adminPaymentNoTransactions)
	sc.Step(`^la respuesta contiene una sola fila para el intento "([^"]*)" y conserva ambas transacciones externas, sus IDs y sus instantes$`, s.adminPaymentTwoTransactions)
	sc.Step(`^la respuesta no contiene intentos ni desglose económico$`, s.adminPaymentEmptyError)
	sc.Step(`^la respuesta no contiene intentos, transacciones ni desglose económico parcial$`, s.adminPaymentEmptyError)
	sc.Step(`^la respuesta no contiene intentos, transacciones ni importes presentados como cero$`, s.adminPaymentEmptyError)
	sc.Step(`^la respuesta no expone URLs de checkout, preferencias externas, credenciales OAuth, tokens, payloads crudos ni datos completos de instrumentos de pago$`, s.adminPaymentNoSensitiveData)
	sc.Step(`^el neto liquidado y las comisiones del procesador se informan como no disponibles, no como cero$`, s.adminPaymentSettlementUnavailable)
	sc.Step(`^como las transacciones persistidas no contienen comisión del procesador ni neto liquidado al prestador, esos campos se informan como no disponibles, no como cero, y no se afirma que los fondos se hayan liquidado$`, s.adminPaymentSettlementUnavailable)
	sc.Step(`^la respuesta identifica la discrepancia preexistente de moneda e importe entre los términos, el intento y la transacción para investigación, sin corregirla ni conciliarla$`, s.adminPaymentHasAnomaly)
	sc.Step(`^la existencia de más de una transacción aprobada se identifica como una inconsistencia para investigar y no elimina ni reemplaza ninguna evidencia$`, s.adminPaymentHasAnomaly)
	sc.Step(`^la transacción conserva el ID externo, el estado, la moneda, el importe y el instante de verificación persistidos$`, s.adminPaymentTransactionEvidence)
	sc.Step(`^los importes del intento separan la porción contractual del prestador y la comisión de plataforma, y su suma coincide con el total de (\d+)$`, s.adminPaymentIntentAmounts)
	sc.Step(`^el desglose en centavos informa (\d+) de servicio, (\d+) de seña del servicio, (\d+) de comisión de la seña, (\d+) a pagar en la reserva, (\d+) de saldo restante pactado del servicio, (\d+) de comisión restante pactada y (\d+) de total restante pactado; estos saldos son contractuales y no afirman cobros pendientes$`, s.adminPaymentBreakdown)
	sc.Step(`^la página contiene exactamente (\d+) intentos y el límite efectivo es (\d+)$`, s.adminPaymentCountAndLimit)
	sc.Step(`^el cursor siguiente no es nulo porque queda un intento coincidente fuera de esta página$`, s.adminPaymentHasCursor)
	sc.Step(`^la primera página contiene "([^"]*)" e "([^"]*)" y la siguiente contiene únicamente "([^"]*)"$`, s.adminPaymentPages)
	sc.Step(`^las páginas respetan el orden estable por fecha de creación descendente e ID de intento descendente, sin omisiones ni repeticiones$`, s.adminPaymentPagesStable)
	sc.Step(`^la continuación conserva el límite (\d+) y los filtros de la primera página$`, s.adminPaymentContinuation)
	sc.Step(`^la página contiene exactamente los intentos de la propuesta "([^"]*)"$`, s.adminPaymentProposalRows)
	sc.Step(`^no se registra un evento de acceso a los pagos para la consulta inválida$`, s.adminPaymentNoAudit)
	sc.Step(`^no se registra ningún evento de acceso a pagos$`, s.adminPaymentNoAudit)
	sc.Step(`^no queda persistido un evento de acceso para la consulta que falló al auditar$`, s.adminPaymentNoAudit)
	sc.Step(`^no se registra un evento de acceso preparado para una página que no pudo leerse$`, s.adminPaymentNoAudit)
	sc.Step(`^queda persistido exactamente un evento de acceso preparado al recurso "([^"]*)" por el operador "([^"]*)" con la correlación "([^"]*)"$`, s.adminPaymentAudit)
	sc.Step(`^antes de entregar la página queda persistido exactamente un evento de acceso preparado al recurso "([^"]*)" sin ID de fila, con el operador "([^"]*)" y la correlación "([^"]*)"$`, s.adminPaymentAudit)
	sc.Step(`^queda persistido exactamente un evento de acceso preparado al recurso "([^"]*)" para cada página exitosa, uno con cada correlación$`, s.adminPaymentAuditPages)
	sc.Step(`^el evento de auditoría no requiere motivo manual ni incluye filtros, correos, montos o contenido de la respuesta$`, s.adminPaymentAuditMetadata)
	sc.Step(`^no se registra un evento separado por cada intento o transacción de la página$`, s.adminPaymentAuditOnce)
	sc.Step(`^la consulta no crea ni modifica intentos, transacciones, propuestas ni órdenes$`, s.adminPaymentNoMutation)
	sc.Step(`^la consulta no modifica ni intenta conciliar el intento, la transacción, la propuesta o la orden$`, s.adminPaymentNoMutation)
	sc.Step(`^todos los filtros se combinan mediante AND y el fin del rango es exclusivo$`, s.adminPaymentFilterCombination)
	sc.Step(`^el rango temporal se aplica a la fecha de creación del intento, no a la verificación de sus transacciones$`, s.adminPaymentFilterCombination)
	sc.Step(`^el resultado específico de esta consulta es "([^"]*)"$`, s.adminPaymentSpecificSearch)
	sc.Step(`^la página contiene únicamente los intentos que coinciden con los filtros$`, s.adminPaymentBasicRows)
	sc.Step(`^que la transacción externa "([^"]*)" de "([^"]*)" tiene el ID de Mercado Pago "([^"]*)", estado "([^"]*)", importe (\d+) (\w+) y verificación "([^"]*)"$`, s.adminPaymentTransactionAt)
	sc.Step(`^no se publican porcentajes recomputados si no forman parte de los términos persistidos$`, s.adminPaymentPersistedTerms)
	sc.Step(`^que "([^"]*)" no tiene transacciones externas persistidas$`, s.adminPaymentNoTransactionsAlias)
	sc.Step(`^el desglose informa el total de servicio, la seña, la comisión de la seña y el saldo pactado en los términos persistidos de "([^"]*)", sin aplicar porcentajes actuales$`, s.adminPaymentPersistedTerms)
	sc.Step(`^el intento de saldo separa (\d+) centavos destinados contractualmente al prestador y (\d+) de comisión, cuya suma es el total de (\d+) centavos (\w+)$`, s.adminPaymentAmountBreakdown)
	sc.Step(`^el balance pendiente deriva de los términos válidos de "([^"]*)", la seña pagada y el estado contractual vigente de "([^"]*)", no de porcentajes actuales ni de la sesión de checkout$`, s.adminPaymentPendingBalance)
	sc.Step(`^el resumen a nivel de propuesta "([^"]*)" y orden "([^"]*)" informa (\d+) centavos (\w+) aprobados brutos por la seña aprobada, sin afirmar liquidación ni ingreso neto, y (\d+) centavos (\w+) de importe pendiente de cobro por el saldo contractual vigente$`, s.adminPaymentProposalSummary)
	sc.Step(`^esos importes corresponden al resumen de "([^"]*)"/"([^"]*)", no a un total global de la colección ni a la suma de importes de la fila del intento "([^"]*)"$`, s.adminPaymentProposalSummaryScope)
	sc.Step(`^ni la seña rechazada "([^"]*)" ni el intento de saldo "([^"]*)" sin transacción aprobada se suman al importe acreditado$`, s.adminPaymentExcludedAttempts)
	sc.Step(`^aparecen los tres intentos por separado con sus estados persistidos y sin una transacción aprobada$`, s.adminPaymentThreeAttempts)
	sc.Step(`^las transacciones persistidas "([^"]*)" y "([^"]*)" se muestran con sus estados "([^"]*)" y "([^"]*)" y ninguna se informa como importe acreditado$`, s.adminPaymentTwoStatuses)
	sc.Step(`^la URL y la preferencia no se presentan como acreditación ni como importe liquidado$`, s.adminPaymentNoSensitiveData)
	sc.Step(`^cada intento conserva el vínculo con "([^"]*)" y el consumidor y prestador correctos, e informa la orden como nula si todavía no existe$`, s.adminPaymentNoOrderLinkedRows)
	sc.Step(`^aparecen exactamente los intentos "([^"]*)" y "([^"]*)" en ese orden, cada uno una sola vez y con su propio estado, importe y fecha$`, s.adminPaymentOrderedAliases)
	sc.Step(`^sólo "([^"]*)" contiene la evidencia aprobada "([^"]*)" y no se atribuye esa transacción a "([^"]*)"$`, s.adminPaymentOnlyTransactionOnIntent)
	sc.Step(`^cada transacción queda vinculada a "([^"]*)" y a la misma propuesta, sin multiplicar la fila del intento$`, s.adminPaymentTransactionParent)
	sc.Step(`^el exceso sobre el importe contractual se identifica para investigación, sin restarlo del saldo contractual pendiente$`, s.adminPaymentExcessFlag)
	sc.Step(`^el intento, sus términos, sus transacciones y la orden relacionada permanecen sin modificaciones$`, s.adminPaymentNoMutation)
	sc.Step(`^los importes y monedas se muestran como persistidos, en centavos enteros, sin conversión ni recomposición con coma flotante$`, s.adminPaymentExactValues)
	sc.Step(`^"([^"]*)" aparece una sola vez y su evidencia contiene una sola transacción con el ID "([^"]*)"$`, s.adminPaymentOneTransaction)
	sc.Step(`^la búsqueda por correo admite coincidencia textual parcial sin distinguir mayúsculas, mientras los identificadores se buscan de manera exacta$`, s.adminPaymentEmailSearchContract)
	sc.Step(`^la página contiene exactamente cien intentos y el límite efectivo es (\d+)$`, s.adminPaymentHundredAndLimit)
	sc.Step(`^que estoy autenticado como administrador "([^"]*)" sólo con "([^"]*)"$`, s.iAmAuthenticatedAsAdminWithPermission)
	sc.Step(`^la transacción aprobada del intento creado exactamente al inicio del rango se verificó el "([^"]*)", fuera del rango de creación consultado$`, s.adminPaymentSetVerificationTime)
}

func (s *testSuite) adminPaymentState() *adminPaymentState {
	if s.adminPayments.proposals == nil {
		s.adminPayments.proposals = map[string]int{}
		s.adminPayments.orders = map[string]int{}
		s.adminPayments.intents = map[string]*payment.Intent{}
		s.adminPayments.before = map[string]string{}
	}
	return &s.adminPayments
}
func (s *testSuite) adminPaymentID(alias string) string {
	if len(alias) > 1 && alias[0] == 'I' {
		if n, err := strconv.ParseInt(strings.TrimLeft(alias[1:], "0"), 10, 64); err == nil && n > 0 {
			return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
		}
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("admin-payments/"+alias)).String()
}
func (s *testSuite) adminPaymentEnsureProposal(label, consumerEmail, providerEmail string, terms *serviceproposal.BookingTerms) (int, error) {
	st := s.adminPaymentState()
	if id := st.proposals[label]; id > 0 {
		return id, nil
	}
	participants, err := s.prepareServiceProposalFixtureParticipants(providerEmail, consumerEmail)
	if err != nil {
		return 0, err
	}
	scheduled := s.clock.Now().Add(72 * time.Hour)
	if terms == nil {
		t, e := serviceproposal.NewBookingPolicy().Calculate(3_333_333, scheduled)
		if e != nil {
			return 0, e
		}
		terms = &t
	} else {
		// Database enforces deadline = scheduled_on - 24h. Rebuild terms using
		// the exact schedule instant stored in this fixture.
		t, e := serviceproposal.NewBookingTerms(terms.Currency(), terms.ServiceTotalCents(), terms.DepositCents(), terms.PlatformFeeTotalCents(), terms.PlatformFeeDueNowCents(), scheduled.Add(-24*time.Hour))
		if e != nil {
			return 0, e
		}
		terms = &t
	}
	proposal, err := serviceproposal.NewServiceProposal(participants.provider, participants.consumer, participants.conversation, scheduled, "Administrative payment fixture", *terms, s.clock, 60)
	if err != nil {
		return 0, err
	}
	proposal.Status = serviceproposal.StatusAccepted
	saved, err := repositories.NewServiceProposalRepository(s.database).Save(proposal)
	if err != nil {
		return 0, err
	}
	st.proposals[label] = saved.ID
	return saved.ID, nil
}
func (s *testSuite) adminPaymentDefaultProposal(label, consumerEmail, providerEmail string) error {
	scheduled := s.clock.Now().Add(72 * time.Hour)
	terms, err := serviceproposal.NewBookingTerms("ARS", 3_333_333, 777_777, 555_555, 123_457, scheduled.Add(-24*time.Hour))
	if err != nil {
		return err
	}
	_, err = s.adminPaymentEnsureProposal(label, consumerEmail, providerEmail, &terms)
	return err
}
func (s *testSuite) adminPaymentProposalWithTerms(label, consumerEmail, providerEmail string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one booking-term row")
	}
	row := rows[0]
	read := func(k string) (int64, error) { return strconv.ParseInt(row[k], 10, 64) }
	serviceTotal, e := read("total del servicio")
	if e != nil {
		return e
	}
	deposit, e := read("seña del servicio")
	if e != nil {
		return e
	}
	fee, e := read("comisión total")
	if e != nil {
		return e
	}
	feeNow, e := read("comisión de la seña")
	if e != nil {
		return e
	}
	scheduled := s.clock.Now().Add(72 * time.Hour)
	terms, e := serviceproposal.NewBookingTerms(row["moneda"], serviceTotal, deposit, fee, feeNow, scheduled.Add(-24*time.Hour))
	if e != nil {
		return e
	}
	_, e = s.adminPaymentEnsureProposal(label, consumerEmail, providerEmail, &terms)
	return e
}
func (s *testSuite) adminPaymentPersistIntent(alias, proposalAlias, purpose, status, currency string, seller, fee, total int64, created time.Time) error {
	st := s.adminPaymentState()
	proposalID := st.proposals[proposalAlias]
	if proposalID == 0 {
		return fmt.Errorf("unknown payment proposal %q", proposalAlias)
	}
	intent := &payment.Intent{ID: s.adminPaymentID(alias), ServiceProposalID: proposalID, Purpose: payment.Purpose(purpose), Currency: currency, SellerAmountCents: seller, PlatformFeeCents: fee, TotalAmountCents: total, Status: payment.IntentStatus(status), CreatedOn: created.UTC(), UpdatedOn: created.UTC()}
	if err := s.paymentIntentRepository.Save(context.Background(), intent); err != nil {
		return err
	}
	st.intents[alias] = intent
	return nil
}
func (s *testSuite) adminPaymentCreateOrder(orderAlias, proposalAlias string) error {
	st := s.adminPaymentState()
	p := st.proposals[proposalAlias]
	if p == 0 {
		return fmt.Errorf("unknown payment proposal %q", proposalAlias)
	}
	proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(context.Background(), p)
	if err != nil {
		return err
	}
	order, err := workorder.New(proposal, s.clock.Now())
	if err != nil {
		return err
	}
	saved, err := s.workOrderRepository.Save(context.Background(), order)
	if err != nil {
		return err
	}
	st.orders[orderAlias] = saved.ID()
	return nil
}
func (s *testSuite) adminPaymentSaveTransaction(alias, external, status, currency string, amount int64, verified time.Time) error {
	intent := s.adminPaymentState().intents[alias]
	if intent == nil {
		return fmt.Errorf("unknown payment intent %q", alias)
	}
	txn := &payment.Transaction{PaymentIntentID: intent.ID, Processor: paymentaccount.PaymentProvider("mercado_pago"), ExternalPaymentID: external, SellerAccountID: "fixture-seller", Status: payment.ExternalPaymentStatus(status), Currency: currency, AmountCents: amount, VerifiedOn: verified.UTC(), CreatedOn: verified.UTC(), UpdatedOn: verified.UTC()}
	if err := s.paymentTransactionRepository.Save(context.Background(), txn); err != nil {
		return err
	}
	for _, id := range s.adminPaymentState().transactionIDs {
		if id == external {
			return nil
		}
	}
	s.adminPayments.transactionIDs = append(s.adminPayments.transactionIDs, external)
	return nil
}
func (s *testSuite) adminPaymentRequest(query url.Values, correlation string) error {
	if query == nil {
		query = url.Values{}
	}
	if correlation != "" {
		s.adminPayments.correlations = append(s.adminPayments.correlations, correlation)
	}
	baseline, err := s.adminPaymentPersistenceSnapshot()
	if err != nil {
		return err
	}
	s.adminPayments.baseline = baseline
	return s.sendAdminGet("/admin/payments", query, correlation)
}
func (s *testSuite) adminPaymentDecoded() (adminPaymentsResponse, error) {
	var got adminPaymentsResponse
	if err := json.Unmarshal(s.lastBody, &got); err != nil {
		return got, fmt.Errorf("decode admin payments response: %w: %s", err, s.lastBody)
	}
	return got, nil
}
func (s *testSuite) adminPaymentQuery() error { return s.adminPaymentRequest(url.Values{}, "") }
func (s *testSuite) adminPaymentQueryCorrelation(c string) error {
	return s.adminPaymentRequest(url.Values{}, c)
}
func (s *testSuite) adminPaymentQueryProposal(label string) error {
	id := s.adminPayments.proposals[label]
	if id == 0 {
		return fmt.Errorf("unknown proposal %q", label)
	}
	return s.adminPaymentRequest(url.Values{"service_proposal_id": {strconv.Itoa(id)}}, "")
}
func (s *testSuite) adminPaymentQueryIntent(label string) error {
	i := s.adminPayments.intents[label]
	if i == nil {
		return fmt.Errorf("unknown intent %q", label)
	}
	return s.adminPaymentRequest(url.Values{"payment_intent_id": {i.ID}}, "")
}
func (s *testSuite) adminPaymentQueryParam(key, value string) error {
	if key == "payment_intent_id" || key == "external_reference" {
		if i := s.adminPayments.intents[value]; i != nil {
			value = i.ID
		}
	}
	if key == "service_proposal_id" {
		if n := s.adminPayments.proposals[value]; n > 0 {
			value = strconv.Itoa(n)
		}
	}
	return s.adminPaymentRequest(url.Values{key: {value}}, "")
}
func (s *testSuite) adminPaymentQueryRaw(raw string) error {
	q, err := url.ParseQuery(raw)
	if err != nil {
		return err
	}
	return s.adminPaymentRequest(q, "")
}
func (s *testSuite) adminPaymentQueryEmails(c, p string) error {
	return s.adminPaymentRequest(url.Values{"consumer_email": {c}, "provider_email": {p}}, "")
}
func (s *testSuite) adminPaymentQueryDateRange(purpose, status, from, to string) error {
	return s.adminPaymentRequest(url.Values{"purpose": {purpose}, "intent_status": {status}, "created_from": {from}, "created_to": {to}}, "")
}
func (s *testSuite) adminPaymentQueryLimit(limit int) error {
	return s.adminPaymentRequest(url.Values{"limit": {strconv.Itoa(limit)}}, "")
}
func (s *testSuite) adminPaymentContinue(c string) error {
	got, err := s.adminPaymentDecoded()
	if err != nil {
		return err
	}
	if got.Page.NextCursor == nil {
		return fmt.Errorf("first response has no continuation cursor")
	}
	s.adminPayments.firstCursor = *got.Page.NextCursor
	s.adminPayments.pageLimit = got.Page.Limit
	q := url.Values{"cursor": {*got.Page.NextCursor}}
	s.adminPayments.firstPage = idsOf(got.Payments)
	return s.adminPaymentRequest(q, c)
}
func (s *testSuite) adminPaymentContinueWrongFilter(label string) error {
	if s.adminPayments.firstCursor == "" {
		got, err := s.adminPaymentDecoded()
		if err != nil {
			return err
		}
		if got.Page.NextCursor == nil {
			return fmt.Errorf("first response has no continuation cursor")
		}
		s.adminPayments.firstCursor = *got.Page.NextCursor
	}
	id := s.adminPayments.proposals[label]
	return s.adminPaymentRequest(url.Values{"cursor": {s.adminPayments.firstCursor}, "service_proposal_id": {strconv.Itoa(id)}}, "")
}
func idsOf(rows []adminPaymentRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// Existing setup helpers are deliberately kept explicit and repository-backed; the test steps never issue SQL.
func (s *testSuite) adminPaymentIntentWithAmounts(alias, proposal, status string, table *godog.Table) error {
	rows, e := inboxTableRows(table)
	if e != nil {
		return e
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one amount row")
	}
	r := rows[0]
	seller, e := strconv.ParseInt(r["porción del prestador"], 10, 64)
	if e != nil {
		return e
	}
	fee, e := strconv.ParseInt(r["comisión"], 10, 64)
	if e != nil {
		return e
	}
	total, e := strconv.ParseInt(r["total a pagar"], 10, 64)
	if e != nil {
		return e
	}
	return s.adminPaymentPersistIntent(alias, proposal, "booking_deposit", status, r["moneda"], seller, fee, total, s.clock.Now())
}
func (s *testSuite) adminPaymentBalanceWithAmounts(alias, order, status string, table *godog.Table) error {
	rows, e := inboxTableRows(table)
	if e != nil {
		return e
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected one amount row")
	}
	r := rows[0]
	seller, e := strconv.ParseInt(r["porción del prestador"], 10, 64)
	if e != nil {
		return e
	}
	fee, e := strconv.ParseInt(r["comisión"], 10, 64)
	if e != nil {
		return e
	}
	total, e := strconv.ParseInt(r["total a pagar"], 10, 64)
	if e != nil {
		return e
	}
	orderID := s.adminPayments.orders[order]
	o, e := s.workOrderRepository.FindByID(context.Background(), orderID)
	if e != nil {
		return e
	}
	intent, e := payment.NewServiceBalanceIntent(s.adminPaymentID(alias), o, s.clock.Now())
	if e != nil {
		return e
	}
	intent.Currency = r["moneda"]
	intent.SellerAmountCents = seller
	intent.PlatformFeeCents = fee
	intent.TotalAmountCents = total
	if status == "checkout_ready" {
		if e = intent.MarkCheckoutReady("pref-"+alias, "https://checkout.example.test/"+alias, s.clock.Now().Add(time.Hour), s.clock.Now()); e != nil {
			return fmt.Errorf("marking service balance checkout session ready: %w", e)
		}
	} else {
		intent.Status = payment.IntentStatus(status)
	}
	if e = s.paymentIntentRepository.Save(context.Background(), intent); e != nil {
		return e
	}
	s.adminPaymentState().intents[alias] = intent
	return nil
}
func (s *testSuite) adminPaymentIntentPaid(alias, proposal string) error {
	p := s.adminPayments.proposals[proposal]
	repo := repositories.NewServiceProposalRepository(s.database)
	pr, e := repo.FindByID(context.Background(), p)
	if e != nil {
		return e
	}
	terms := pr.BookingTerms
	return s.adminPaymentPersistIntent(alias, proposal, "booking_deposit", "paid", terms.Currency(), terms.DepositCents(), terms.PlatformFeeDueNowCents(), terms.AmountDueNowCents(), s.clock.Now())
}
func (s *testSuite) adminPaymentIntentRejected(alias, proposal string) error {
	p := s.adminPayments.proposals[proposal]
	pr, e := repositories.NewServiceProposalRepository(s.database).FindByID(context.Background(), p)
	if e != nil {
		return e
	}
	t := pr.BookingTerms
	return s.adminPaymentPersistIntent(alias, proposal, "booking_deposit", "rejected", t.Currency(), t.DepositCents(), t.PlatformFeeDueNowCents(), t.AmountDueNowCents(), s.clock.Now())
}
func (s *testSuite) adminPaymentCreateAndMaybeOrderIntent(alias, proposal, purpose, status string, created time.Time) error {
	p := s.adminPayments.proposals[proposal]
	pr, e := repositories.NewServiceProposalRepository(s.database).FindByID(context.Background(), p)
	if e != nil {
		return e
	}
	terms := pr.BookingTerms
	if purpose == "service_balance" {
		var orderID int
		for _, id := range s.adminPayments.orders {
			candidate, findErr := s.workOrderRepository.FindByID(context.Background(), id)
			if findErr == nil && candidate.ServiceProposalID() == p {
				orderID = id
				break
			}
		}
		if orderID == 0 {
			return fmt.Errorf("no order fixture exists for proposal %q", proposal)
		}
		o, e := s.workOrderRepository.FindByID(context.Background(), orderID)
		if e != nil {
			return e
		}
		intent, e := payment.NewServiceBalanceIntent(s.adminPaymentID(alias), o, created)
		if e != nil {
			return e
		}
		if status == "checkout_ready" {
			if e = intent.MarkCheckoutReady("pref-"+alias, "https://checkout.example.test/"+alias, s.clock.Now().Add(time.Hour), s.clock.Now()); e != nil {
				return e
			}
		} else {
			intent.Status = payment.IntentStatus(status)
		}
		if e = s.paymentIntentRepository.Save(context.Background(), intent); e != nil {
			return e
		}
		s.adminPaymentState().intents[alias] = intent
		return nil
	}
	intent, e := payment.NewBookingDepositIntent(s.adminPaymentID(alias), p, terms, created)
	if e != nil {
		return e
	}
	if status == "checkout_ready" {
		if e = intent.MarkCheckoutReady("pref-"+alias, "https://checkout.example.test/"+alias, s.clock.Now().Add(time.Hour), s.clock.Now()); e != nil {
			return e
		}
	} else {
		intent.Status = payment.IntentStatus(status)
	}
	if e = s.paymentIntentRepository.Save(context.Background(), intent); e != nil {
		return e
	}
	s.adminPaymentState().intents[alias] = intent
	return nil
}
func (s *testSuite) adminPaymentCreateOrderWithStatus(alias, proposal, status string) error {
	if e := s.adminPaymentCreateOrder(alias, proposal); e != nil {
		return e
	}
	if status != "awaiting_payment" {
		return nil
	}
	orderID := s.adminPayments.orders[alias]
	o, e := s.workOrderRepository.FindByID(context.Background(), orderID)
	if e != nil {
		return e
	}
	imageName := fmt.Sprintf("admin-payment-%s.jpg", strings.ToLower(alias))
	adminAuthID := s.currentAuth0ID
	s.currentAuth0ID = auth0IDForProviderEmail("juan@example.com")
	if e := s.uploadAndConfirmCompletionImage(imageName); e != nil {
		return fmt.Errorf("preparing awaiting-payment order evidence: %w", e)
	}
	s.currentAuth0ID = adminAuthID
	fileID := s.completionImagesByName[imageName].FileID
	report, e := workorder.NewCompletionReport("fixture", []string{fileID}, o.ScheduledOn().Add(time.Hour))
	if e != nil {
		return e
	}
	if e = o.ReportCompletion(o.ProviderID(), report); e != nil {
		return e
	}
	_, e = s.workOrderRepository.Save(context.Background(), o)
	return e
}
func (s *testSuite) adminPaymentOrderWithDeposit(order, proposal, status, deposit string, amount int64, currency string) error {
	if err := s.adminPaymentCreateOrderWithStatus(order, proposal, status); err != nil {
		return err
	}
	if err := s.adminPaymentIntentPaid(deposit, proposal); err != nil {
		return err
	}
	return s.adminPaymentSaveTransaction(deposit, "approved-"+deposit, "approved", currency, amount, s.clock.Now())
}
func (s *testSuite) adminPaymentOrderAndBalance(order, proposal, orderStatus, deposit, balance string) error {
	if err := s.adminPaymentCreateOrderWithStatus(order, proposal, orderStatus); err != nil {
		return err
	}
	if err := s.adminPaymentIntentPaid(deposit, proposal); err != nil {
		return err
	}
	return s.adminPaymentCreateAndMaybeOrderIntent(balance, proposal, "service_balance", "checkout_ready", s.clock.Now())
}
func (s *testSuite) adminPaymentApprovedTransaction(alias, external string) error {
	i := s.adminPayments.intents[alias]
	if i == nil {
		return fmt.Errorf("unknown intent %q", alias)
	}
	return s.adminPaymentSaveTransaction(alias, external, "approved", i.Currency, i.TotalAmountCents, s.clock.Now())
}
func (s *testSuite) adminPaymentTransaction(alias, external, status string, amount int64, currency string) error {
	return s.adminPaymentSaveTransaction(alias, external, status, currency, amount, s.clock.Now())
}
func (s *testSuite) adminPaymentTransactionWithCheckout(alias, external, status string, amount int64, currency string) error {
	if err := s.adminPaymentTransaction(alias, external, status, amount, currency); err != nil {
		return err
	}
	i := s.adminPayments.intents[alias]
	i.CheckoutSession = &payment.CheckoutSession{ExternalID: "preference-" + alias, URL: "https://checkout.example.test/" + alias, ExpiresOn: s.clock.Now().Add(time.Hour), CreatedOn: s.clock.Now()}
	return s.paymentIntentRepository.Save(context.Background(), i)
}
func (s *testSuite) adminPaymentTransactionsTable(alias string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	for _, r := range rows {
		n, e := strconv.ParseInt(r["importe"], 10, 64)
		if e != nil {
			return e
		}
		verified, e := time.Parse(time.RFC3339, r["verificada"])
		if e != nil {
			return e
		}
		if e = s.adminPaymentSaveTransaction(alias, r["ID de Mercado Pago"], r["estado"], r["moneda"], n, verified); e != nil {
			return e
		}
	}
	return nil
}
func (s *testSuite) adminPaymentPaidIntent(label, consumerEmail, providerEmail, alias string) error {
	if _, e := s.adminPaymentEnsureProposal(label, consumerEmail, providerEmail, nil); e != nil {
		return e
	}
	p := s.adminPayments.proposals[label]
	pr, e := repositories.NewServiceProposalRepository(s.database).FindByID(context.Background(), p)
	if e != nil {
		return e
	}
	t := pr.BookingTerms
	if e = s.adminPaymentPersistIntent(alias, label, "booking_deposit", "paid", t.Currency(), t.DepositCents(), t.PlatformFeeDueNowCents(), t.AmountDueNowCents(), s.clock.Now()); e != nil {
		return e
	}
	return s.adminPaymentSaveTransaction(alias, "mp-payment-9005", "approved", t.Currency(), t.AmountDueNowCents(), s.clock.Now())
}
func (s *testSuite) adminPaymentCreatedStatus(alias, created, status string) error {
	t, e := time.Parse(time.RFC3339, created)
	if e != nil {
		return e
	}
	return s.adminPaymentCreateAndMaybeOrderIntent(alias, "P1", "booking_deposit", status, t)
}
func (s *testSuite) adminPaymentIntentCreatedStatus(alias, created, status string) error {
	t, e := time.Parse(time.RFC3339, created)
	if e != nil {
		return e
	}
	status = map[string]string{"rechazado": "rejected", "pagado": "paid"}[status]
	if status == "" {
		return fmt.Errorf("unsupported persisted payment status")
	}
	return s.adminPaymentCreateAndMaybeOrderIntent(alias, "P1", "booking_deposit", status, t)
}
func (s *testSuite) adminPaymentIntentCreatedPaid(alias, created, paidOn string) error {
	if err := s.adminPaymentCreatedStatus(alias, created, "paid"); err != nil {
		return err
	}
	i := s.adminPayments.intents[alias]
	t, e := time.Parse(time.RFC3339, paidOn)
	if e != nil {
		return e
	}
	i.UpdatedOn = t.UTC()
	return s.paymentIntentRepository.Save(context.Background(), i)
}
func (s *testSuite) adminPaymentTwoSearchIntents(alias1, purpose1, status1, external, alias2, purpose2, status2 string) error {
	if err := s.adminPaymentCreateAndMaybeOrderIntent(alias1, "P2", purpose1, status1, s.clock.Now()); err != nil {
		return err
	}
	if err := s.adminPaymentApprovedTransaction(alias1, external); err != nil {
		return err
	}
	return s.adminPaymentCreateAndMaybeOrderIntent(alias2, "P2", purpose2, status2, s.clock.Now())
}
func (s *testSuite) adminPaymentSequence(first, second, third, status string) error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	for index, item := range []struct{ alias, status string }{{first, "expired"}, {second, "rejected"}, {third, status}} {
		if err := s.adminPaymentCreateAndMaybeOrderIntent(item.alias, "P1", "booking_deposit", item.status, s.clock.Now().Add(time.Duration(index)*time.Minute)); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) adminPaymentSearchFixture(p1, i1, p2, o2, status, i2, i3 string) error {
	if _, e := s.adminPaymentEnsureProposal(p1, "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	if _, e := s.adminPaymentEnsureProposal(p2, "beatriz@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	if e := s.adminPaymentCreateAndMaybeOrderIntent(i1, p1, "booking_deposit", "rejected", s.clock.Now()); e != nil {
		return e
	}
	if e := s.adminPaymentCreateOrderWithStatus(o2, p2, status); e != nil {
		return e
	}
	if e := s.adminPaymentCreateAndMaybeOrderIntent(i2, p2, "booking_deposit", "paid", s.clock.Now()); e != nil {
		return e
	}
	if e := s.adminPaymentApprovedTransaction(i2, "9010"); e != nil {
		return e
	}
	return s.adminPaymentCreateAndMaybeOrderIntent(i3, p2, "service_balance", "checkout_ready", s.clock.Now())
}
func (s *testSuite) adminPaymentMismatchFixture() error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	if e := s.adminPaymentCreateAndMaybeOrderIntent("I1", "P1", "booking_deposit", "paid", s.clock.Now()); e != nil {
		return e
	}
	return s.adminPaymentSaveTransaction("I1", "mismatch-usd", "approved", "USD", 901235, s.clock.Now())
}
func (s *testSuite) adminPaymentAssertOneSavedTransaction(external, alias string) error {
	tx, e := s.paymentTransactionRepository.FindByExternalID(context.Background(), paymentaccount.PaymentProvider("mercado_pago"), external)
	if e != nil {
		return e
	}
	i := s.adminPayments.intents[alias]
	if tx.PaymentIntentID != i.ID {
		return fmt.Errorf("transaction linked to %s not %s", tx.PaymentIntentID, i.ID)
	}
	tracked := false
	for _, id := range s.adminPayments.transactionIDs {
		if id == external {
			tracked = true
			break
		}
	}
	if !tracked {
		s.adminPayments.transactionIDs = append(s.adminPayments.transactionIDs, external)
	}
	s.adminPayments.before["single_transaction"] = "yes"
	return nil
}
func (s *testSuite) adminPaymentExternalReference(alias, external string) error {
	i := s.adminPayments.intents[alias]
	if i == nil {
		return fmt.Errorf("unknown intent %q", alias)
	}
	// This checks the identity mapping queried by the admin API. Existing
	// checkout-flow tests prove the gateway receives this UUID; this historical
	// fixture intentionally does not replay or fabricate a checkout request.
	if i.ID != s.adminPaymentID(alias) || i.ID == external {
		return fmt.Errorf("canonical external_reference must be the exact internal UUID, distinct from payment ID")
	}
	s.adminPayments.before["external_reference"] = i.ID
	s.adminPayments.before["external_payment_id"] = external
	return nil
}
func (s *testSuite) adminPaymentCanonicalExternalReference(alias string) error {
	i := s.adminPayments.intents[alias]
	if i == nil {
		return fmt.Errorf("unknown intent %q", alias)
	}
	if i.ID != s.adminPaymentID(alias) {
		return fmt.Errorf("canonical external_reference must be the exact internal UUID")
	}
	s.adminPayments.before["external_reference"] = i.ID
	return nil
}

func (s *testSuite) adminPaymentEmailSearchFixture(c1, p1, c2, p2, c3, p3 string) error {
	for idx, pair := range [][2]string{{c1, p1}, {c2, p2}, {c3, p3}} {
		pl := fmt.Sprintf("P%d", idx+1)
		il := fmt.Sprintf("I%d", idx+1)
		if _, e := s.adminPaymentEnsureProposal(pl, pair[0], pair[1], nil); e != nil {
			return e
		}
		if e := s.adminPaymentCreateAndMaybeOrderIntent(il, pl, "booking_deposit", "paid", s.clock.Now().Add(time.Duration(idx)*time.Second)); e != nil {
			return e
		}
	}
	return nil
}
func (s *testSuite) adminPaymentDateFilterFixture(from, to, rejected, balance string) error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	if _, e := s.adminPaymentEnsureProposal("P2", "beatriz@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	for _, item := range []struct{ a, when, status, purpose string }{{"I1", from, "paid", "booking_deposit"}, {"I2", to, "paid", "booking_deposit"}, {"I3", rejected, "rejected", "booking_deposit"}, {"I4", balance, "paid", "service_balance"}} {
		t, e := time.Parse(time.RFC3339, item.when)
		if e != nil {
			return e
		}
		if item.purpose == "service_balance" { // no associated order needed for the persisted historical search fixture
			if e = s.adminPaymentPersistIntent(item.a, "P1", item.purpose, item.status, "ARS", 100, 0, 100, t); e != nil {
				return e
			}
		} else {
			proposal := "P1"
			if item.a == "I2" {
				proposal = "P2"
			}
			if e = s.adminPaymentCreateAndMaybeOrderIntent(item.a, proposal, item.purpose, item.status, t); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *testSuite) adminPaymentSetVerificationTime(value string) error {
	t, e := time.Parse(time.RFC3339, value)
	if e != nil {
		return e
	}
	i := s.adminPayments.intents["I1"]
	if i == nil {
		return fmt.Errorf("missing I1")
	}
	return s.adminPaymentSaveTransaction("I1", "range-approved", "approved", i.Currency, i.TotalAmountCents, t)
}
func (s *testSuite) adminPaymentPaginationFixture(i3, i2, i1 string) error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	created := s.clock.Now()
	for _, a := range []string{i1, i2, i3} {
		if e := s.adminPaymentCreateAndMaybeOrderIntent(a, "P1", "booking_deposit", "rejected", created); e != nil {
			return e
		}
	}
	return nil
}
func (s *testSuite) adminPaymentFirstPage(limit int, c string) error {
	s.adminPayments.pageLimit = limit
	if err := s.adminPaymentRequest(url.Values{"limit": {strconv.Itoa(limit)}}, c); err != nil {
		return err
	}
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	s.adminPayments.firstPage = idsOf(got.Payments)
	if got.Page.NextCursor == nil {
		return fmt.Errorf("expected pagination cursor")
	}
	s.adminPayments.firstCursor = *got.Page.NextCursor
	return nil
}
func (s *testSuite) adminPayment101Fixture() error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	for n := 0; n < 101; n++ {
		a := fmt.Sprintf("I%03d", n)
		if e := s.adminPaymentCreateAndMaybeOrderIntent(a, "P1", "booking_deposit", "rejected", s.clock.Now().Add(time.Duration(n)*time.Microsecond)); e != nil {
			return e
		}
	}
	return nil
}
func (s *testSuite) adminPaymentTwoProposalFixture(p1, p2 string) error {
	for n, p := range []string{p1, p2} {
		if _, e := s.adminPaymentEnsureProposal(p, "ana@example.com", "juan@example.com", nil); e != nil {
			return e
		}
		for i := 0; i < 2; i++ {
			if e := s.adminPaymentCreateAndMaybeOrderIntent(fmt.Sprintf("%s-I%d", p, i), p, "booking_deposit", "rejected", s.clock.Now().Add(time.Duration(n*2+i)*time.Second)); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *testSuite) adminPaymentFirstProposalPage(proposal string) error {
	id := s.adminPayments.proposals[proposal]
	if e := s.adminPaymentRequest(url.Values{"service_proposal_id": {strconv.Itoa(id)}, "limit": {"1"}}, ""); e != nil {
		return e
	}
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if got.Page.NextCursor == nil {
		return fmt.Errorf("proposal first page lacks cursor")
	}
	s.adminPayments.firstCursor = *got.Page.NextCursor
	return nil
}
func (s *testSuite) adminPaymentTwoAuditFixture(correlation string) error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	for _, a := range []string{"I1", "I2"} {
		if e := s.adminPaymentCreateAndMaybeOrderIntent(a, "P1", "booking_deposit", "rejected", s.clock.Now()); e != nil {
			return e
		}
	}
	s.adminPayments.before["audit_correlation"] = correlation
	return nil
}
func (s *testSuite) adminPaymentOneFixture() error {
	if _, e := s.adminPaymentEnsureProposal("P1", "ana@example.com", "juan@example.com", nil); e != nil {
		return e
	}
	return s.adminPaymentCreateAndMaybeOrderIntent("I1", "P1", "booking_deposit", "requires_checkout", s.clock.Now())
}
func (s *testSuite) adminPaymentFailAudit() error {
	s.adminPayments.before["fail_audit"] = "true"
	s.adminPaymentCapture.failAudit = true
	return nil
}
func (s *testSuite) adminPaymentFailRead() error {
	s.adminPayments.readFailure = true
	s.adminPaymentCapture.failRead = true
	return nil
}

func (s *testSuite) adminPaymentFindRow(alias string) (adminPaymentRow, error) {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return adminPaymentRow{}, e
	}
	wanted := s.adminPaymentID(alias)
	for _, r := range got.Payments {
		if r.ID == wanted {
			return r, nil
		}
	}
	return adminPaymentRow{}, fmt.Errorf("response does not contain intent %s (%s)", alias, wanted)
}
func (s *testSuite) adminPaymentLinkedRow(alias, proposal, order, consumerEmail, providerEmail string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	p := s.adminPayments.proposals[proposal]
	if r.ServiceProposalID != p {
		return fmt.Errorf("intent linked to proposal %d, expected %d", r.ServiceProposalID, p)
	}
	if order != "" {
		id := s.adminPayments.orders[order]
		if r.WorkOrderID == nil || *r.WorkOrderID != id {
			return fmt.Errorf("intent work order is %v, expected %d", r.WorkOrderID, id)
		}
	}
	consumerUser, e := s.userRepository.FindByAuthID(auth0IDForConsumerEmail(consumerEmail))
	if e != nil {
		return e
	}
	providerUser, e := s.userRepository.FindByAuthID(auth0IDForProviderEmail(providerEmail))
	if e != nil {
		return e
	}
	if consumerUser.ID() != r.ConsumerID || providerUser.ID() != r.ProviderID {
		return fmt.Errorf("participant IDs mismatch: response consumer/provider %d/%d", r.ConsumerID, r.ProviderID)
	}
	return nil
}
func (s *testSuite) adminPaymentIntentFields(purpose, status, currency string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) == 0 {
		return fmt.Errorf("expected payment row")
	}
	for _, r := range got.Payments {
		if r.Purpose != purpose || r.IntentStatus != status || r.Currency != currency || r.CreatedOn.IsZero() {
			return fmt.Errorf("unexpected intent fields %+v", r)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentApprovedZero(amount int64, currency string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		if len(r.Summary.ApprovedAmounts) != 0 {
			return fmt.Errorf("expected no approved amount rows, got %+v", r.Summary.ApprovedAmounts)
		}
	}
	if amount != 0 {
		return fmt.Errorf("expected zero aggregate fixture amount, got %d", amount)
	}
	return nil
}
func (s *testSuite) adminPaymentApprovedAmount(amount int64, currency, alias string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	return assertApprovedAmount(r, amount, currency)
}
func (s *testSuite) adminPaymentApprovedAmountForSingleRow(amount int64, currency string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		if e = assertApprovedAmount(r, amount, currency); e == nil {
			return nil
		}
	}
	return fmt.Errorf("no row has approved aggregate %d %s", amount, currency)
}
func assertApprovedAmount(r adminPaymentRow, amount int64, currency string) error {
	for _, a := range r.Summary.ApprovedAmounts {
		if a.Currency == currency && a.AmountCents == amount {
			return nil
		}
	}
	return fmt.Errorf("approved amounts do not include %d %s: %+v", amount, currency, r.Summary.ApprovedAmounts)
}
func (s *testSuite) adminPaymentExpectedIDs(values string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	expected := strings.Split(values, ",")
	actual := map[string]int{}
	for _, r := range got.Payments {
		actual[r.ID]++
	}
	for i := range expected {
		expected[i] = strings.TrimSpace(expected[i])
	}
	if len(expected) == 1 && expected[0] == "ninguno" {
		if len(got.Payments) != 0 {
			return fmt.Errorf("expected no rows, got %d", len(got.Payments))
		}
		return nil
	}
	if len(got.Payments) != len(expected) {
		return fmt.Errorf("expected aliases %v (%d rows), got %v", expected, len(expected), idsOf(got.Payments))
	}
	for _, a := range expected {
		if actual[s.adminPaymentID(a)] != 1 {
			return fmt.Errorf("expected exactly one row for %s; got %v", a, idsOf(got.Payments))
		}
	}
	return nil
}
func (s *testSuite) adminPaymentOnlyEmailPair(consumerEmail, providerEmail string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) != 1 {
		return fmt.Errorf("expected one payment row, got %d", len(got.Payments))
	}
	c, err := s.userRepository.FindByAuthID(auth0IDForConsumerEmail(strings.ToLower(consumerEmail)))
	if err != nil {
		return err
	}
	p, err := s.userRepository.FindByAuthID(auth0IDForProviderEmail(strings.ToLower(providerEmail)))
	if err != nil {
		return err
	}
	if got.Payments[0].ConsumerID != c.ID() || got.Payments[0].ProviderID != p.ID() {
		return fmt.Errorf("partial email query returned unrelated internal participant IDs %d/%d", got.Payments[0].ConsumerID, got.Payments[0].ProviderID)
	}
	return nil
}
func (s *testSuite) adminPaymentOneRow() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) != 1 {
		return fmt.Errorf("expected one matching row, got %d", len(got.Payments))
	}
	r := got.Payments[0]
	expected := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if r.ID != s.adminPaymentID("I1") || r.Purpose != "booking_deposit" || r.IntentStatus != "paid" || !r.CreatedOn.Equal(expected) {
		return fmt.Errorf("filter returned wrong row instead of paid I1 created at inclusive start: %+v", r)
	}
	return nil
}
func (s *testSuite) adminPaymentDefaultEmptyPage(limit int) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if got.Payments == nil || len(got.Payments) != 0 || got.Page.Limit != limit || got.Page.NextCursor != nil {
		return fmt.Errorf("unexpected empty page: %+v", got)
	}
	return nil
}
func (s *testSuite) adminPaymentNoTransactions(alias string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if r.Transactions == nil || len(r.Transactions) != 0 {
		return fmt.Errorf("expected no transaction evidence: %+v", r.Transactions)
	}
	return nil
}
func (s *testSuite) adminPaymentTwoTransactions(alias string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if len(r.Transactions) != 2 {
		return fmt.Errorf("expected two transactions, got %d", len(r.Transactions))
	}
	expected := map[string]bool{"9003": false, "9004": false}
	for _, t := range r.Transactions {
		if _, ok := expected[t.ExternalPaymentID]; ok {
			expected[t.ExternalPaymentID] = true
		}
	}
	for id, found := range expected {
		if !found {
			return fmt.Errorf("missing transaction %s", id)
		}
	}
	return s.adminPaymentTransactionEvidence()
}
func (s *testSuite) adminPaymentEmptyError() error {
	var body map[string]json.RawMessage
	if e := json.Unmarshal(s.lastBody, &body); e != nil {
		return e
	}
	if len(body) == 0 {
		return nil
	}
	if raw, ok := body["payments"]; ok && string(raw) != "[]" {
		return fmt.Errorf("error response contains payment rows: %s", raw)
	}
	if strings.Contains(strings.ToLower(string(s.lastBody)), "approved_amounts") || strings.Contains(strings.ToLower(string(s.lastBody)), "total_amount_cents") {
		return fmt.Errorf("error response contains economic breakdown: %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) adminPaymentNoSensitiveData() error {
	for _, needle := range []string{"checkout.example.test", "preference-", "access_token", "refresh_token", "oauth", "payload", "card_number"} {
		if strings.Contains(string(s.lastBody), needle) {
			return fmt.Errorf("response leaks sensitive marker %q", needle)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentSettlementUnavailable() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		for _, t := range r.Transactions {
			if t.ProcessorFeeCents != nil || t.NetSettlementCents != nil {
				return fmt.Errorf("unsupported settlement was published: %+v", t)
			}
		}
	}
	return nil
}
func (s *testSuite) adminPaymentHasAnomaly() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	intent := s.adminPayments.intents["I1"]
	if intent == nil {
		return fmt.Errorf("missing anomaly intent I1")
	}
	row, e := s.adminPaymentFindRow("I1")
	if e != nil {
		return e
	}
	flags := append(append([]string{}, row.Anomalies...), row.Summary.Anomalies...)
	has := func(w string) bool {
		for _, f := range flags {
			if f == w {
				return true
			}
		}
		return false
	}
	if len(row.Transactions) > 1 {
		if !has("multiple_approved_transactions") || !has("approved_amount_exceeds_intent_total") {
			return fmt.Errorf("multiple-approval anomaly not reported: %v", flags)
		}
		return nil
	}
	if len(row.Transactions) == 1 && row.Transactions[0].Currency != "ARS" {
		if !has("transaction_currency_mismatch") || !has("transaction_amount_mismatch") {
			return fmt.Errorf("currency/amount anomaly not reported: %v", flags)
		}
		return nil
	}
	return fmt.Errorf("fixture has no targeted anomaly evidence; response flags=%v payments=%d", flags, len(got.Payments))
}
func (s *testSuite) adminPaymentTransactionEvidence() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, external := range s.adminPayments.transactionIDs {
		persisted, err := s.paymentTransactionRepository.FindByExternalID(context.Background(), paymentaccount.PaymentProvider("mercado_pago"), external)
		if err != nil {
			return err
		}
		parentVisible := false
		matched := false
		for _, row := range got.Payments {
			if row.ID == persisted.PaymentIntentID {
				parentVisible = true
			}
			for _, tx := range row.Transactions {
				if tx.ExternalPaymentID == external {
					if row.ID != persisted.PaymentIntentID {
						return fmt.Errorf("transaction %s is attributed to intent %s, expected %s", external, row.ID, persisted.PaymentIntentID)
					}
					matched = true
					if tx.Status != string(persisted.Status) || tx.Currency != persisted.Currency || tx.AmountCents != persisted.AmountCents || tx.VerifiedOn == nil || !tx.VerifiedOn.Equal(persisted.VerifiedOn) {
						return fmt.Errorf("transaction %s differs from persisted evidence", external)
					}
				}
			}
		}
		if parentVisible && !matched {
			return fmt.Errorf("response for intent %s omitted persisted transaction %s", persisted.PaymentIntentID, external)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentIntentAmounts(total int64) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		if r.TotalAmountCents == total && r.SellerAmountCents+r.PlatformFeeCents == total {
			return nil
		}
	}
	return fmt.Errorf("no intent has consistent split totaling %d", total)
}
func (s *testSuite) adminPaymentBreakdown(service, deposit, feeNow, dueNow, remainingService, remainingFee, remainingTotal int64) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) == 0 {
		return fmt.Errorf("no payment rows")
	}
	b := got.Payments[0].Breakdown
	if b.ServiceTotalCents != service || b.DepositCents != deposit || b.PlatformFeeDueNowCents != feeNow || b.AmountDueNowCents == nil || *b.AmountDueNowCents != dueNow || b.RemainingServiceBalanceCents == nil || *b.RemainingServiceBalanceCents != remainingService || b.RemainingPlatformFeeCents == nil || *b.RemainingPlatformFeeCents != remainingFee || b.RemainingAmountDueCents == nil || *b.RemainingAmountDueCents != remainingTotal {
		return fmt.Errorf("unexpected contractual breakdown %+v", b)
	}
	return nil
}
func (s *testSuite) adminPaymentCountAndLimit(count, limit int) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) != count || got.Page.Limit != limit {
		return fmt.Errorf("expected %d rows at limit %d, got %d/%d", count, limit, len(got.Payments), got.Page.Limit)
	}
	return nil
}
func (s *testSuite) adminPaymentHundredAndLimit(limit int) error {
	return s.adminPaymentCountAndLimit(100, limit)
}
func (s *testSuite) adminPaymentHasCursor() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if got.Page.NextCursor == nil {
		return fmt.Errorf("expected next cursor")
	}
	return nil
}
func (s *testSuite) adminPaymentPages(i3, i2, i1 string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	ids := idsOf(got.Payments)
	want := []string{s.adminPaymentID(i3), s.adminPaymentID(i2)}
	if !equalStrings(s.adminPayments.firstPage, want) {
		return fmt.Errorf("first page %v, expected %v", s.adminPayments.firstPage, want)
	}
	if !equalStrings(ids, []string{s.adminPaymentID(i1)}) {
		return fmt.Errorf("continuation %v, expected %s", ids, i1)
	}
	s.adminPayments.continuedPage = ids
	return nil
}
func (s *testSuite) adminPaymentPagesStable() error {
	seen := map[string]bool{}
	for _, id := range s.adminPayments.firstPage {
		if seen[id] {
			return fmt.Errorf("duplicate ID across pages: %s", id)
		}
		seen[id] = true
	}
	for _, id := range s.adminPayments.continuedPage {
		if seen[id] {
			return fmt.Errorf("duplicate ID across pages: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != len(s.adminPayments.firstPage)+len(s.adminPayments.continuedPage) {
		return fmt.Errorf("page omission/repetition")
	}
	return nil
}
func (s *testSuite) adminPaymentContinuation(limit int) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if got.Page.Limit != limit {
		return fmt.Errorf("continuation limit %d, expected %d", got.Page.Limit, limit)
	}
	return nil
}
func (s *testSuite) adminPaymentProposalRows(label string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	proposalID := s.adminPayments.proposals[label]
	for _, r := range got.Payments {
		if r.ServiceProposalID != proposalID {
			return fmt.Errorf("row from unrelated proposal %d", r.ServiceProposalID)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentNoAudit() error {
	if s.adminPayments.before["fail_audit"] == "true" {
		e := s.adminPaymentCapture.lastEvent
		if e == nil {
			return fmt.Errorf("failed audit did not construct an event")
		}
		if s.adminPaymentCapture.auditAttempts != 1 {
			return fmt.Errorf("expected one failed audit write, got %d", s.adminPaymentCapture.auditAttempts)
		}
		if _, err := s.auditEvents.Reader.FindByID(context.Background(), e.ID()); !errors.Is(err, audit.ErrNotFound) {
			return fmt.Errorf("failed audit event unexpectedly persisted")
		}
		return nil
	}
	if s.adminPaymentCapture.auditAttempts != 0 {
		return fmt.Errorf("expected no access audit, got %d attempts", s.adminPaymentCapture.auditAttempts)
	}
	return nil
}
func (s *testSuite) adminPaymentAudit(resource, operator, correlation string) error {
	if resource != "payment" {
		return fmt.Errorf("unexpected audit resource %q", resource)
	}
	e := s.adminPaymentCapture.lastEvent
	if e == nil || s.adminPaymentCapture.auditAttempts != 1 {
		return fmt.Errorf("expected exactly one captured audit event; event=%v attempts=%d", e, s.adminPaymentCapture.auditAttempts)
	}
	if e.Result() != audit.ResultPrepared || e.ResourceType() != resource || e.ResourceID() != "" || e.CorrelationID() != correlation || e.Reason() != nil || e.StateChange() != nil || e.ConversationID() != nil {
		return fmt.Errorf("unexpected payment audit metadata: result=%s resource=%s/%q correlation=%q", e.Result(), e.ResourceType(), e.ResourceID(), e.CorrelationID())
	}
	operatorID, err := s.userRepository.FindIDByEmail(operator)
	if err != nil {
		return err
	}
	if e.OperatorID() != operatorID {
		return fmt.Errorf("audit operator was not resolved: %q/%d", operator, e.OperatorID())
	}
	persisted, err := s.auditEvents.Reader.FindByID(context.Background(), e.ID())
	if err != nil {
		return fmt.Errorf("finding persisted payment audit: %w", err)
	}
	if persisted.ID() != e.ID() || persisted.OperatorID() != operatorID || persisted.Action() != audit.ActionAccess || persisted.ResourceType() != resource || persisted.ResourceID() != "" || persisted.Result() != audit.ResultPrepared || persisted.CorrelationID() != correlation || persisted.Reason() != nil || persisted.StateChange() != nil || persisted.ConversationID() != nil {
		return fmt.Errorf("persisted payment audit event does not match the prepared collection-access contract")
	}
	return nil
}
func (s *testSuite) adminPaymentAuditPages(resource string) error {
	if resource != "payment" || len(s.adminPayments.correlations) < 2 {
		return fmt.Errorf("expected one payment audit per successful page, got %v", s.adminPayments.correlations)
	}
	if s.adminPaymentCapture.auditAttempts != 2 || len(s.adminPaymentCapture.events) != 2 {
		return fmt.Errorf("expected exactly two audits for two successful pages, got %d", s.adminPaymentCapture.auditAttempts)
	}
	for i, event := range s.adminPaymentCapture.events {
		if event.ResourceType() != resource || event.Result() != audit.ResultPrepared || event.ResourceID() != "" || event.CorrelationID() != s.adminPayments.correlations[i] {
			return fmt.Errorf("unexpected audit event for page %d", i)
		}
		if _, err := s.auditEvents.Reader.FindByID(context.Background(), event.ID()); err != nil {
			return fmt.Errorf("page %d audit was not persisted: %w", i, err)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentAuditMetadata() error {
	e := s.adminPaymentCapture.lastEvent
	if e == nil {
		return fmt.Errorf("missing payment audit event")
	}
	if e.Reason() != nil || e.StateChange() != nil || e.ConversationID() != nil || e.ResourceID() != "" {
		return fmt.Errorf("audit event contains disallowed metadata")
	}
	return nil
}
func (s *testSuite) adminPaymentAuditOnce() error {
	if s.adminPaymentCapture.auditAttempts != 1 {
		return fmt.Errorf("expected one collection-level audit event, got %d", s.adminPaymentCapture.auditAttempts)
	}
	return nil
}
func (s *testSuite) adminPaymentNoMutation() error {
	after, err := s.adminPaymentPersistenceSnapshot()
	if err != nil {
		return err
	}
	if s.adminPayments.baseline == "" {
		return fmt.Errorf("missing pre-query persistence snapshot")
	}
	if after != s.adminPayments.baseline {
		return fmt.Errorf("administrative query mutated persisted payment evidence")
	}
	return nil
}

func (s *testSuite) adminPaymentPersistenceSnapshot() (string, error) {
	type snapshot struct{ Intents, Transactions, Proposals, Orders []string }
	out := snapshot{}
	for alias, intent := range s.adminPayments.intents {
		stored, err := s.paymentIntentRepository.FindByID(context.Background(), intent.ID)
		if err != nil {
			return "", err
		}
		checkout := ""
		if stored.CheckoutSession != nil {
			checkout = fmt.Sprintf("%s|%s|%s|%s", stored.CheckoutSession.ExternalID, stored.CheckoutSession.URL, stored.CheckoutSession.ExpiresOn.UTC().Format(time.RFC3339Nano), stored.CheckoutSession.CreatedOn.UTC().Format(time.RFC3339Nano))
		}
		out.Intents = append(out.Intents, fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%s|%s|%s|%s", alias, stored.ID, stored.Purpose, stored.Status, stored.SellerAmountCents, stored.PlatformFeeCents, stored.TotalAmountCents, stored.Currency, stored.CreatedOn.UTC().Format(time.RFC3339Nano), stored.UpdatedOn.UTC().Format(time.RFC3339Nano), checkout))
	}
	for _, external := range s.adminPayments.transactionIDs {
		tx, err := s.paymentTransactionRepository.FindByExternalID(context.Background(), paymentaccount.PaymentProvider("mercado_pago"), external)
		if err != nil {
			return "", err
		}
		out.Transactions = append(out.Transactions, fmt.Sprintf("%d|%s|%s|%s|%s|%s|%d|%s|%s|%s", tx.ID, tx.PaymentIntentID, tx.Processor, tx.ExternalPaymentID, tx.SellerAccountID, tx.Status, tx.AmountCents, tx.Currency, tx.VerifiedOn.UTC().Format(time.RFC3339Nano), tx.UpdatedOn.UTC().Format(time.RFC3339Nano)))
	}
	proposalRepo := repositories.NewServiceProposalRepository(s.database)
	for alias, id := range s.adminPayments.proposals {
		p, err := proposalRepo.FindByID(context.Background(), id)
		if err != nil {
			return "", err
		}
		t := p.BookingTerms
		out.Proposals = append(out.Proposals, fmt.Sprintf("%s|%d|%s|%s|%d|%d|%d|%d|%s|%s|%d|%d", alias, p.ID, p.Status, t.Currency(), t.ServiceTotalCents(), t.DepositCents(), t.PlatformFeeTotalCents(), t.PlatformFeeDueNowCents(), t.BookingPaymentDeadline().UTC().Format(time.RFC3339Nano), p.ScheduledOn.UTC().Format(time.RFC3339Nano), p.ConsumerID(), p.ProviderID()))
	}
	for alias, id := range s.adminPayments.orders {
		o, err := s.workOrderRepository.FindByID(context.Background(), id)
		if err != nil {
			return "", err
		}
		report := ""
		if r := o.CompletionReport(); r != nil {
			report = fmt.Sprintf("%d|%s|%s|%v", r.ID(), r.Description(), r.ReportedOn().UTC().Format(time.RFC3339Nano), r.ImageFileIDs())
		}
		out.Orders = append(out.Orders, fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s", alias, o.ID(), o.ServiceProposalID(), o.Status(), o.AcceptedOn().UTC().Format(time.RFC3339Nano), o.PaidOn().UTC().Format(time.RFC3339Nano), report))
	}
	for _, values := range [][]string{out.Intents, out.Transactions, out.Proposals, out.Orders} {
		sort.Strings(values)
	}
	encoded, err := json.Marshal(out)
	return string(encoded), err
}
func (s *testSuite) adminPaymentSpecificSearch(detail string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	switch {
	case strings.Contains(detail, "ID interno"):
		if len(got.Payments) != 1 {
			return fmt.Errorf("expected exact ID search result")
		}
	case strings.Contains(detail, "busca el UUID interno"):
		if len(got.Payments) != 1 {
			return fmt.Errorf("expected UUID reference result")
		}
	case strings.Contains(detail, "prefijo"):
		if len(got.Payments) != 0 {
			return fmt.Errorf("prefix must not match external ID")
		}
	case strings.Contains(detail, "ID externo"):
		if len(got.Payments) != 1 || len(got.Payments[0].Transactions) == 0 {
			return fmt.Errorf("expected matching transaction evidence")
		}
	case strings.Contains(detail, "conjunto de intentos"):
		if len(got.Payments) != 2 {
			return fmt.Errorf("expected proposal intent set")
		}
	}
	return nil
}
func (s *testSuite) adminPaymentBasicRows() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		if r.Transactions == nil {
			return fmt.Errorf("transactions must be non-null")
		}
	}
	return nil
}
func (s *testSuite) adminPaymentFilterCombination() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) != 1 {
		return fmt.Errorf("expected one row inside [from,to), got %d", len(got.Payments))
	}
	return nil
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type adminPaymentTestCapture struct {
	failRead      bool
	failAudit     bool
	readerCalls   int
	auditAttempts int
	lastEvent     *audit.Event
	events        []*audit.Event
}

func (c *adminPaymentTestCapture) reset() {
	c.failRead = false
	c.failAudit = false
	c.readerCalls = 0
	c.auditAttempts = 0
	c.lastEvent = nil
	c.events = nil
}

type adminPaymentReaderDecorator struct {
	inner   payment.AdminPaymentReader
	capture *adminPaymentTestCapture
}

func (d adminPaymentReaderDecorator) FindPage(ctx context.Context, q payment.AdminPaymentQuery) (*readmodel.AdminPaymentSnapshot, error) {
	d.capture.readerCalls++
	if d.capture.failRead {
		return nil, fmt.Errorf("injected admin payment read failure")
	}
	return d.inner.FindPage(ctx, q)
}

type adminPaymentAuditWriterDecorator struct {
	inner   audit.Writer
	capture *adminPaymentTestCapture
}

func (d adminPaymentAuditWriterDecorator) Save(ctx context.Context, event *audit.Event) error {
	d.capture.auditAttempts++
	d.capture.lastEvent = event
	d.capture.events = append(d.capture.events, event)
	if d.capture.failAudit {
		return fmt.Errorf("injected admin payment audit failure")
	}
	return d.inner.Save(ctx, event)
}

func (s *testSuite) adminPaymentTransactionAt(txAlias, alias, external, status string, amount int64, currency, verified string) error {
	when, e := time.Parse(time.RFC3339, verified)
	if e != nil {
		return e
	}
	if e = s.adminPaymentCreateAndMaybeOrderIntent(alias, "P1", "booking_deposit", "paid", s.clock.Now()); e != nil {
		return e
	}
	if err := s.adminPaymentSaveTransaction(alias, external, status, currency, amount, when); err != nil {
		return err
	}
	s.adminPayments.before["transaction_alias:"+txAlias] = external
	return nil
}
func (s *testSuite) adminPaymentPersistedTerms() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) == 0 {
		return fmt.Errorf("no payment projection")
	}
	for _, r := range got.Payments {
		b := r.Breakdown
		if b.Currency != "ARS" || b.ServiceTotalCents != 3_333_333 || b.DepositCents != 777_777 || b.PlatformFeeTotalCents != 555_555 || b.PlatformFeeDueNowCents != 123_457 {
			return fmt.Errorf("terms were not projected from persisted contract: %+v", b)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentNoTransactionsAlias(alias string) error {
	intent := s.adminPayments.intents[alias]
	if intent == nil {
		return fmt.Errorf("unknown intent %q", alias)
	}
	readerCallsBefore := s.adminPaymentCapture.readerCalls
	auditAttemptsBefore := s.adminPaymentCapture.auditAttempts
	snapshot, err := repositories.NewAdminPaymentReader(s.database).FindPage(
		s.scenarioContext,
		payment.AdminPaymentQuery{PaymentIntentID: intent.ID, Limit: 1},
	)
	if err != nil {
		return fmt.Errorf("reading persisted payment evidence for %q: %w", alias, err)
	}
	if len(snapshot.Payments) != 1 || snapshot.Payments[0].ID != intent.ID {
		return fmt.Errorf("expected persisted payment intent %q in administrative reader", alias)
	}
	if len(snapshot.Payments[0].Transactions) != 0 {
		return fmt.Errorf("expected no persisted transaction evidence for %q, got %d", alias, len(snapshot.Payments[0].Transactions))
	}
	if s.adminPaymentCapture.readerCalls != readerCallsBefore || s.adminPaymentCapture.auditAttempts != auditAttemptsBefore {
		return fmt.Errorf("persisted transaction assertion must not issue an HTTP read or audit event")
	}
	return nil
}
func (s *testSuite) adminPaymentAmountBreakdown(seller, fee, total int64, currency string) error {
	r, e := s.adminPaymentFindRow("I2")
	if e != nil {
		return e
	}
	if r.Currency != currency || r.SellerAmountCents != seller || r.PlatformFeeCents != fee || r.TotalAmountCents != total || seller+fee != total {
		return fmt.Errorf("unexpected attempt breakdown %+v", r)
	}
	return nil
}
func (s *testSuite) adminPaymentPendingBalance(proposal, order string) error {
	r, e := s.adminPaymentFindRow("I2")
	if e != nil {
		return e
	}
	if r.ServiceProposalID != s.adminPayments.proposals[proposal] || r.WorkOrderID == nil || *r.WorkOrderID != s.adminPayments.orders[order] {
		return fmt.Errorf("pending summary row is not proposal/order scoped")
	}
	if r.Summary.PendingAmount == nil || r.Summary.PendingAmount.Currency != "ARS" || r.Summary.PendingAmount.AmountCents != 2_987_654 {
		return fmt.Errorf("unexpected contractual pending amount: %+v", r.Summary.PendingAmount)
	}
	return nil
}
func (s *testSuite) adminPaymentProposalSummary(proposal, order string, approved int64, approvedCurrency string, pending int64, pendingCurrency string) error {
	r, e := s.adminPaymentFindRow("I2")
	if e != nil {
		return e
	}
	if r.ServiceProposalID != s.adminPayments.proposals[proposal] || r.WorkOrderID == nil || *r.WorkOrderID != s.adminPayments.orders[order] {
		return fmt.Errorf("summary row is not proposal/order scoped")
	}
	if e = assertApprovedAmount(r, approved, approvedCurrency); e != nil {
		return e
	}
	if r.Summary.PendingAmount == nil || r.Summary.PendingAmount.AmountCents != pending || r.Summary.PendingAmount.Currency != pendingCurrency {
		return fmt.Errorf("unexpected pending summary: %+v", r.Summary.PendingAmount)
	}
	return nil
}
func (s *testSuite) adminPaymentProposalSummaryScope(proposal, order, alias string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if r.ServiceProposalID != s.adminPayments.proposals[proposal] || r.WorkOrderID == nil || *r.WorkOrderID != s.adminPayments.orders[order] {
		return fmt.Errorf("summary is not scoped to proposal/order")
	}
	return nil
}
func (s *testSuite) adminPaymentExcludedAttempts(rejected, balance string) error {
	r, e := s.adminPaymentFindRow(balance)
	if e != nil {
		return e
	}
	if e = assertApprovedAmount(r, 901_234, "ARS"); e != nil {
		return e
	}
	for _, tx := range r.Transactions {
		if tx.Status == "approved" && tx.ExternalPaymentID == s.adminPayments.intents[rejected].ID {
			return fmt.Errorf("rejected attempt was counted as approved")
		}
	}
	return nil
}
func (s *testSuite) adminPaymentThreeAttempts() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	want := map[string]string{"I1": "expired", "I2": "rejected", "I3": "processing"}
	for alias, status := range want {
		r, e := s.adminPaymentFindRow(alias)
		if e != nil {
			return e
		}
		if r.IntentStatus != status || len(r.Transactions) > 0 && hasApproved(r.Transactions) {
			return fmt.Errorf("attempt %s state/evidence mismatch: %s %+v", alias, r.IntentStatus, r.Transactions)
		}
	}
	if len(got.Payments) != 3 {
		return fmt.Errorf("expected exactly three attempts, got %d", len(got.Payments))
	}
	return nil
}
func (s *testSuite) adminPaymentTwoStatuses(first, second, firstStatus, secondStatus string) error {
	for alias, status := range map[string]string{first: firstStatus, second: secondStatus} {
		r, e := s.adminPaymentFindRowByExternal(alias)
		if e != nil {
			return e
		}
		if len(r.Transactions) != 1 || r.Transactions[0].Status != status || len(r.Summary.ApprovedAmounts) > 0 {
			return fmt.Errorf("transaction %s incorrectly contributes or has wrong status", alias)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentFindRowByExternal(external string) (adminPaymentRow, error) {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return adminPaymentRow{}, e
	}
	for _, r := range got.Payments {
		for _, tx := range r.Transactions {
			if tx.ExternalPaymentID == external {
				return r, nil
			}
		}
	}
	return adminPaymentRow{}, fmt.Errorf("no transaction %q", external)
}
func hasApproved(txs []adminPaymentTransaction) bool {
	for _, t := range txs {
		if t.Status == "approved" {
			return true
		}
	}
	return false
}
func (s *testSuite) adminPaymentNoOrderLinkedRows(proposal string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	expectedProposal := s.adminPayments.proposals[proposal]
	consumer, e := s.userRepository.FindByAuthID(auth0IDForConsumerEmail("ana@example.com"))
	if e != nil {
		return e
	}
	provider, e := s.userRepository.FindByAuthID(auth0IDForProviderEmail("juan@example.com"))
	if e != nil {
		return e
	}
	for _, r := range got.Payments {
		if r.ServiceProposalID != expectedProposal || r.ConsumerID != consumer.ID() || r.ProviderID != provider.ID() || r.WorkOrderID != nil {
			return fmt.Errorf("payment row linkage mismatch: %+v", r)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentOrderedAliases(first, second string) error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if !equalStrings(idsOf(got.Payments), []string{s.adminPaymentID(first), s.adminPaymentID(second)}) {
		return fmt.Errorf("unexpected payment ordering: %v", idsOf(got.Payments))
	}
	if got.Payments[0].IntentStatus == got.Payments[1].IntentStatus || got.Payments[0].TotalAmountCents == got.Payments[1].TotalAmountCents && got.Payments[0].CreatedOn.Equal(got.Payments[1].CreatedOn) {
		return fmt.Errorf("rows do not preserve distinct attempt evidence")
	}
	return nil
}
func (s *testSuite) adminPaymentOnlyTransactionOnIntent(alias, external, other string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if len(r.Transactions) != 1 || r.Transactions[0].ExternalPaymentID != external {
		return fmt.Errorf("expected only transaction %s on %s", external, alias)
	}
	o, e := s.adminPaymentFindRow(other)
	if e != nil {
		return e
	}
	if len(o.Transactions) != 0 {
		return fmt.Errorf("transaction was attributed to unrelated intent %s", other)
	}
	return nil
}
func (s *testSuite) adminPaymentTransactionParent(alias string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if len(r.Transactions) != 2 {
		return fmt.Errorf("expected 2 transactions")
	}
	for _, tx := range r.Transactions {
		saved, e := s.paymentTransactionRepository.FindByExternalID(context.Background(), paymentaccount.PaymentProvider("mercado_pago"), tx.ExternalPaymentID)
		if e != nil {
			return e
		}
		if saved.PaymentIntentID != r.ID {
			return fmt.Errorf("transaction %s has another parent", tx.ExternalPaymentID)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentExcessFlag() error {
	r, e := s.adminPaymentFindRow("I1")
	if e != nil {
		return e
	}
	want := []string{"approved_amount_exceeds_intent_total"}
	for _, f := range want {
		found := false
		for _, x := range append(r.Anomalies, r.Summary.Anomalies...) {
			if x == f {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("missing excess flag %s", f)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentExactValues() error {
	r, e := s.adminPaymentFindRow("I1")
	if e != nil {
		return e
	}
	if r.Currency != "ARS" || r.TotalAmountCents != 901_234 {
		return fmt.Errorf("unexpected persisted amount %s/%d", r.Currency, r.TotalAmountCents)
	}
	for _, tx := range r.Transactions {
		if tx.AmountCents != 901_234 && tx.AmountCents != 901_235 {
			return fmt.Errorf("non-integral/persisted transaction amount %d", tx.AmountCents)
		}
	}
	return nil
}
func (s *testSuite) adminPaymentOneTransaction(alias, external string) error {
	r, e := s.adminPaymentFindRow(alias)
	if e != nil {
		return e
	}
	if len(r.Transactions) != 1 || r.Transactions[0].ExternalPaymentID != external {
		return fmt.Errorf("expected one transaction %s for %s, got %+v", external, alias, r.Transactions)
	}
	return nil
}
func (s *testSuite) adminPaymentEmailSearchContract() error {
	got, e := s.adminPaymentDecoded()
	if e != nil {
		return e
	}
	if len(got.Payments) != 1 {
		return fmt.Errorf("expected one case-insensitive partial email result, got %d", len(got.Payments))
	}
	c, e := s.userRepository.FindByAuthID(auth0IDForConsumerEmail("ana@example.com"))
	if e != nil {
		return e
	}
	p, e := s.userRepository.FindByAuthID(auth0IDForProviderEmail("juan@example.com"))
	if e != nil {
		return e
	}
	if got.Payments[0].ConsumerID != c.ID() || got.Payments[0].ProviderID != p.ID() {
		return fmt.Errorf("partial email filters matched wrong participants")
	}
	return nil
}
func (s *testSuite) adminPaymentDuplicateNotification(externalID string) error {
	intent := s.adminPayments.intents["I1"]
	if intent == nil {
		return fmt.Errorf("duplicate notification setup requires I1")
	}
	if err := s.prepareLinkedMercadoPagoAccount("mp-juan", "juan@example.com"); err != nil {
		return fmt.Errorf("prepare webhook payment account: %w", err)
	}
	external := payment.ExternalPayment{ID: externalID, SellerAccountID: "mp-juan", ExternalReference: intent.ID, Status: payment.ExternalPaymentStatusApproved, Currency: intent.Currency, AmountCents: intent.TotalAmountCents}
	if got := s.checkoutClient.AddPayment(external); got != externalID {
		return fmt.Errorf("fake processor registered ID %q, expected %q", got, externalID)
	}
	if err := s.sendMercadoPagoPaymentNotification(externalID); err != nil {
		return fmt.Errorf("send first payment notification: %w", err)
	}
	if err := s.sendMercadoPagoPaymentNotification(externalID); err != nil {
		return fmt.Errorf("send duplicate payment notification: %w", err)
	}
	return nil
}
