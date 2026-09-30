package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func (suite *testSuite) isCollectionScenario() bool {
	return suite.collectionStatistics != nil
}

func (suite *testSuite) providerStatisticsQueryDailyDateRange(start, end string) error {
	if suite.isCollectionScenario() {
		return suite.collectionQuery(start, end, "day", false, false)
	}
	return suite.activityQueryDailyDateRange(start, end)
}

func (suite *testSuite) providerStatisticsQueryWeeklyDateRange(start, end string) error {
	if suite.isCollectionScenario() {
		return suite.collectionQuery(start, end, "week", false, false)
	}
	return suite.activityQueryWeeklyDateRange(start, end)
}

func (suite *testSuite) providerStatisticsQueryMonthlyDateRange(start, end string) error {
	if suite.isCollectionScenario() {
		return suite.collectionQuery(start, end, "month", false, false)
	}
	return suite.activityQueryMonthlyDateRange(start, end)
}

func (suite *testSuite) providerStatisticsEvolutionMatches(table *godog.Table) error {
	if suite.isCollectionScenario() {
		return suite.collectionEvolutionMatches(table)
	}
	return suite.activityEvolutionMatches(table)
}

func (suite *testSuite) providerStatisticsNoOtherBuckets() error {
	if suite.isCollectionScenario() {
		return suite.collectionNoOtherBuckets()
	}
	return suite.activityNoOtherBuckets()
}

func (suite *testSuite) providerStatisticsComparisonPeriodIs(period string) error {
	if suite.isCollectionScenario() {
		return suite.collectionComparisonPeriodIs(period)
	}
	return suite.activityComparisonPeriodIs(period)
}

func registerGetCollectionStatisticsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que acordé con "([^"]+)" un trabajo de ARS ([0-9,.]+) con seña de ARS ([0-9,.]+) y comisión total de ARS ([0-9,.]+)$`, suite.collectionAgreedWork)
	sc.Step(`^que la seña se verificó el 2 de septiembre de 2026 por ARS ([0-9,.]+), incluidos ARS ([0-9,.]+) de comisión$`, suite.collectionDepositVerifiedOnSeptember2)
	sc.Step(`^que el saldo se verificó el 8 de septiembre de 2026 por ARS ([0-9,.]+), incluidos ARS ([0-9,.]+) de comisión$`, suite.collectionBalanceVerifiedOnSeptember8WithFee)
	sc.Step(`^que inicié el pago de la seña el (.+) y se verificó el (.+)$`, suite.collectionDepositStartedAndVerified)
	sc.Step(`^que el saldo se verificó el 8 de septiembre de 2026$`, suite.collectionBalanceVerifiedOnSeptember8)
	sc.Step(`^que tengo dos trabajos distintos con señas verificadas de ARS ([0-9,.]+) cada una el 2 y el 3 de septiembre de 2026$`, suite.collectionTwoSameAmountDeposits)
	sc.Step(`^que una de esas señas se aprobó en un nuevo intento después de uno rechazado el 1 de septiembre$`, suite.collectionRejectedBeforeApproved)
	sc.Step(`^que otra propuesta mía tiene un pago de seña iniciado el 2 de septiembre que sigue en procesamiento$`, suite.collectionProcessingDeposit)
	sc.Step(`^que otra propuesta mía tiene un pago de seña iniciado el 2 de septiembre sin cobro aprobado$`, suite.collectionUnpaidDeposit)
	sc.Step(`^que "([^"]+)" cobró una seña y un saldo entre el 1 y el 28 de septiembre de 2026$`, suite.collectionOtherProviderWasPaid)
	sc.Step(`^que yo cobré una seña de ARS ([0-9,.]+) entre el 1 y el 28 de septiembre de 2026$`, suite.collectionOwnDeposit)

	sc.Step(`^consulto mis cobros desde el (.+) hasta antes del (.+)$`, suite.collectionQueryDateRange)
	sc.Step(`^consulto mis cobros y su detalle desde el (.+) hasta antes del (.+)$`, suite.collectionQuerySummaryAndDetail)
	sc.Step(`^comparo mis cobros del (.+) con el período anterior$`, suite.collectionQueryWithComparison)

	sc.Step(`^veo ARS ([0-9,.]+) en señas, ARS ([0-9,.]+) en saldos y ARS ([0-9,.]+) en total, expresados en pesos argentinos y centavos enteros$`, suite.collectionTotalsWithCurrency)
	sc.Step(`^veo (ARS [0-9,.]+) en señas, (ARS [0-9,.]+) en saldos y (ARS [0-9,.]+) en total$`, suite.collectionTotals)
	sc.Step(`^el total coincide con la suma de señas y saldos$`, suite.collectionTotalAddsUp)
	sc.Step(`^no veo datos de la cuenta de cobros ni de los medios de pago$`, suite.collectionNoSensitiveData)
	sc.Step(`^veo exactamente ARS ([0-9,.]+) en señas y ARS ([0-9,.]+) en saldos$`, suite.collectionExactCategories)
	sc.Step(`^el detalle contiene solo dos cobros distintos de ARS ([0-9,.]+)$`, suite.collectionDetailHasTwoEqualPayments)
	sc.Step(`^solo veo mis ARS ([0-9,.]+) en el resumen y en el total del detalle$`, suite.collectionOnlyOwnTotal)
	sc.Step(`^ninguna fila del detalle pertenece a "([^"]+)"$`, suite.collectionDetailExcludesProvider)

	sc.Step(`^que cobré una seña de ARS ([0-9,.]+) el (.+)$`, suite.collectionDepositAt)
	sc.Step(`^que cobré un saldo de ARS ([0-9,.]+) el (.+), cuya seña cobré (.+)$`, suite.collectionBalanceAtWithPriorDeposit)
	sc.Step(`^que cobré otra seña el (.+)$`, suite.collectionAnotherDepositAt)
	sc.Step(`^la suma de esos días coincide con los importes del período$`, suite.collectionEvolutionAddsUp)
	sc.Step(`^veo la seña en el tramo del (.+) al (.+)$`, suite.collectionDepositBucketIs)
	sc.Step(`^veo el saldo en el tramo del (.+) al (.+)$`, suite.collectionBalanceBucketIs)
	sc.Step(`^veo agosto sin cobros y septiembre con ARS ([0-9,.]+) en señas y ARS ([0-9,.]+) en saldos$`, suite.collectionMonthlyBuckets)
	sc.Step(`^que cobré ARS ([0-9,.]+) en señas y ARS ([0-9,.]+) en saldos durante el (.+)$`, suite.collectionDepositAndBalanceInTwoDays)
	sc.Step(`^que no tuve cobros el 8 y 9 de septiembre de 2026$`, suite.collectionNoPreviousPayments)
	sc.Step(`^que cobré una seña de ARS ([0-9,.]+) durante el 10 y 11 de septiembre de 2026$`, suite.collectionDepositCurrentTwoDays)
	sc.Step(`^veo estas diferencias entre ambos períodos:$`, suite.collectionComparisonMatches)
	sc.Step(`^veo ARS ([0-9,.]+) más en señas y en total, y ARS ([0-9,.]+) de diferencia en saldos$`, suite.collectionZeroBaselineDifference)
	sc.Step(`^los cambios porcentuales de señas, saldos y total se muestran sin valor$`, suite.collectionZeroBaselinePercentages)

	sc.Step(`^que tengo un trabajo programado de ARS ([0-9,.]+) con seña de ARS ([0-9,.]+) pagada antes del 20 de septiembre de 2026$`, suite.collectionScheduledPendingWork)
	sc.Step(`^que informé la finalización de otro trabajo de ARS ([0-9,.]+) con seña de ARS ([0-9,.]+) pagada antes del 20 de septiembre, cuyo saldo aún no se pagó$`, suite.collectionAwaitingPendingWork)
	sc.Step(`^que las comisiones pactadas para esos trabajos suman ARS ([0-9,.]+)$`, suite.collectionPendingFeesAre)
	sc.Step(`^que no se inició el pago de los saldos de esos trabajos$`, suite.collectionPendingWithoutCheckout)
	sc.Step(`^que tengo un tercer trabajo completamente pagado antes del 20 de septiembre de 2026$`, suite.collectionPaidWorkOutsidePeriod)
	sc.Step(`^que "([^"]+)" tiene un trabajo finalizado con pago pendiente$`, suite.collectionOtherProviderAwaitingWork)
	sc.Step(`^veo (\d+) trabajo programado con ARS ([0-9,.]+) de saldo contractual pendiente$`, suite.collectionScheduledPendingIs)
	sc.Step(`^veo (\d+) trabajo finalizado con ARS ([0-9,.]+) de saldo contractual pendiente$`, suite.collectionAwaitingPendingIs)
	sc.Step(`^los cobros del período son cero, sin sumar comisiones ni pendientes de "([^"]+)"$`, suite.collectionPendingPeriodIsEmpty)

	sc.Step(`^que tengo tres cobros verificados entre el 1 y el 28 de septiembre de 2026: dos señas de ARS ([0-9,.]+) y un saldo de ARS ([0-9,.]+)$`, suite.collectionThreeVerifiedPayments)
	sc.Step(`^el detalle informa (\d+) cobros y ARS ([0-9,.]+) para el conjunto completo$`, suite.collectionDetailWholeSetIs)
	sc.Step(`^cada cobro muestra su identificador local, fecha de verificación, concepto, importe, moneda y propuesta$`, suite.collectionDetailRowsAreComplete)
	sc.Step(`^cada cobro muestra también el trabajo al que corresponde$`, suite.collectionDetailRowsHaveOrders)
	sc.Step(`^la suma de señas y saldos del resumen coincide con el total del detalle$`, suite.collectionSummaryAndDetailMatch)
	sc.Step(`^que tengo dos señas de ARS ([0-9,.]+) verificadas el 3 y el 5 de septiembre de 2026, y un saldo de ARS ([0-9,.]+) verificado el 6 de septiembre$`, suite.collectionThreeForPagination)
	sc.Step(`^recorro el detalle de señas desde el 1 hasta antes del 29 de septiembre de 2026, de a un cobro por página$`, suite.collectionWalkDepositPages)
	sc.Step(`^veo primero la seña más reciente y luego la anterior, sin repeticiones ni omisiones$`, suite.collectionPagesOrderIs)
	sc.Step(`^cada página mantiene (\d+) cobros y ARS ([0-9,.]+) para el conjunto filtrado$`, suite.collectionPageTotalsAre)
	sc.Step(`^no aparece el saldo en ninguna de esas páginas$`, suite.collectionPagesExcludeBalance)
	sc.Step(`^que tengo dos cobros verificados dentro de los últimos 30 días, uno el 30 de agosto de 2026 a las 13:00 y otro el 28 de septiembre de 2026$`, suite.collectionTwoDefaultWindowPayments)
	sc.Step(`^que obtuve la primera página del detalle de a un cobro por página sin elegir período$`, suite.collectionFirstDefaultPage)
	sc.Step(`^que el reloj avanzó un día$`, suite.collectionClockAdvancedOneDay)
	sc.Step(`^continúo a la siguiente página del detalle$`, suite.collectionContinueDefaultPage)
	sc.Step(`^la segunda página conserva el período de la primera y muestra el cobro restante$`, suite.collectionDefaultContinuationIsStable)
	sc.Step(`^que no tuve cobros verificados en los últimos 30 días$`, suite.collectionNoRecentPayments)
	sc.Step(`^que tengo un trabajo programado con ARS ([0-9,.]+) de saldo contractual pendiente$`, suite.collectionScheduledWithoutRecentCollections)
	sc.Step(`^consulto mis cobros y su detalle sin elegir un período$`, suite.collectionDefaultSummaryAndDetail)
	sc.Step(`^veo mis cobros desde el (.+) hasta el (.+)$`, suite.collectionDefaultPeriodMatches)
	sc.Step(`^veo señas, saldos y total en cero, y (\d+) días calendario sin movimiento en la evolución diaria$`, suite.collectionEmptyEvolution)
	sc.Step(`^veo el saldo pendiente del trabajo programado y ninguna comparación anterior$`, suite.collectionPendingWithNoComparison)
	sc.Step(`^el detalle muestra cero cobros, importe total cero y ninguna fila$`, suite.collectionEmptyDetail)

	sc.Step(`^intento consultar mis cobros (.+)$`, suite.collectionTryInvalidQuery)
	sc.Step(`^intento consultar (el resumen|el detalle)$`, suite.collectionTryWithoutIdentity)
	sc.Step(`^se me informa que la consulta no es válida y no se muestran cobros$`, suite.collectionInvalidQueryRejected)
	sc.Step(`^(se rechaza la consulta porque no inicié sesión|se rechaza la consulta porque mi sesión no es válida|se rechaza la consulta porque no soy prestador|se rechaza la consulta porque no tengo una cuenta en LoResuelvo) y no se muestran cobros$`, suite.collectionIdentityRejected)
}

