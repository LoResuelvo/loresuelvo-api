package repositories_test

import (
	"context"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
	"time"
)

func TestConsumerHistoryReaderGlobalSummarySurvivesFiltersAndPaymentRetries(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, juan := savedJobRequestParticipants(t, f.testContext)
	other := savedConsumerIDWithData(t, f.testContext, "auth0|history-other", "history.other@example.com", "Other", "Consumer")
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	first := f.proposal(t, request, base.Add(3*time.Hour), serviceproposal.StatusAccepted)
	later := f.proposal(t, request, base.Add(time.Hour), serviceproposal.StatusRejected)
	order := f.workOrder(t, first, base.Add(4*time.Hour), "paid")
	f.completionReport(t, order, base.Add(5*time.Hour))
	_, err := f.testContext.database.Exec(`UPDATE work_orders SET paid_on=$1 WHERE id=$2`, base.Add(6*time.Hour), order)
	require.NoError(t, err)
	foreign := f.jobRequest(t, other, juan, base, jobrequest.StatusPending)
	f.proposal(t, foreign, base, serviceproposal.StatusPending)
	for i := 1; i <= 3; i++ {
		_, err := f.testContext.database.Exec(`INSERT INTO payment_intents(id,service_proposal_id,purpose,currency,seller_amount_cents,platform_fee_cents,total_amount_cents,status,created_on,updated_on) VALUES($1,$2,'booking_deposit','ARS',100,10,110,'rejected',$3,$3)`, fmt.Sprintf("00000000-0000-4000-8000-%012d", i), first, base)
		require.NoError(t, err)
	}
	r := repositories.NewConsumerHistoryReader(f.testContext.database)
	h, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConsumerHistorySummary{JobRequests: 1, ServiceProposals: 2, WorkOrders: 1}, h.Summary)
	require.Len(t, h.Items, 4)
	require.False(t, h.HasMore)
	require.Equal(t, order, h.Items[0].ID)
	require.Equal(t, "work_order", h.Items[0].Type)
	require.Equal(t, &request.ID, h.Items[0].JobRequestID)
	require.Equal(t, first, h.Items[0].ServiceProposalID)
	require.Equal(t, base.Add(5*time.Hour), *h.Items[0].CompletionReportedOn)
	require.Equal(t, base.Add(6*time.Hour), *h.Items[0].BalancePaidOn)
	// First proposal is selected by minimum ID, not by its later creation date.
	require.Equal(t, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, h.Items[0].Operation)
	require.Equal(t, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, h.Items[1].Operation)
	require.Equal(t, operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: later}, h.Items[2].Operation)
	from := base.Add(time.Hour)
	to := base.Add(3 * time.Hour)
	filtered, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20, Type: "service_proposal", Status: "rejected", ProviderID: juan, From: &from, To: &to})
	require.NoError(t, err)
	require.Equal(t, h.Summary, filtered.Summary)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, later, filtered.Items[0].ID)
	empty, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20, ProviderID: 2147483647})
	require.NoError(t, err)
	require.Equal(t, h.Summary, empty.Summary)
	require.NotNil(t, empty.Items)
	require.Empty(t, empty.Items)
}
func TestConsumerHistoryReaderKeysetHandlesCrossTypeAndSameTypeTies(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, juan := savedJobRequestParticipants(t, f.testContext)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	first := f.proposal(t, request, base, serviceproposal.StatusAccepted)
	later := f.proposal(t, request, base, serviceproposal.StatusAccepted)
	firstOrder := f.workOrder(t, first, base, "scheduled")
	laterOrder := f.workOrder(t, later, base, "awaiting_payment")
	r := repositories.NewConsumerHistoryReader(f.testContext.database)
	q := admin.ConsumerHistoryQuery{Limit: 2}
	var got []readmodel.ConsumerHistoryPosition
	for n := 0; n < 5; n++ {
		h, err := r.FindByConsumerID(t.Context(), ana, q)
		require.NoError(t, err)
		require.LessOrEqual(t, len(h.Items), 2)
		for _, i := range h.Items {
			got = append(got, i.Position())
		}
		if !h.HasMore {
			break
		}
		require.Len(t, h.Items, 2)
		position := h.Items[len(h.Items)-1].Position()
		q.After = &position
	}
	require.Equal(t, []readmodel.ConsumerHistoryPosition{{OccurredOn: base, Type: "work_order", ID: laterOrder}, {OccurredOn: base, Type: "work_order", ID: firstOrder}, {OccurredOn: base, Type: "service_proposal", ID: later}, {OccurredOn: base, Type: "service_proposal", ID: first}, {OccurredOn: base, Type: "job_request", ID: request.ID}}, got)
}
func TestConsumerHistoryReaderPreservesOrphanProposalAndLegacyAbsentAddress(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, juan := savedJobRequestParticipants(t, f.testContext)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	proposal := f.proposal(t, request, base.Add(time.Hour), serviceproposal.StatusAccepted)
	order := f.workOrder(t, proposal, base.Add(2*time.Hour), "scheduled")
	_, err := f.testContext.database.Exec(`DELETE FROM job_requests WHERE id=$1`, request.ID)
	require.NoError(t, err)
	_, err = f.testContext.database.Exec(`DELETE FROM consumer_addresses WHERE consumer_id=$1`, ana)
	require.NoError(t, err)
	r := repositories.NewConsumerHistoryReader(f.testContext.database)
	h, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20})
	require.NoError(t, err)
	require.Nil(t, h.Address)
	require.Nil(t, h.CoverageZone)
	require.Empty(t, h.Consumer.ProfilePhotoFileID)
	require.Equal(t, readmodel.ConsumerHistorySummary{ServiceProposals: 1, WorkOrders: 1}, h.Summary)
	require.Len(t, h.Items, 2)
	require.Equal(t, order, h.Items[0].ID)
	for _, i := range h.Items {
		require.Nil(t, i.JobRequestID)
		require.Equal(t, operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: proposal}, i.Operation)
	}
	wrongRole, err := r.FindByConsumerID(t.Context(), juan, admin.ConsumerHistoryQuery{Limit: 20})
	require.NoError(t, err)
	require.Nil(t, wrongRole)
	missing, err := r.FindByConsumerID(t.Context(), 2147483647, admin.ConsumerHistoryQuery{Limit: 20})
	require.NoError(t, err)
	require.Nil(t, missing)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	h, err = r.FindByConsumerID(ctx, ana, admin.ConsumerHistoryQuery{Limit: 20})
	require.Nil(t, h)
	require.ErrorIs(t, err, context.Canceled)
}
func TestConsumerHistoryReaderReadsCurrentAddressWithProvenanceData(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, _ := savedJobRequestParticipants(t, f.testContext)
	_, err := f.testContext.database.Exec(`UPDATE consumer_addresses SET street='Rivadavia',street_number='5100',floor='4',unit='B' WHERE consumer_id=$1`, ana)
	require.NoError(t, err)
	var zoneID int
	var originalEnabled bool
	require.NoError(t, f.testContext.database.QueryRow(`SELECT z.id,z.enabled FROM coverage_zones z JOIN consumer_addresses a ON a.coverage_zone_id=z.id WHERE a.consumer_id=$1`, ana).Scan(&zoneID, &originalEnabled))
	t.Cleanup(func() {
		_, restoreErr := f.testContext.database.Exec(`UPDATE coverage_zones SET enabled=$1 WHERE id=$2`, originalEnabled, zoneID)
		require.NoError(t, restoreErr)
	})
	_, err = f.testContext.database.Exec(`UPDATE coverage_zones SET enabled=false WHERE id=$1`, zoneID)
	require.NoError(t, err)
	h, err := repositories.NewConsumerHistoryReader(f.testContext.database).FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20})
	require.NoError(t, err)
	require.NotNil(t, h.Address)
	require.Equal(t, "Rivadavia", h.Address.Street)
	require.Equal(t, "5100", h.Address.StreetNumber)
	require.Equal(t, "4", *h.Address.Floor)
	require.Equal(t, "B", *h.Address.Unit)
	require.NotNil(t, h.CoverageZone)
	require.False(t, h.CoverageZone.Enabled)
}

