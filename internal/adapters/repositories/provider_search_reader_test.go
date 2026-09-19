package repositories_test

import (
	"bytes"
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderSearchReaderHydratesRelationsWithoutMultiplyingRatings(t *testing.T) {
	fixture := newServiceProposalRepositoryTest(t)
	first := newProviderWorkOrderTestFixture(t, fixture, "search-first")
	second := newProviderWorkOrderTestFixture(t, fixture, "search-second")
	scheduled := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	for i, rating := range []int{5, 5, 4} {
		savePaidWorkOrderWithReviewForFixture(t, fixture, first, scheduled.Add(time.Duration(i)*24*time.Hour), uuid.NewString(), rating, "Good work")
	}
	found, err := fixture.userRepository.FindProviderByID(t.Context(), first.providerID)
	require.NoError(t, err)
	extra := savedCoverageZoneForProvider(t, fixture.database, "Extra zone")
	_, err = fixture.database.Exec(`UPDATE coverage_zones SET enabled = false, parent_zone_id = $1 WHERE id = $2`, found.CoverageZones[0].ID, extra.ID)
	require.NoError(t, err)
	_, err = fixture.database.Exec(`INSERT INTO provider_coverage_zones (provider_id, coverage_zone_id) VALUES ($1,$2)`, first.providerID, extra.ID)
	require.NoError(t, err)
	extra.Enabled = false
	extra.ParentZoneID = &found.CoverageZones[0].ID
	_, err = fixture.database.Exec(`UPDATE users SET name = 'Ana', surname = 'Z' WHERE id = $1`, first.providerID)
	require.NoError(t, err)
	_, err = fixture.database.Exec(`UPDATE users SET name = 'Ana', surname = 'A', profile_photo_file_id = NULL WHERE id = $1`, second.providerID)
	require.NoError(t, err)
	_, err = fixture.database.Exec(`DELETE FROM provider_coverage_zones WHERE provider_id = $1`, second.providerID)
	require.NoError(t, err)
	// Another category must not enter the search.
	saveIdentityProvider(t, fixture.database)
	identities := repositories.NewIdentityVerificationRepository(fixture.database)
	for i, status := range []identityverification.VerificationStatus{identityverification.StatusApproved, identityverification.StatusDeclined, identityverification.StatusApproved} {
		sessionID := uuid.MustParse([]string{"ffffffff-ffff-ffff-ffff-ffffffffffff", "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"}[i])
		created := scheduled
		if i == 0 {
			created = created.Add(-time.Hour)
		}
		session, err := identityverification.NewVerification(first.providerID, sessionID, uuid.New(), "didit", 1, created)
		require.NoError(t, err)
		session.Status = status
		require.NoError(t, identities.Save(t.Context(), session))
	}
	var queries bytes.Buffer
	ctx := observability.ContextWithLogger(t.Context(), slog.New(slog.NewTextHandler(&queries, &slog.HandlerOptions{Level: slog.LevelDebug})))
	results, err := repositories.NewProviderSearchReader(fixture.database).FindByCategoryID(ctx, found.Category.ID)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(queries.String(), "db.query.completed"))
	require.Len(t, results, 2)
	require.Equal(t, second.providerID, results[0].ID)
	require.NotNil(t, results[0].CoverageZones)
	require.Empty(t, results[0].CoverageZones)
	require.Zero(t, results[0].RatingAverage)
	require.Zero(t, results[0].RatingCount)
	require.False(t, results[0].IdentityVerified)
	require.Empty(t, results[0].ProfilePhoto.FileID)
	require.Equal(t, first.providerID, results[1].ID)
	require.Equal(t, append(found.CoverageZones, *extra), results[1].CoverageZones)
	require.Equal(t, 4.7, results[1].RatingAverage)
	require.Equal(t, 3, results[1].RatingCount)
	require.True(t, results[1].IdentityVerified)
	require.Equal(t, found.ProfilePhoto(), results[1].ProfilePhoto)

	_, err = fixture.database.Exec(`UPDATE identity_verification_sessions SET status = 'declined' WHERE external_session_id = '00000000-0000-0000-0000-000000000002'`)
	require.NoError(t, err)
	results, err = repositories.NewProviderSearchReader(fixture.database).FindByCategoryID(t.Context(), found.Category.ID)
	require.NoError(t, err)
	require.False(t, results[1].IdentityVerified)
}

func TestProviderSearchReaderEmptyAndCancelled(t *testing.T) {
	fixture := newProviderRepositoryTest(t)
	reader := repositories.NewProviderSearchReader(fixture.database)
	results, err := reader.FindByCategoryID(t.Context(), -1)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Empty(t, results)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	results, err = reader.FindByCategoryID(ctx, -1)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, results)
}

func TestProviderSearchQueryBudget(t *testing.T) {
	fixture := newProviderRepositoryTest(t)
	found := validProvider(t, fixture)
	_, err := fixture.providerRepository.Save(t.Context(), found)
	require.NoError(t, err)
	other := validProviderWithData(t, fixture.categoryRepository, fixture.database, "auth0|zoe", "zoe@example.com", "Zoe", "Perez", found.Category.Name)
	_, err = fixture.providerRepository.Save(t.Context(), other)
	require.NoError(t, err)
	var queries bytes.Buffer
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	config.Logger = slog.New(slog.NewTextHandler(&queries, &slog.HandlerOptions{Level: slog.LevelDebug}))
	database, err := db.ConnectPostgres(t.Context(), config)
	require.NoError(t, err)
	defer database.Close()
	files := filedomain.NewService(repositories.NewFileRepository(database), storage.NewMemoryStorage("https://cdn.example"), "public-bucket", "private-bucket", nil, nil)
	categories := repositories.NewCategoryRepository(database)
	service := provider.NewService(repositories.NewProviderSearchReader(database), nil, categories, files, nil, nil)

	for _, status := range []string{"confirmed", "pending"} {
		_, err = fixture.database.Exec(`UPDATE files SET status = $1 WHERE id = $2`, status, found.ProfilePhoto().FileID)
		require.NoError(t, err)
		queries.Reset()
		results, err := service.SearchProvidersByCategoryID(t.Context(), found.Category.ID)
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.Equal(t, 3, strings.Count(queries.String(), "db.query.completed"))
		if status == "confirmed" {
			require.NotEmpty(t, results[0].ProfilePhoto.URL)
		} else {
			require.Empty(t, results[0].ProfilePhoto.URL)
		}
	}
	_, err = fixture.database.Exec(`UPDATE files SET status = 'confirmed', visibility = 'private' WHERE id = $1`, found.ProfilePhoto().FileID)
	require.NoError(t, err)
	results, err := service.SearchProvidersByCategoryID(t.Context(), found.Category.ID)
	require.NoError(t, err)
	require.Empty(t, results[0].ProfilePhoto.URL)

	_, err = fixture.database.Exec(`UPDATE users SET profile_photo_file_id = NULL WHERE id IN ($1, $2)`, found.ID(), other.ID())
	require.NoError(t, err)
	queries.Reset()
	_, err = service.SearchProvidersByCategoryID(t.Context(), found.Category.ID)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(queries.String(), "db.query.completed"))

	_, err = fixture.database.Exec(`DELETE FROM providers WHERE category_id = $1`, found.Category.ID)
	require.NoError(t, err)
	queries.Reset()
	results, err = service.SearchProvidersByCategoryID(t.Context(), found.Category.ID)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Empty(t, results)
	require.Equal(t, 2, strings.Count(queries.String(), "db.query.completed"))

	queries.Reset()
	_, err = categories.ListAll()
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(queries.String(), "db.query.completed"))
}