func (suite *testSuite) collectionAgreedWork(consumerEmail, amountText, depositText, feeText string) error {
	amount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	fee, err := collectionCents(feeText)
	if err != nil {
		return err
	}
	return suite.createCollectionOrder("collection-agreed", consumerEmail, "juan@example.com", amount, deposit, fee, fee/5,
		collectionAt(2, 0, 10), collectionAt(6, 10, 0), collectionAt(8, 10, 0))
}

func (suite *testSuite) collectionDepositVerifiedOnSeptember2(grossText, feeText string) error {
	gross, err := collectionCents(grossText)
	if err != nil {
		return err
	}
	fee, err := collectionCents(feeText)
	if err != nil {
		return err
	}
	label := suite.collectionState().latestOrder
	fixture := suite.collectionState().orders[label]
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, fixture.proposalID)
	if err != nil {
		return err
	}
	if proposal.BookingTerms.DepositCents()+proposal.BookingTerms.PlatformFeeDueNowCents() != gross || proposal.BookingTerms.PlatformFeeDueNowCents() != fee {
		return fmt.Errorf("deposit payment does not match stated gross amount and commission")
	}
	return suite.collectionSavePayment(label, payment.PurposeBookingDeposit, collectionAt(2, 10, 0), time.Time{})
}

func (suite *testSuite) collectionBalanceVerifiedOnSeptember8WithFee(grossText, feeText string) error {
	gross, err := collectionCents(grossText)
	if err != nil {
		return err
	}
	fee, err := collectionCents(feeText)
	if err != nil {
		return err
	}
	label := suite.collectionState().latestOrder
	fixture := suite.collectionState().orders[label]
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, fixture.proposalID)
	if err != nil {
		return err
	}
	if proposal.BookingTerms.RemainingAmountDueCents() != gross || proposal.BookingTerms.RemainingPlatformFeeCents() != fee {
		return fmt.Errorf("balance payment does not match stated gross amount and commission")
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, collectionAt(8, 10, 0), time.Time{})
}

func (suite *testSuite) collectionDepositStartedAndVerified(startText, verifiedText string) error {
	start, err := collectionDate(startText)
	if err != nil {
		return err
	}
	verified, err := collectionDate(verifiedText)
	if err != nil {
		return err
	}
	if verified.Sub(start) != 20*time.Minute {
		return fmt.Errorf("deposit fixture did not preserve checkout-to-verification interval")
	}
	return suite.collectionSavePayment(suite.collectionState().latestOrder, payment.PurposeBookingDeposit, verified, start)
}

func (suite *testSuite) collectionBalanceVerifiedOnSeptember8() error {
	return suite.collectionSavePayment(suite.collectionState().latestOrder, payment.PurposeServiceBalance, collectionAt(8, 10, 0), time.Time{})
}

func (suite *testSuite) collectionTwoSameAmountDeposits(amountText string) error {
	deposit, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	for index, day := range []int{2, 3} {
		label := fmt.Sprintf("collection-equal-%d", index)
		at := collectionAt(day, 10, 0)
		if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit*5, deposit, deposit/2, deposit/10, at, time.Time{}, time.Time{}); err != nil {
			return err
		}
		if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, at, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) collectionRejectedBeforeApproved() error {
	fixture := suite.collectionState().orders["collection-equal-0"]
	if fixture.proposalID <= 0 {
		return fmt.Errorf("first equal collection fixture is missing")
	}
	return suite.collectionSaveUnapprovedIntent(fixture.proposalID, payment.StatusRejected, collectionAt(1, 10, 0))
}

func (suite *testSuite) collectionProcessingDeposit() error {
	proposalID, err := suite.collectionCreateProposalOnly("processing", "ana@example.com", "juan@example.com", 50000, 10000, 5000, 1000, collectionAt(2, 11, 0))
	if err != nil {
		return err
	}
	return suite.collectionSaveUnapprovedIntent(proposalID, payment.StatusProcessing, collectionAt(2, 11, 0))
}