func TestConsumerHistoryReaderSQLSourceFailureDoesNotReturnEmptyHistory(t *testing.T) {
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	parsed, err := url.Parse(config.URL)
	require.NoError(t, err)
	values := parsed.Query()
	values.Set("search_path", "consumer_history_missing_test_schema")
	parsed.RawQuery = values.Encode()
	config.URL = parsed.String()
	database, err := db.ConnectPostgres(t.Context(), config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	h, err := repositories.NewConsumerHistoryReader(database).FindByConsumerID(t.Context(), 12, admin.ConsumerHistoryQuery{Limit: 20})
	require.Nil(t, h)
	require.ErrorContains(t, err, "reading consumer history profile")
	// This connection's path is isolated: no schema or shared database state was changed.
}

func TestConsumerHistoryReaderDoesNotMergeCollidingResourceIDs(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, juan := savedJobRequestParticipants(t, f.testContext)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	proposal := f.proposal(t, request, base, serviceproposal.StatusAccepted)
	_, err := f.testContext.database.Exec(`UPDATE service_proposals SET id=$1 WHERE id=$2`, request.ID, proposal)
	require.NoError(t, err)
	order := f.workOrder(t, request.ID, base, "scheduled")
	_, err = f.testContext.database.Exec(`UPDATE work_orders SET id=$1 WHERE id=$2`, request.ID, order)
	require.NoError(t, err)
	r := repositories.NewConsumerHistoryReader(f.testContext.database)
	q := admin.ConsumerHistoryQuery{Limit: 1}
	got := []string{}
	for n := 0; n < 4; n++ {
		h, err := r.FindByConsumerID(t.Context(), ana, q)
		require.NoError(t, err)
		require.Len(t, h.Items, 1)
		require.Equal(t, request.ID, h.Items[0].ID)
		got = append(got, h.Items[0].Type)
		if !h.HasMore {
			break
		}
		p := h.Items[0].Position()
		q.After = &p
	}
	require.Equal(t, []string{"work_order", "service_proposal", "job_request"}, got)
}

func TestConsumerHistoryReaderPreservesNanosecondWindowOnMicrosecondEvidence(t *testing.T) {
	f := newOperationInboxFixture(t)
	ana, juan := savedJobRequestParticipants(t, f.testContext)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	proposal := f.proposal(t, request, base.Add(time.Microsecond), serviceproposal.StatusAccepted)
	order := f.workOrder(t, proposal, base.Add(time.Microsecond), "scheduled")
	r := repositories.NewConsumerHistoryReader(f.testContext.database)
	from := base.Add(time.Nanosecond)
	to := base.Add(1501 * time.Nanosecond)
	q := admin.ConsumerHistoryQuery{Limit: 1, From: &from, To: &to}
	first, err := r.FindByConsumerID(t.Context(), ana, q)
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	require.True(t, first.HasMore)
	require.Equal(t, order, first.Items[0].ID)
	require.Equal(t, "work_order", first.Items[0].Type)
	position := first.Items[0].Position()
	q.After = &position
	require.NoError(t, q.Validate())
	next, err := r.FindByConsumerID(t.Context(), ana, q)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
	require.Equal(t, proposal, next.Items[0].ID)
	require.Equal(t, "service_proposal", next.Items[0].Type)
	require.False(t, next.HasMore)
	upper := base.Add(time.Nanosecond)
	onlyZero, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20, To: &upper})
	require.NoError(t, err)
	require.Len(t, onlyZero.Items, 1)
	require.Equal(t, request.ID, onlyZero.Items[0].ID)
	require.Equal(t, "job_request", onlyZero.Items[0].Type)
	// Aligned exclusive upper remains unchanged: a row exactly on it is excluded.
	aligned := base.Add(time.Microsecond)
	exact, err := r.FindByConsumerID(t.Context(), ana, admin.ConsumerHistoryQuery{Limit: 20, From: &base, To: &aligned})
	require.NoError(t, err)
	require.Len(t, exact.Items, 1)
	require.Equal(t, request.ID, exact.Items[0].ID)
}
