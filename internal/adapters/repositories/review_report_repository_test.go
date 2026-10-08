package repositories_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestReviewReportRepositoryRoundTrip(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	repo := repositories.NewReviewReportRepository(fixture.database)
	at := time.Date(2026, 8, 17, 15, 0, 0, 0, time.FixedZone("AR", -3*3600))
	report, err := workorder.NewReviewReport(order.ID(), order.ServiceProposal().ProviderID(), "personal_data", " private ", at)
	require.NoError(t, err)
	require.NoError(t, repo.Save(t.Context(), report))
	require.Positive(t, report.ID())
	found, err := repo.FindByWorkOrderID(t.Context(), order.ID())
	require.NoError(t, err)
	require.Equal(t, report, found)
	require.Equal(t, time.UTC, found.CreatedOn().Location())
}
func TestReviewReportRepositoryConcurrentLifetimeUniqueness(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	repo := repositories.NewReviewReportRepository(fixture.database)
	const attempts = 8
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range attempts {
		wg.Go(func() {
			<-start
			r, e := workorder.NewReviewReport(order.ID(), order.ServiceProposal().ProviderID(), "spam_advertising", "", time.Now())
			if e == nil {
				e = repo.Save(t.Context(), r)
			}
			results <- e
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, workorder.ErrReviewReportAlreadyExists)
		}
	}
	require.Equal(t, 1, success)
	var count int
	require.NoError(t, fixture.database.QueryRow(`SELECT count(*) FROM review_reports WHERE work_order_id=$1`, order.ID()).Scan(&count))
	require.Equal(t, 1, count)
	_, err = fixture.database.Exec(`UPDATE review_reports SET status='dismissed' WHERE work_order_id=$1`, order.ID())
	require.NoError(t, err)
	report, err := workorder.NewReviewReport(order.ID(), order.ServiceProposal().ProviderID(), "personal_data", "", time.Now())
	require.NoError(t, err)
	require.ErrorIs(t, repo.Save(t.Context(), report), workorder.ErrReviewReportAlreadyExists)
}
func TestWorkOrderRepositoryPreservesHiddenReview(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	_, err = fixture.database.Exec(`UPDATE work_order_reviews SET visible=false WHERE work_order_id=$1`, order.ID())
	require.NoError(t, err)
	found, err := fixture.workOrderRepository.FindByID(t.Context(), order.ID())
	require.NoError(t, err)
	require.False(t, found.Review().Visible())
	_, err = fixture.workOrderRepository.Save(t.Context(), found)
	require.NoError(t, err)
	again, err := fixture.workOrderRepository.FindByID(t.Context(), order.ID())
	require.NoError(t, err)
	require.False(t, again.Review().Visible())
	_, err = again.NewReviewReport(again.ServiceProposal().ProviderID(), "personal_data", "", time.Now())
	require.ErrorIs(t, err, workorder.ErrReviewNotAvailable)
}
func TestReviewReportRepositoryMissingAndForeignKeyErrors(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	repo := repositories.NewReviewReportRepository(fixture.database)
	_, err := repo.FindByWorkOrderID(t.Context(), 999999)
	require.ErrorIs(t, err, workorder.ErrReviewReportNotFound)
	report, err := workorder.NewReviewReport(999999, 999999, "personal_data", "", time.Now())
	require.NoError(t, err)
	err = repo.Save(t.Context(), report)
	require.Error(t, err)
	require.False(t, errors.Is(err, workorder.ErrReviewReportAlreadyExists))
	require.Zero(t, report.ID())
}