func (suite *testSuite) collectionUnpaidDeposit() error {
	proposalID, err := suite.collectionCreateProposalOnly("unpaid", "ana@example.com", "juan@example.com", 50000, 10000, 5000, 1000, collectionAt(2, 12, 0))
	if err != nil {
		return err
	}
	return suite.collectionSaveUnapprovedIntent(proposalID, payment.StatusCheckoutReady, collectionAt(2, 12, 0))
}

func (suite *testSuite) collectionOtherProviderWasPaid(providerEmail string) error {
	label := "collection-foreign"
	if err := suite.createCollectionOrder(label, "ana@example.com", providerEmail, 40000, 10000, 5000, 1000,
		collectionAt(5, 10, 0), collectionAt(6, 10, 0), collectionAt(7, 10, 0)); err != nil {
		return err
	}
	if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, collectionAt(5, 10, 0), time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, collectionAt(7, 10, 0), time.Time{})
}

func (suite *testSuite) collectionOwnDeposit(amountText string) error {
	deposit, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	label := "collection-own"
	if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit*5, deposit, deposit/2, deposit/10,
		collectionAt(8, 10, 0), time.Time{}, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeBookingDeposit, collectionAt(8, 10, 0), time.Time{})
}

func (suite *testSuite) collectionQueryDateRange(start, end string) error {
	return suite.collectionQuery(start, end, "day", false, false)
}

func (suite *testSuite) collectionQuerySummaryAndDetail(start, end string) error {
	return suite.collectionQuery(start, end, "day", false, true)
}

func (suite *testSuite) collectionQueryWithComparison(period string) error {
	return suite.collectionQuery(period, "", "day", true, false)
}

func (suite *testSuite) collectionTotalsWithCurrency(depositText, balanceText, totalText string) error {
	if err := suite.collectionTotals("ARS "+depositText, "ARS "+balanceText, "ARS "+totalText); err != nil {
		return err
	}
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Currency != "ARS" || response.Period.TimeZone != "America/Argentina/Buenos_Aires" || response.CalculatedAt.IsZero() {
		return fmt.Errorf("collection summary omits currency, time zone or calculation instant")
	}
	return nil
}

func (suite *testSuite) collectionTotals(depositText, balanceText, totalText string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	total, err := collectionCents(totalText)
	if err != nil {
		return err
	}
	if response.Results != (collectionStatisticsAmounts{BookingDepositCents: deposit, ServiceBalanceCents: balance, TotalCents: total}) {
		return fmt.Errorf("collection totals are %+v, want deposit=%d balance=%d total=%d", response.Results, deposit, balance, total)
	}
	return nil
}

func (suite *testSuite) collectionTotalAddsUp() error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Results.TotalCents != response.Results.BookingDepositCents+response.Results.ServiceBalanceCents {
		return fmt.Errorf("collection total does not equal deposit plus balance")
	}
	return nil
}

func (suite *testSuite) collectionNoSensitiveData() error {
	if suite.collectionState().lastPath != collectionSummaryPath {
		return fmt.Errorf("sensitive-data assertion must inspect summary response")
	}
	for _, key := range []string{"external_payment_id", "seller_account_id", "access_token", "payment_method", "account_id"} {
		if strings.Contains(string(suite.lastBody), key) {
			return fmt.Errorf("collection summary contains sensitive field %q", key)
		}
	}
	return nil
}

func (suite *testSuite) collectionExactCategories(depositText, balanceText string) error {
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	return suite.collectionTotals(fmt.Sprintf("ARS %d,%02d", deposit/100, deposit%100), fmt.Sprintf("ARS %d,%02d", balance/100, balance%100), fmt.Sprintf("ARS %d,%02d", (deposit+balance)/100, (deposit+balance)%100))
}

func (suite *testSuite) collectionDetailHasTwoEqualPayments(amountText string) error {
	response, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	amount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	if response.TotalCount != 2 || response.TotalAmountCents != 2*amount || len(response.Transactions) != 2 || response.Transactions[0].ID == response.Transactions[1].ID {
		return fmt.Errorf("collection detail does not contain exactly two distinct payments of %d cents: %+v", amount, response)
	}
	for _, transaction := range response.Transactions {
		if transaction.SellerAmountCents != amount || transaction.Purpose != "booking_deposit" {
			return fmt.Errorf("unexpected collection transaction %+v", transaction)
		}
	}
	return nil
}

func (suite *testSuite) collectionOnlyOwnTotal(amountText string) error {
	amount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	summary, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if summary.Results.TotalCents != amount || detail.TotalAmountCents != amount || detail.TotalCount != 1 {
		return fmt.Errorf("collection own totals are summary=%d detail=%d count=%d, want %d/1", summary.Results.TotalCents, detail.TotalAmountCents, detail.TotalCount, amount)
	}
	return nil
}

func (suite *testSuite) collectionDetailExcludesProvider(email string) error {
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	foreignProposals := make(map[int]bool)
	for _, fixture := range suite.collectionState().orders {
		if fixture.provider == email {
			foreignProposals[fixture.proposalID] = true
		}
	}
	if len(foreignProposals) == 0 {
		return fmt.Errorf("no collection fixture exists for excluded provider %q", email)
	}
	for _, transaction := range detail.Transactions {
		if foreignProposals[transaction.ServiceProposalID] {
			return fmt.Errorf("collection detail exposed proposal %d of %q", transaction.ServiceProposalID, email)
		}
	}
	return nil
}

func (suite *testSuite) collectionDepositAt(amountText, dateText string) error {
	deposit, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	verified, err := collectionDate(dateText)
	if err != nil {
		return err
	}
	state := suite.collectionState()
	state.nextFixture++
	label := fmt.Sprintf("collection-deposit-%d", state.nextFixture)
	if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit*5, deposit, 0, 0, verified, time.Time{}, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeBookingDeposit, verified, time.Time{})
}

func (suite *testSuite) collectionBalanceAtWithPriorDeposit(balanceText, dateText, priorText string) error {
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	verified, err := collectionDate(dateText)
	if err != nil {
		return err
	}
	prior := collectionAugustAt(20, 10, 0)
	if strings.HasPrefix(priorText, "el ") {
		prior, err = collectionDate(priorText)
		if err != nil {
			return err
		}
	} else if strings.TrimSpace(priorText) != "en agosto" {
		return fmt.Errorf("unsupported earlier deposit date %q", priorText)
	}
	deposit := balance / 3
	if deposit <= 0 || balance+deposit <= 0 {
		return fmt.Errorf("invalid balance fixture amount %d", balance)
	}
	state := suite.collectionState()
	state.nextFixture++
	label := fmt.Sprintf("collection-balance-%d", state.nextFixture)
	if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", balance+deposit, deposit, 0, 0,
		prior, verified.Add(-2*time.Hour), verified); err != nil {
		return err
	}
	if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, prior, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, verified, time.Time{})
}

func (suite *testSuite) collectionAnotherDepositAt(dateText string) error {
	return suite.collectionDepositAt("100,00", dateText)
}

func (suite *testSuite) collectionEvolutionMatches(table *godog.Table) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(response.Evolution) != len(rows) {
		return fmt.Errorf("collection evolution has %d buckets, want %d", len(response.Evolution), len(rows))
	}
	for index, row := range rows {
		bucket := response.Evolution[index]
		start, err := collectionDate(row["día"] + " de 2026")
		if err != nil {
			return err
		}
		deposit, err := collectionCents(row["señas"])
		if err != nil {
			return err
		}
		balance, err := collectionCents(row["saldos"])
		if err != nil {
			return err
		}
		total, err := collectionCents(row["total"])
		if err != nil {
			return err
		}
		if !bucket.From.Equal(start) || !bucket.To.Equal(start.AddDate(0, 0, 1)) || bucket.BookingDepositCents != deposit || bucket.ServiceBalanceCents != balance || bucket.TotalCents != total {
			return fmt.Errorf("collection bucket %d is %+v, want %s and %d/%d/%d", index, bucket, row["día"], deposit, balance, total)
		}
	}
	return nil
}

func (suite *testSuite) collectionEvolutionAddsUp() error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	var deposit, balance, total int64
	for _, bucket := range response.Evolution {
		deposit += bucket.BookingDepositCents
		balance += bucket.ServiceBalanceCents
		total += bucket.TotalCents
	}
	if deposit != response.Results.BookingDepositCents || balance != response.Results.ServiceBalanceCents || total != response.Results.TotalCents {
		return fmt.Errorf("collection evolution %d/%d/%d disagrees with totals %+v", deposit, balance, total, response.Results)
	}
	return nil
}

func (suite *testSuite) collectionDepositBucketIs(startText, endText string) error {
	return suite.collectionBucketIs(0, startText, endText, 10000, 0)
}

func (suite *testSuite) collectionBalanceBucketIs(startText, endText string) error {
	return suite.collectionBucketIs(1, startText, endText, 0, 30000)
}

func (suite *testSuite) collectionBucketIs(index int, startText, endText string, deposit, balance int64) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if len(response.Evolution) <= index {
		return fmt.Errorf("missing collection bucket %d", index)
	}
	start, err := collectionDate(startText)
	if err != nil {
		return err
	}
	end, err := collectionDate(endText)
	if err != nil {
		return err
	}
	bucket := response.Evolution[index]
	if !bucket.From.Equal(start) || !bucket.To.Equal(end) || bucket.BookingDepositCents != deposit || bucket.ServiceBalanceCents != balance || bucket.TotalCents != deposit+balance {
		return fmt.Errorf("collection bucket %d is %+v, want %s–%s and %d/%d", index, bucket, startText, endText, deposit, balance)
	}
	return nil
}

func (suite *testSuite) collectionNoOtherBuckets() error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if len(response.Evolution) != 2 {
		return fmt.Errorf("collection weekly evolution has %d buckets, want two", len(response.Evolution))
	}
	return nil
}

func (suite *testSuite) collectionMonthlyBuckets(depositText, balanceText string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	if len(response.Evolution) != 2 {
		return fmt.Errorf("collection monthly evolution has %d buckets, want two", len(response.Evolution))
	}
	august, september := response.Evolution[0], response.Evolution[1]
	if !august.From.Equal(collectionAugustAt(31, 12, 0)) || !august.To.Equal(collectionAt(1, 0, 0)) || august.BookingDepositCents != 0 || august.ServiceBalanceCents != 0 || august.TotalCents != 0 {
		return fmt.Errorf("partial August collection bucket is %+v", august)
	}
	if !september.From.Equal(collectionAt(1, 0, 0)) || !september.To.Equal(collectionAt(8, 12, 0)) || september.BookingDepositCents != deposit || september.ServiceBalanceCents != balance || september.TotalCents != deposit+balance {
		return fmt.Errorf("partial September collection bucket is %+v", september)
	}
	return nil
}

func (suite *testSuite) collectionDepositAndBalanceInTwoDays(depositText, balanceText, periodText string) error {
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	from, _, err := activitySelectedPeriod(periodText)
	if err != nil {
		return err
	}
	accepted := from.Add(10 * time.Hour)
	reported := from.Add(36 * time.Hour)
	paid := from.Add(38 * time.Hour)
	state := suite.collectionState()
	state.nextFixture++
	label := fmt.Sprintf("collection-comparison-%d", state.nextFixture)
	if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit+balance, deposit, 0, 0, accepted, reported, paid); err != nil {
		return err
	}
	if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, accepted, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, paid, time.Time{})
}

func (suite *testSuite) collectionNoPreviousPayments() error {
	if len(suite.collectionState().orders) != 0 {
		return fmt.Errorf("previous period must begin without collection fixtures")
	}
	return nil
}

func (suite *testSuite) collectionDepositCurrentTwoDays(depositText string) error {
	return suite.collectionDepositAt(depositText, "10 de septiembre de 2026 a las 10:00")
}

func (suite *testSuite) collectionComparisonPeriodIs(periodText string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("collection comparison is absent")
	}
	from, to, err := activitySelectedPeriod(periodText)
	if err != nil {
		return err
	}
	if !response.Comparison.Period.From.Equal(from) || !response.Comparison.Period.To.Equal(to) {
		return fmt.Errorf("collection comparison period is %+v, want %s–%s", response.Comparison.Period, from, to)
	}
	return nil
}

func (suite *testSuite) collectionComparisonMatches(table *godog.Table) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("collection comparison is absent")
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 3 || len(response.Comparison.Changes) != 3 {
		return fmt.Errorf("collection comparison must contain exactly three metrics")
	}
	for _, row := range rows {
		key := map[string]string{"señas": "booking_deposit_cents", "saldos": "service_balance_cents", "total": "total_cents"}[row["concepto"]]
		if key == "" {
			return fmt.Errorf("unknown collection comparison concept %q", row["concepto"])
		}
		current, err := collectionCents(row["actual"])
		if err != nil {
			return err
		}
		previous, err := collectionCents(row["anterior"])
		if err != nil {
			return err
		}
		difference, err := collectionCents(row["diferencia"])
		if err != nil {
			return err
		}
		actual := map[string]int64{"booking_deposit_cents": response.Results.BookingDepositCents, "service_balance_cents": response.Results.ServiceBalanceCents, "total_cents": response.Results.TotalCents}[key]
		prior := map[string]int64{"booking_deposit_cents": response.Comparison.Results.BookingDepositCents, "service_balance_cents": response.Comparison.Results.ServiceBalanceCents, "total_cents": response.Comparison.Results.TotalCents}[key]
		change, exists := response.Comparison.Changes[key]
		percentText := strings.TrimSpace(strings.TrimSuffix(row["cambio porcentual"], "%"))
		wantPercent, err := strconv.ParseFloat(percentText, 64)
		if err != nil {
			return fmt.Errorf("invalid expected percentage %q: %w", row["cambio porcentual"], err)
		}
		if !exists || actual != current || prior != previous || change.Absolute != difference || change.Percentage == nil || *change.Percentage != wantPercent {
			return fmt.Errorf("collection comparison %s differs from expected current/prior/difference/percentage: %d/%d/%+v", key, actual, prior, change)
		}
	}
	return nil
}

func (suite *testSuite) collectionZeroBaselineDifference(depositText, balanceText string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil {
		return fmt.Errorf("collection comparison is absent")
	}
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	changes := response.Comparison.Changes
	if response.Comparison.Results != (collectionStatisticsAmounts{}) || changes["booking_deposit_cents"].Absolute != deposit || changes["service_balance_cents"].Absolute != balance || changes["total_cents"].Absolute != deposit || response.Results.TotalCents != deposit {
		return fmt.Errorf("collection zero-baseline difference is %+v with prior %+v", changes, response.Comparison.Results)
	}
	return nil
}

func (suite *testSuite) collectionZeroBaselinePercentages() error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Comparison == nil || len(response.Comparison.Changes) != 3 {
		return fmt.Errorf("collection comparison changes are incomplete")
	}
	for _, key := range []string{"booking_deposit_cents", "service_balance_cents", "total_cents"} {
		if response.Comparison.Changes[key].Percentage != nil {
			return fmt.Errorf("collection %s percentage has a value despite zero baseline", key)
		}
	}
	return nil
}

const collectionSummaryPath = "/providers/me/statistics/collections"
const collectionDetailPath = "/providers/me/statistics/collections/transactions"

type collectionStatisticsState struct {
	orders             map[string]collectionOrderFixture
	latestOrder        string
	nextFixture        int
	clockNow           time.Time
	summary            collectionStatisticsSummary
	detail             collectionStatisticsDetail
	pages              []collectionStatisticsDetail
	lastSummaryValid   bool
	lastDetailValid    bool
	firstDefaultPeriod collectionStatisticsPeriod
	firstDefaultCursor string
	lastQuery          url.Values
	lastPath           string
}

type collectionOrderFixture struct {
	proposalID int
	orderID    int
	provider   string
	consumer   string
	depositID  int
	balanceID  int
}

type collectionStatisticsPeriod struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Granularity string    `json:"granularity"`
	TimeZone    string    `json:"time_zone"`
}

type collectionStatisticsAmounts struct {
	BookingDepositCents int64 `json:"booking_deposit_cents"`
	ServiceBalanceCents int64 `json:"service_balance_cents"`
	TotalCents          int64 `json:"total_cents"`
}

type collectionStatisticsPendingBalance struct {
	Orders      int64 `json:"orders"`
	AmountCents int64 `json:"amount_cents"`
}