func TestReportReviewServiceDoesNotChangeReviewOrReputation(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, review := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	repo := repositories.NewReviewReportRepository(fixture.database)
	service := workorder.NewReportReviewService(repositories.NewProviderActivityActorFinder(fixture.database), fixture.workOrderRepository, repositories.NewReviewUnitOfWork(fixture.database, repositories.NewReviewModerationRepository(), repo, repositories.NewAuditEventRepository(fixture.database)), clockadapter.NewSystemClock())
	before, err := fixture.workOrderRepository.FindRatingStatsByProviderID(t.Context(), order.ServiceProposal().ProviderID())
	require.NoError(t, err)
	report, err := service.Report(t.Context(), "auth0|review-repository-provider", order.ID(), "personal_data", "secret explanation")
	require.NoError(t, err)
	require.Positive(t, report.ID())
	found, err := fixture.workOrderRepository.FindByID(t.Context(), order.ID())
	require.NoError(t, err)
	require.Equal(t, review, found.Review())
	require.True(t, found.Review().Visible())
	after, err := fixture.workOrderRepository.FindRatingStatsByProviderID(t.Context(), order.ServiceProposal().ProviderID())
	require.NoError(t, err)
	require.Equal(t, before, after)
	// A receipt does not permit a second consumer assessment.
	author, ok := found.ServiceProposal().ServiceProposalConsumer().(*consumer.Consumer)
	require.True(t, ok)
	require.ErrorIs(t, found.AddReview(author, review), workorder.ErrReviewAlreadyExists)
}
func TestReportReviewServiceKeepsLifetimeConflictAfterHandlingAndHiding(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	repo := repositories.NewReviewReportRepository(fixture.database)
	service := workorder.NewReportReviewService(repositories.NewProviderActivityActorFinder(fixture.database), fixture.workOrderRepository, repositories.NewReviewUnitOfWork(fixture.database, repositories.NewReviewModerationRepository(), repo, repositories.NewAuditEventRepository(fixture.database)), clockadapter.NewSystemClock())
	original, err := service.Report(t.Context(), "auth0|review-repository-provider", order.ID(), "personal_data", "private text")
	require.NoError(t, err)
	_, err = fixture.database.Exec(`UPDATE review_reports SET status='upheld' WHERE work_order_id=$1`, order.ID())
	require.NoError(t, err)
	_, err = fixture.database.Exec(`UPDATE work_order_reviews SET visible=false WHERE work_order_id=$1`, order.ID())
	require.NoError(t, err)
	_, err = service.Report(t.Context(), "auth0|review-repository-provider", order.ID(), "spam_advertising", "new text")
	require.ErrorIs(t, err, workorder.ErrReviewReportAlreadyExists)
	stored, err := repo.FindByWorkOrderID(t.Context(), order.ID())
	require.NoError(t, err)
	require.Equal(t, original.ID(), stored.ID())
	require.Equal(t, "private text", stored.Explanation())
	require.Equal(t, "upheld", stored.Status())
}
func TestReportReviewServiceRejectsHiddenFirstReceipt(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	order, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	_, err = fixture.database.Exec(`UPDATE work_order_reviews SET visible=false WHERE work_order_id=$1`, order.ID())
	require.NoError(t, err)
	repo := repositories.NewReviewReportRepository(fixture.database)
	service := workorder.NewReportReviewService(repositories.NewProviderActivityActorFinder(fixture.database), fixture.workOrderRepository, repositories.NewReviewUnitOfWork(fixture.database, repositories.NewReviewModerationRepository(), repo, repositories.NewAuditEventRepository(fixture.database)), clockadapter.NewSystemClock())
	_, err = service.Report(t.Context(), "auth0|review-repository-provider", order.ID(), "personal_data", "")
	require.ErrorIs(t, err, workorder.ErrReviewNotAvailable)
	_, err = repo.FindByWorkOrderID(t.Context(), order.ID())
	require.ErrorIs(t, err, workorder.ErrReviewReportNotFound)
}

func TestReviewReportRepositoryDoesNotMapOtherUniqueConstraint(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	first, _ := savePaidWorkOrderWithReview(t, fixture)
	_, err := fixture.workOrderRepository.Save(t.Context(), first)
	require.NoError(t, err)
	secondFixture := newProviderWorkOrderTestFixture(t, fixture, "report-other-constraint")
	second := savePaidWorkOrderWithReviewForFixture(t, fixture, secondFixture, time.Now().UTC().Add(48*time.Hour), "a0000000-0000-0000-0000-000000000001", 5, "Good work")
	repo := repositories.NewReviewReportRepository(fixture.database)
	original, err := workorder.NewReviewReport(first.ID(), first.ServiceProposal().ProviderID(), "personal_data", "", time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Save(t.Context(), original))
	// Force a real primary-key collision, not the lifetime uniqueness constraint.
	_, err = fixture.database.Exec(`SELECT setval(pg_get_serial_sequence('review_reports','id'),$1,false)`, original.ID())
	require.NoError(t, err)
	candidate, err := workorder.NewReviewReport(second.ID(), secondFixture.providerID, "personal_data", "", time.Now())
	require.NoError(t, err)
	err = repo.Save(t.Context(), candidate)
	require.Error(t, err)
	require.NotErrorIs(t, err, workorder.ErrReviewReportAlreadyExists)
	require.Zero(t, candidate.ID())
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
	require.Equal(t, "review_reports_pkey", pgErr.ConstraintName)
}