type collectionStatisticsSummary struct {
	Period       collectionStatisticsPeriod  `json:"period"`
	CalculatedAt time.Time                   `json:"calculated_at"`
	Currency     string                      `json:"currency"`
	Results      collectionStatisticsAmounts `json:"results"`
	Evolution    []struct {
		From                time.Time `json:"from"`
		To                  time.Time `json:"to"`
		BookingDepositCents int64     `json:"booking_deposit_cents"`
		ServiceBalanceCents int64     `json:"service_balance_cents"`
		TotalCents          int64     `json:"total_cents"`
	} `json:"evolution"`
	CurrentPending struct {
		Scheduled       collectionStatisticsPendingBalance `json:"scheduled"`
		AwaitingPayment collectionStatisticsPendingBalance `json:"awaiting_payment"`
	} `json:"current_pending"`
	Comparison *struct {
		Period  collectionStatisticsPeriod  `json:"period"`
		Results collectionStatisticsAmounts `json:"results"`
		Changes map[string]struct {
			Absolute   int64    `json:"absolute"`
			Percentage *float64 `json:"percentage"`
		} `json:"changes"`
	} `json:"comparison"`
}

type collectionStatisticsTransaction struct {
	ID                int       `json:"id"`
	VerifiedOn        time.Time `json:"verified_on"`
	Purpose           string    `json:"purpose"`
	SellerAmountCents int64     `json:"seller_amount_cents"`
	Currency          string    `json:"currency"`
	ServiceProposalID int       `json:"service_proposal_id"`
	WorkOrderID       *int      `json:"work_order_id"`
}

type collectionStatisticsDetail struct {
	Period           collectionStatisticsPeriod        `json:"period"`
	CalculatedAt     time.Time                         `json:"calculated_at"`
	Currency         string                            `json:"currency"`
	TotalCount       int64                             `json:"total_count"`
	TotalAmountCents int64                             `json:"total_amount_cents"`
	Transactions     []collectionStatisticsTransaction `json:"transactions"`
	NextCursor       *string                           `json:"next_cursor"`
}

func (suite *testSuite) collectionState() *collectionStatisticsState {
	if suite.collectionStatistics == nil {
		suite.collectionStatistics = &collectionStatisticsState{
			orders:   make(map[string]collectionOrderFixture),
			clockNow: mustActivityDateTime("2026-09-29 12:00"),
		}
	}
	return suite.collectionStatistics
}

func (suite *testSuite) createCollectionOrder(label, consumerEmail, providerEmail string, amount, deposit, fee, feeDueNow int64, acceptedOn, reportedOn, paidOn time.Time) error {
	if err := suite.createActivityOrderWithTerms(label, consumerEmail, providerEmail, amount, deposit, fee, feeDueNow, acceptedOn, reportedOn, paidOn, 1); err != nil {
		return err
	}
	proposalLabel := suite.activityState().proposalByOrder[label]
	proposal, exists := suite.operationInbox.proposals[proposalLabel]
	if !exists {
		return fmt.Errorf("collection proposal %q not stored", proposalLabel)
	}
	orderID, exists := suite.operationInbox.orders[label]
	if !exists || orderID <= 0 {
		return fmt.Errorf("collection order %q not stored", label)
	}
	state := suite.collectionState()
	state.orders[label] = collectionOrderFixture{proposalID: proposal.id, orderID: orderID, consumer: consumerEmail, provider: providerEmail}
	state.latestOrder = label
	return nil
}

func (suite *testSuite) collectionSavePayment(label string, purpose payment.Purpose, verifiedOn, startedOn time.Time) error {
	state := suite.collectionState()
	fixture, exists := state.orders[label]
	if !exists {
		return fmt.Errorf("collection order %q is not prepared", label)
	}
	if startedOn.IsZero() {
		startedOn = verifiedOn.Add(-5 * time.Minute)
	}
	ctx := suite.scenarioContext
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(ctx, fixture.proposalID)
	if err != nil {
		return fmt.Errorf("finding collection proposal %q: %w", label, err)
	}
	var intent *payment.Intent
	if purpose == payment.PurposeBookingDeposit {
		intent, err = payment.NewBookingDepositIntent(uuid.NewString(), fixture.proposalID, proposal.BookingTerms, startedOn)
	} else {
		order, findErr := suite.workOrderRepository.FindByID(ctx, fixture.orderID)
		if findErr != nil {
			return findErr
		}
		intent, err = payment.NewServiceBalanceIntent(uuid.NewString(), order, startedOn)
	}
	if err != nil {
		return fmt.Errorf("creating collection intent for %q: %w", label, err)
	}
	if err := intent.MarkCheckoutReady(uuid.NewString(), "https://www.mercadopago.com.ar/checkout/v1/redirect", startedOn.Add(30*time.Minute), startedOn); err != nil {
		return err
	}
	external := payment.ExternalPayment{
		ID: uuid.NewString(), SellerAccountID: "collection-test-seller-account",
		ExternalReference: intent.ID, Status: payment.ExternalPaymentStatusApproved,
		Currency: intent.Currency, AmountCents: intent.TotalAmountCents,
	}
	if err := intent.MarkPaid(external, verifiedOn); err != nil {
		return err
	}
	if err := suite.paymentIntentRepository.Save(ctx, intent); err != nil {
		return err
	}
	transaction, err := payment.NewTransaction(intent.ID, paymentaccount.PaymentProvider("mercado_pago"), external, verifiedOn)
	if err != nil {
		return err
	}
	if err := suite.paymentTransactionRepository.Save(ctx, transaction); err != nil {
		return err
	}
	if purpose == payment.PurposeBookingDeposit {
		fixture.depositID = transaction.ID
	} else {
		fixture.balanceID = transaction.ID
	}
	state.orders[label] = fixture
	return nil
}

func (suite *testSuite) collectionSaveUnapprovedIntent(proposalID int, status payment.IntentStatus, startedOn time.Time) error {
	proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, proposalID)
	if err != nil {
		return err
	}
	intent, err := payment.NewBookingDepositIntent(uuid.NewString(), proposalID, proposal.BookingTerms, startedOn)
	if err != nil {
		return err
	}
	if err := intent.MarkCheckoutReady(uuid.NewString(), "https://www.mercadopago.com.ar/checkout/v1/redirect", startedOn.Add(30*time.Minute), startedOn); err != nil {
		return err
	}
	if status == payment.StatusRejected || status == payment.StatusProcessing {
		externalStatus := payment.ExternalPaymentStatusRejected
		if status == payment.StatusProcessing {
			externalStatus = payment.ExternalPaymentStatusProcessing
		}
		external := payment.ExternalPayment{ID: uuid.NewString(), SellerAccountID: "collection-test-seller-account", ExternalReference: intent.ID, Status: externalStatus, Currency: intent.Currency, AmountCents: intent.TotalAmountCents}
		if status == payment.StatusRejected {
			err = intent.MarkRejected(external, startedOn.Add(time.Minute))
		} else {
			err = intent.MarkProcessing(external, startedOn.Add(time.Minute))
		}
		if err != nil {
			return err
		}
	}
	return suite.paymentIntentRepository.Save(suite.scenarioContext, intent)
}

func (suite *testSuite) collectionCreateProposalOnly(label, consumerEmail, providerEmail string, amount, deposit, fee, feeDueNow int64, createdOn time.Time) (int, error) {
	previousAuth, previousPermissions := suite.currentAuth0ID, suite.currentPermissions
	defer func() { suite.currentAuth0ID, suite.currentPermissions = previousAuth, previousPermissions }()
	activity := suite.activityState()
	requestKey := consumerEmail + "\x00" + providerEmail
	requestLabel := activity.requestByPair[requestKey]
	if requestLabel == "" {
		activity.nextFixture++
		requestLabel = fmt.Sprintf("collection-request-%d", activity.nextFixture)
		if err := suite.createInboxJobRequest(requestLabel, consumerEmail, providerEmail, createdOn.AddDate(0, 0, -367), "pending"); err != nil {
			return 0, err
		}
		if err := suite.setInboxFixtureClock(createdOn.AddDate(0, 0, -365)); err != nil {
			return 0, err
		}
		if err := suite.acceptInboxJobRequest(requestLabel); err != nil {
			return 0, err
		}
		activity.requestByPair[requestKey] = requestLabel
	}
	activity.nextFixture++
	proposalLabel := fmt.Sprintf("collection-proposal-%s-%d", label, activity.nextFixture)
	scheduledOn := createdOn.Add(48 * time.Hour)
	row := map[string]string{
		"propuesta": proposalLabel, "solicitud": requestLabel,
		"creada": createdOn.UTC().Format(time.RFC3339), "fecha programada": scheduledOn.UTC().Format(time.RFC3339),
		"duración": "60", "moneda": "ARS", "precio total": strconv.FormatInt(amount, 10),
		"seña": strconv.FormatInt(deposit, 10), "comisión total": strconv.FormatInt(fee, 10),
		"comisión inicial": strconv.FormatInt(feeDueNow, 10),
		"saldo servicio":   strconv.FormatInt(amount-deposit, 10), "saldo comisión": strconv.FormatInt(fee-feeDueNow, 10),
		"descripción": "Propuesta para estadísticas de cobros", "estado": "pending",
	}
	if err := suite.createDetailProposal(row); err != nil {
		return 0, err
	}
	return suite.operationInbox.proposals[proposalLabel].id, nil
}

func (suite *testSuite) collectionQuery(start, end, granularity string, compare, withDetail bool) error {
	query := make(url.Values)
	if compare {
		from, to, err := activitySelectedPeriod(start)
		if err != nil {
			return err
		}
		query.Set("from", from.Format(time.RFC3339))
		query.Set("to", to.Format(time.RFC3339))
	} else {
		if start != "" {
			from, err := activityQueryDate(start)
			if err != nil {
				return err
			}
			query.Set("from", from.Format(time.RFC3339))
		}
		if end != "" {
			to, err := activityQueryDate(end)
			if err != nil {
				return err
			}
			query.Set("to", to.Format(time.RFC3339))
		}
	}
	if granularity != "" && granularity != "day" {
		query.Set("granularity", granularity)
	}
	if compare {
		query.Set("compare_previous", "true")
	}
	if err := suite.collectionRequest(collectionSummaryPath, query); err != nil {
		return err
	}
	if withDetail {
		detailQuery := make(url.Values)
		for _, key := range []string{"from", "to"} {
			if query.Has(key) {
				detailQuery.Set(key, query.Get(key))
			}
		}
		return suite.collectionRequest(collectionDetailPath, detailQuery)
	}
	return nil
}

func (suite *testSuite) collectionRequest(path string, query url.Values) error {
	state := suite.collectionState()
	if path == collectionSummaryPath {
		state.lastSummaryValid = false
	} else {
		state.lastDetailValid = false
	}
	if err := suite.requestTestClockMock(state.clockNow.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting collection reference time: %w", err)
	}
	request, err := http.NewRequestWithContext(suite.scenarioContext, http.MethodGet, suite.server.URL+path+queryString(query), nil)
	if err != nil {
		return err
	}
	if suite.invalidSession {
		request.Header.Set("Authorization", "Bearer invalid.collection.token")
	} else if suite.currentAuth0ID != "" {
		request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	suite.lastStatus = response.StatusCode
	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	state.lastQuery, state.lastPath = query, path
	if response.StatusCode != http.StatusOK {
		return nil
	}
	if got := response.Header.Get("Cache-Control"); got != "private, no-store" {
		return fmt.Errorf("collection response cache control is %q", got)
	}
	if path == collectionSummaryPath {
		state.summary = collectionStatisticsSummary{}
		if err := json.Unmarshal(suite.lastBody, &state.summary); err != nil {
			return err
		}
		state.lastSummaryValid = true
	} else {
		state.detail = collectionStatisticsDetail{}
		if err := json.Unmarshal(suite.lastBody, &state.detail); err != nil {
			return err
		}
		state.lastDetailValid = true
	}
	return nil
}

func (suite *testSuite) collectionSummaryResponse() (collectionStatisticsSummary, error) {
	state := suite.collectionState()
	if !state.lastSummaryValid {
		return collectionStatisticsSummary{}, fmt.Errorf("collection summary was not read: %s", suite.lastBody)
	}
	return state.summary, nil
}

func (suite *testSuite) collectionDetailResponse() (collectionStatisticsDetail, error) {
	state := suite.collectionState()
	if !state.lastDetailValid {
		return collectionStatisticsDetail{}, fmt.Errorf("collection detail was not read: %s", suite.lastBody)
	}
	return state.detail, nil
}

func collectionCents(value string) (int64, error) {
	return activityCurrencyToCents("ARS " + strings.TrimPrefix(value, "ARS "))
}

func collectionDate(value string) (time.Time, error) {
	return activityQueryDate(value)
}

func collectionAt(day int, hour, minute int) time.Time {
	return mustActivityDateTime(fmt.Sprintf("2026-09-%02d %02d:%02d", day, hour, minute))
}

func collectionAugustAt(day int, hour, minute int) time.Time {
	return mustActivityDateTime(fmt.Sprintf("2026-08-%02d %02d:%02d", day, hour, minute))
}

func (suite *testSuite) collectionScheduledPendingWork(amountText, depositText string) error {
	return suite.collectionPendingOrder("pending-scheduled", "juan@example.com", amountText, depositText, 3000, 600, false)
}

func (suite *testSuite) collectionAwaitingPendingWork(amountText, depositText string) error {
	return suite.collectionPendingOrder("pending-awaiting", "juan@example.com", amountText, depositText, 2000, 400, true)
}

func (suite *testSuite) collectionPendingOrder(label, providerEmail, amountText, depositText string, fee, feeDueNow int64, awaiting bool) error {
	amount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	accepted := collectionAt(10, 10, 0)
	if label == "default-pending" {
		accepted = collectionAugustAt(20, 10, 0)
	}
	var reported time.Time
	if awaiting {
		reported = collectionAt(12, 10, 0)
	}
	if err := suite.createCollectionOrder(label, "ana@example.com", providerEmail, amount, deposit, fee, feeDueNow, accepted, reported, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeBookingDeposit, accepted, time.Time{})
}

func (suite *testSuite) collectionPendingFeesAre(amountText string) error {
	want, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	var total int64
	for _, label := range []string{"pending-scheduled", "pending-awaiting"} {
		fixture, exists := suite.collectionState().orders[label]
		if !exists {
			return fmt.Errorf("missing %s", label)
		}
		proposal, err := repositories.NewServiceProposalRepository(suite.database).FindByID(suite.scenarioContext, fixture.proposalID)
		if err != nil {
			return err
		}
		total += proposal.BookingTerms.PlatformFeeTotalCents()
	}
	if total != want {
		return fmt.Errorf("pending work fees total %d, want %d", total, want)
	}
	return nil
}

func (suite *testSuite) collectionPendingWithoutCheckout() error {
	for _, label := range []string{"pending-scheduled", "pending-awaiting"} {
		if suite.collectionState().orders[label].balanceID != 0 {
			return fmt.Errorf("%s has balance transaction", label)
		}
	}
	return nil
}

func (suite *testSuite) collectionPaidWorkOutsidePeriod() error {
	label := "pending-fully-paid"
	if err := suite.createCollectionOrder(label, "carla@example.com", "juan@example.com", 50000, 10000, 2000, 400, collectionAt(10, 9, 0), collectionAt(12, 9, 0), collectionAt(15, 9, 0)); err != nil {
		return err
	}
	if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, collectionAt(10, 9, 0), time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, collectionAt(15, 9, 0), time.Time{})
}

func (suite *testSuite) collectionOtherProviderAwaitingWork(email string) error {
	return suite.collectionPendingOrder("pending-foreign", email, "600,00", "100,00", 2000, 400, true)
}

func (suite *testSuite) collectionScheduledPendingIs(countText, amountText string) error {
	return suite.collectionPendingMatches("scheduled", countText, amountText)
}

func (suite *testSuite) collectionAwaitingPendingIs(countText, amountText string) error {
	return suite.collectionPendingMatches("awaiting_payment", countText, amountText)
}

func (suite *testSuite) collectionPendingMatches(kind, countText, amountText string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	wantCount, err := strconv.ParseInt(countText, 10, 64)
	if err != nil {
		return err
	}
	wantAmount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	got := response.CurrentPending.Scheduled
	if kind == "awaiting_payment" {
		got = response.CurrentPending.AwaitingPayment
	}
	if got.Orders != wantCount || got.AmountCents != wantAmount {
		return fmt.Errorf("%s pending balance %+v, want %d/%d", kind, got, wantCount, wantAmount)
	}
	return nil
}

func (suite *testSuite) collectionPendingPeriodIsEmpty(foreignEmail string) error {
	response, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if response.Results != (collectionStatisticsAmounts{}) || response.CurrentPending.Scheduled != (collectionStatisticsPendingBalance{Orders: 1, AmountCents: 80000}) || response.CurrentPending.AwaitingPayment != (collectionStatisticsPendingBalance{Orders: 1, AmountCents: 40000}) {
		return fmt.Errorf("current period or pending balances include fees or %s: results=%+v pending=%+v", foreignEmail, response.Results, response.CurrentPending)
	}
	return nil
}

func (suite *testSuite) collectionThreeVerifiedPayments(depositText, balanceText string) error {
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	for index, day := range []int{3, 5} {
		label := fmt.Sprintf("detail-deposit-%d", index)
		at := collectionAt(day, 10, 0)
		if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit*5, deposit, 0, 0, at, time.Time{}, time.Time{}); err != nil {
			return err
		}
		if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, at, time.Time{}); err != nil {
			return err
		}
	}
	return suite.collectionBalanceFixture("detail-balance", balance, collectionAt(6, 10, 0))
}

func (suite *testSuite) collectionBalanceFixture(label string, amount int64, verified time.Time) error {
	deposit := amount / 3
	accepted := collectionAugustAt(20, 10, 0)
	if err := suite.createCollectionOrder(label, "carla@example.com", "juan@example.com", deposit+amount, deposit, 0, 0, accepted, verified.Add(-time.Hour), verified); err != nil {
		return err
	}
	if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, accepted, time.Time{}); err != nil {
		return err
	}
	return suite.collectionSavePayment(label, payment.PurposeServiceBalance, verified, time.Time{})
}

func (suite *testSuite) collectionDetailWholeSetIs(countText, amountText string) error {
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	wantCount, err := strconv.ParseInt(countText, 10, 64)
	if err != nil {
		return err
	}
	wantAmount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	if detail.TotalCount != wantCount || detail.TotalAmountCents != wantAmount || int64(len(detail.Transactions)) != wantCount {
		return fmt.Errorf("collection detail count/amount/page = %d/%d/%d, want %d/%d", detail.TotalCount, detail.TotalAmountCents, len(detail.Transactions), wantCount, wantAmount)
	}
	return nil
}

func (suite *testSuite) collectionDetailRowsAreComplete() error {
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if len(detail.Transactions) == 0 {
		return fmt.Errorf("collection detail is empty")
	}
	seen := make(map[int]bool)
	for _, row := range detail.Transactions {
		if row.ID <= 0 || row.VerifiedOn.IsZero() || (row.Purpose != "booking_deposit" && row.Purpose != "service_balance") || row.SellerAmountCents <= 0 || row.Currency != "ARS" || row.ServiceProposalID <= 0 || seen[row.ID] {
			return fmt.Errorf("incomplete or repeated collection row %+v", row)
		}
		seen[row.ID] = true
	}
	return nil
}

func (suite *testSuite) collectionDetailRowsHaveOrders() error {
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	byProposal := make(map[int]int)
	for _, fixture := range suite.collectionState().orders {
		byProposal[fixture.proposalID] = fixture.orderID
	}
	for _, row := range detail.Transactions {
		if row.WorkOrderID == nil || *row.WorkOrderID != byProposal[row.ServiceProposalID] {
			return fmt.Errorf("collection row %d lacks its correct work order: %+v", row.ID, row)
		}
	}
	return nil
}

func (suite *testSuite) collectionSummaryAndDetailMatch() error {
	summary, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if summary.Results.BookingDepositCents+summary.Results.ServiceBalanceCents != detail.TotalAmountCents || summary.Results.TotalCents != detail.TotalAmountCents {
		return fmt.Errorf("summary %+v and detail %d disagree", summary.Results, detail.TotalAmountCents)
	}
	return nil
}

func (suite *testSuite) collectionThreeForPagination(depositText, balanceText string) error {
	deposit, err := collectionCents(depositText)
	if err != nil {
		return err
	}
	balance, err := collectionCents(balanceText)
	if err != nil {
		return err
	}
	for index, day := range []int{3, 5} {
		label := fmt.Sprintf("page-deposit-%d", index)
		at := collectionAt(day, 10, 0)
		if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", deposit*5, deposit, 0, 0, at, time.Time{}, time.Time{}); err != nil {
			return err
		}
		if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, at, time.Time{}); err != nil {
			return err
		}
	}
	return suite.collectionBalanceFixture("page-balance", balance, collectionAt(6, 10, 0))
}

func (suite *testSuite) collectionWalkDepositPages() error {
	from, to := collectionAt(1, 0, 0), collectionAt(29, 0, 0)
	query := url.Values{"from": {from.Format(time.RFC3339)}, "to": {to.Format(time.RFC3339)}, "purpose": {"booking_deposit"}, "limit": {"1"}}
	if err := suite.collectionRequest(collectionDetailPath, query); err != nil {
		return err
	}
	first, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK || first.NextCursor == nil {
		return fmt.Errorf("first deposit page lacks continuation: %s", suite.lastBody)
	}
	query = url.Values{"cursor": {*first.NextCursor}}
	if err := suite.collectionRequest(collectionDetailPath, query); err != nil {
		return err
	}
	second, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if second.NextCursor != nil {
		return fmt.Errorf("second deposit page unexpectedly continues")
	}
	suite.collectionState().pages = []collectionStatisticsDetail{first, second}
	return nil
}

func (suite *testSuite) collectionPagesOrderIs() error {
	pages := suite.collectionState().pages
	if len(pages) != 2 || len(pages[0].Transactions) != 1 || len(pages[1].Transactions) != 1 {
		return fmt.Errorf("expected two single-row pages, got %+v", pages)
	}
	first, second := pages[0].Transactions[0], pages[1].Transactions[0]
	orders := suite.collectionState().orders
	if first.ID != orders["page-deposit-1"].depositID || second.ID != orders["page-deposit-0"].depositID || first.ID == second.ID || !first.VerifiedOn.After(second.VerifiedOn) {
		return fmt.Errorf("deposit pages are out of order or duplicated: %+v, %+v", first, second)
	}
	return nil
}

func (suite *testSuite) collectionPageTotalsAre(countText, amountText string) error {
	wantCount, err := strconv.ParseInt(countText, 10, 64)
	if err != nil {
		return err
	}
	wantAmount, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	if len(suite.collectionState().pages) != 2 {
		return fmt.Errorf("no deposit pages")
	}
	for index, page := range suite.collectionState().pages {
		if page.TotalCount != wantCount || page.TotalAmountCents != wantAmount || !page.Period.From.Equal(collectionAt(1, 0, 0)) || !page.Period.To.Equal(collectionAt(29, 0, 0)) {
			return fmt.Errorf("page %d totals/period %+v", index, page)
		}
	}
	return nil
}

func (suite *testSuite) collectionPagesExcludeBalance() error {
	for _, page := range suite.collectionState().pages {
		for _, row := range page.Transactions {
			if row.Purpose != "booking_deposit" || row.ID == suite.collectionState().orders["page-balance"].balanceID {
				return fmt.Errorf("balance in deposit page: %+v", row)
			}
		}
	}
	return nil
}

func (suite *testSuite) collectionTwoDefaultWindowPayments() error {
	for index, at := range []time.Time{collectionAugustAt(30, 13, 0), collectionAt(28, 10, 0)} {
		label := fmt.Sprintf("default-payment-%d", index)
		if err := suite.createCollectionOrder(label, "ana@example.com", "juan@example.com", 50000, 10000, 0, 0, at, time.Time{}, time.Time{}); err != nil {
			return err
		}
		if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, at, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) collectionFirstDefaultPage() error {
	if err := suite.collectionRequest(collectionDetailPath, url.Values{"limit": {"1"}}); err != nil {
		return err
	}
	page, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if page.NextCursor == nil || len(page.Transactions) != 1 {
		return fmt.Errorf("default first page has no continuation: %+v", page)
	}
	state := suite.collectionState()
	state.firstDefaultPeriod, state.firstDefaultCursor = page.Period, *page.NextCursor
	state.pages = []collectionStatisticsDetail{page}
	return nil
}

func (suite *testSuite) collectionClockAdvancedOneDay() error {
	suite.collectionState().clockNow = suite.collectionState().clockNow.AddDate(0, 0, 1)
	return nil
}

func (suite *testSuite) collectionContinueDefaultPage() error {
	cursor := suite.collectionState().firstDefaultCursor
	if cursor == "" {
		return fmt.Errorf("missing first default cursor")
	}
	if err := suite.collectionRequest(collectionDetailPath, url.Values{"cursor": {cursor}}); err != nil {
		return err
	}
	page, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	suite.collectionState().pages = append(suite.collectionState().pages, page)
	return nil
}

func (suite *testSuite) collectionDefaultContinuationIsStable() error {
	state := suite.collectionState()
	if len(state.pages) != 2 || len(state.pages[1].Transactions) != 1 {
		return fmt.Errorf("default continuation lacks remaining row: %+v", state.pages)
	}
	first, second := state.pages[0], state.pages[1]
	if !first.Period.From.Equal(second.Period.From) || !first.Period.To.Equal(second.Period.To) || !first.Period.From.Equal(collectionAugustAt(30, 12, 0)) || !first.Period.To.Equal(collectionAt(29, 12, 0)) || second.Transactions[0].ID != state.orders["default-payment-0"].depositID || first.Transactions[0].ID != state.orders["default-payment-1"].depositID || second.NextCursor != nil {
		return fmt.Errorf("default continuation changed period or rows: first=%+v second=%+v", first, second)
	}
	return nil
}

func (suite *testSuite) collectionNoRecentPayments() error {
	if len(suite.collectionState().orders) != 0 {
		return fmt.Errorf("recent payment fixtures already exist")
	}
	return nil
}

func (suite *testSuite) collectionScheduledWithoutRecentCollections(amountText string) error {
	want, err := collectionCents(amountText)
	if err != nil {
		return err
	}
	if want != 80000 {
		return fmt.Errorf("unsupported scheduled balance fixture %d", want)
	}
	return suite.collectionPendingOrder("default-pending", "juan@example.com", "1.000,00", "200,00", 0, 0, false)
}

func (suite *testSuite) collectionDefaultSummaryAndDetail() error {
	if err := suite.collectionRequest(collectionSummaryPath, nil); err != nil {
		return err
	}
	return suite.collectionRequest(collectionDetailPath, nil)
}

func (suite *testSuite) collectionDefaultPeriodMatches(startText, endText string) error {
	wantFrom, err := collectionDate(startText)
	if err != nil {
		return err
	}
	wantTo, err := collectionDate(endText)
	if err != nil {
		return err
	}
	summary, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if !summary.Period.From.Equal(wantFrom) || !summary.Period.To.Equal(wantTo) || !detail.Period.From.Equal(wantFrom) || !detail.Period.To.Equal(wantTo) {
		return fmt.Errorf("default summary/detail periods are %+v/%+v, want %s–%s", summary.Period, detail.Period, wantFrom, wantTo)
	}
	return nil
}

func (suite *testSuite) collectionEmptyEvolution(dayCount string) error {
	summary, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	wantCount, err := strconv.Atoi(dayCount)
	if err != nil {
		return err
	}
	if summary.Results != (collectionStatisticsAmounts{}) || len(summary.Evolution) != wantCount {
		return fmt.Errorf("default collection results/evolution %+v/%d, want zero/%d", summary.Results, len(summary.Evolution), wantCount)
	}
	for _, bucket := range summary.Evolution {
		if bucket.BookingDepositCents != 0 || bucket.ServiceBalanceCents != 0 || bucket.TotalCents != 0 {
			return fmt.Errorf("nonempty default collection day %+v", bucket)
		}
	}
	return nil
}

func (suite *testSuite) collectionPendingWithNoComparison() error {
	summary, err := suite.collectionSummaryResponse()
	if err != nil {
		return err
	}
	if summary.CurrentPending.Scheduled != (collectionStatisticsPendingBalance{Orders: 1, AmountCents: 80000}) || summary.CurrentPending.AwaitingPayment != (collectionStatisticsPendingBalance{}) || summary.Comparison != nil {
		return fmt.Errorf("default pending/comparison %+v/%+v", summary.CurrentPending, summary.Comparison)
	}
	return nil
}

func (suite *testSuite) collectionEmptyDetail() error {
	detail, err := suite.collectionDetailResponse()
	if err != nil {
		return err
	}
	if detail.TotalCount != 0 || detail.TotalAmountCents != 0 || len(detail.Transactions) != 0 || detail.NextCursor != nil {
		return fmt.Errorf("default empty detail %+v", detail)
	}
	return nil
}

func (suite *testSuite) collectionTryInvalidQuery(choice string) error {
	from, to := collectionAt(1, 0, 0), collectionAt(2, 0, 0)
	query := url.Values{"from": {from.Format(time.RFC3339)}, "to": {to.Format(time.RFC3339)}}
	path := collectionSummaryPath
	switch choice {
	case "sin indicar el comienzo":
		query.Del("from")
	case "sin indicar el final":
		query.Del("to")
	case "con el mismo comienzo y final":
		query.Set("to", from.Format(time.RFC3339))
	case "con un final anterior al comienzo":
		query.Set("to", collectionAugustAt(31, 0, 0).Format(time.RFC3339))
	case "por más de 365 días":
		query.Set("from", mustActivityDateTime("2025-08-01 00:00").Format(time.RFC3339))
	case "con un final posterior al momento actual":
		query.Set("to", suite.collectionState().clockNow.Add(time.Minute).Format(time.RFC3339))
	case "con una agrupación desconocida":
		query.Set("granularity", "fortnight")
	case "con un concepto desconocido en el detalle":
		path = collectionDetailPath
		query.Set("purpose", "platform_fee")
	case "con un tamaño de página mayor al permitido en el detalle":
		path = collectionDetailPath
		query.Set("limit", "101")
	case "con una continuación alterada en el detalle":
		path = collectionDetailPath
		query = url.Values{"cursor": {"forged.collection.cursor"}}
	case "con una continuación de otro prestador en el detalle":
		path = collectionDetailPath
		cursor, err := suite.collectionForeignCursor()
		if err != nil {
			return err
		}
		query = url.Values{"cursor": {cursor}}
	case "con una continuación combinada con opciones incompatibles en el detalle":
		path = collectionDetailPath
		cursor, err := suite.collectionOwnCursor()
		if err != nil {
			return err
		}
		query = url.Values{"cursor": {cursor}, "limit": {"2"}}
	default:
		return fmt.Errorf("unknown invalid collection query choice %q", choice)
	}
	return suite.collectionRequest(path, query)
}

func (suite *testSuite) collectionForeignCursor() (string, error) {
	cursor, err := suite.collectionCursorForProvider("pedro@example.com")
	if err != nil {
		return "", err
	}
	return cursor, suite.iAmAuthenticatedAsProvider("juan@example.com")
}

func (suite *testSuite) collectionOwnCursor() (string, error) {
	return suite.collectionCursorForProvider("juan@example.com")
}

func (suite *testSuite) collectionCursorForProvider(email string) (string, error) {
	for index, day := range []int{3, 5} {
		label := fmt.Sprintf("invalid-cursor-%d", index)
		at := collectionAt(day, 10, 0)
		if err := suite.createCollectionOrder(label, "ana@example.com", email, 50000, 10000, 0, 0, at, time.Time{}, time.Time{}); err != nil {
			return "", err
		}
		if err := suite.collectionSavePayment(label, payment.PurposeBookingDeposit, at, time.Time{}); err != nil {
			return "", err
		}
	}
	if err := suite.iAmAuthenticatedAsProvider(email); err != nil {
		return "", err
	}
	query := url.Values{"from": {collectionAt(1, 0, 0).Format(time.RFC3339)}, "to": {collectionAt(29, 0, 0).Format(time.RFC3339)}, "limit": {"1"}}
	if err := suite.collectionRequest(collectionDetailPath, query); err != nil {
		return "", err
	}
	page, err := suite.collectionDetailResponse()
	if err != nil {
		return "", err
	}
	if page.NextCursor == nil {
		return "", fmt.Errorf("cursor fixture did not produce a second page: %+v", page)
	}
	return *page.NextCursor, nil
}

func (suite *testSuite) collectionTryWithoutIdentity(view string) error {
	path := collectionSummaryPath
	if view == "el detalle" {
		path = collectionDetailPath
	}
	return suite.collectionRequest(path, nil)
}

func (suite *testSuite) collectionInvalidQueryRejected() error {
	if suite.lastStatus != http.StatusBadRequest || strings.Contains(string(suite.lastBody), `"results"`) || strings.Contains(string(suite.lastBody), `"transactions"`) {
		return fmt.Errorf("invalid collection query returned status %d or leaked results: %s", suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) collectionIdentityRejected(message string) error {
	want := http.StatusUnauthorized
	if strings.Contains(message, "no soy prestador") {
		want = http.StatusForbidden
	}
	if strings.Contains(message, "no tengo una cuenta") {
		want = http.StatusNotFound
	}
	if suite.lastStatus != want || strings.Contains(string(suite.lastBody), `"results"`) || strings.Contains(string(suite.lastBody), `"transactions"`) {
		return fmt.Errorf("collection identity rejection status %d, want %d or leaked results: %s", suite.lastStatus, want, suite.lastBody)
	}
	return nil
}
