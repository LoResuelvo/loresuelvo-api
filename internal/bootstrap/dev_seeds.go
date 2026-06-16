package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
)

const (
	developmentSeedRetryInterval = 2 * time.Second
	developmentSeedMaxAttempts   = 60

	seedProviderRole                = "provider"
	seedProviderProfilePhotoPurpose = "provider_profile_photo"
	seedPublicFileStatus            = "confirmed"
	seedPublicFileVisibility        = "public"
)

type developmentProviderSeed struct {
	AuthID             string
	Email              string
	Name               string
	Surname            string
	CategoryName       string
	ProfilePhotoFileID string
	ProfilePhotoName   string
	ProfilePhotoKey    string
}

var developmentProviderSeeds = []developmentProviderSeed{
	{
		AuthID:             "auth0|seed-provider-plumbing",
		Email:              "juan.plomero.seed@example.com",
		Name:               "Juan",
		Surname:            "Gómez",
		CategoryName:       "Plomería",
		ProfilePhotoFileID: "11111111-1111-4111-8111-111111111111",
		ProfilePhotoName:   "juan-plomero-seed.webp",
		ProfilePhotoKey:    "seed/provider_profile_photo/juan-plomero-seed.webp",
	},
	{
		AuthID:             "auth0|seed-provider-electricity",
		Email:              "laura.electricista.seed@example.com",
		Name:               "Laura",
		Surname:            "Suárez",
		CategoryName:       "Electricidad",
		ProfilePhotoFileID: "22222222-2222-4222-8222-222222222222",
		ProfilePhotoName:   "laura-electricista-seed.webp",
		ProfilePhotoKey:    "seed/provider_profile_photo/laura-electricista-seed.webp",
	},
}

func StartDevelopmentDataSeederFromEnv(ctx context.Context, database *sql.DB) {
	if !developmentSeedsEnabledFromEnv() {
		return
	}

	go func() {
		storageConfig := storage.NewConfigFromEnv()
		for attempt := 1; attempt <= developmentSeedMaxAttempts; attempt++ {
			if err := seedDevelopmentProviders(ctx, database, storageConfig.PublicBucket); err != nil {
				slog.Warn(
					"development provider seeds are not ready yet; will retry",
					"attempt",
					attempt,
					"max_attempts",
					developmentSeedMaxAttempts,
					"error",
					err,
				)

				select {
				case <-ctx.Done():
					return
				case <-time.After(developmentSeedRetryInterval):
					continue
				}
			}

			slog.Info("development provider seeds are ready", "providers", len(developmentProviderSeeds))
			return
		}

		slog.Error("development provider seeds could not be applied after retries", "max_attempts", developmentSeedMaxAttempts)
	}()
}

func SeedDevelopmentDataFromEnv(ctx context.Context, database *sql.DB) error {
	if !developmentSeedsEnabledFromEnv() {
		return nil
	}

	storageConfig := storage.NewConfigFromEnv()
	if err := seedDevelopmentProviders(ctx, database, storageConfig.PublicBucket); err != nil {
		return err
	}

	slog.Info("development provider seeds are ready", "providers", len(developmentProviderSeeds))
	return nil
}

func developmentSeedsEnabledFromEnv() bool {
	if strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) != "dev" {
		return false
	}

	return !strings.EqualFold(strings.TrimSpace(os.Getenv("DEV_SEEDS_ENABLED")), "false")
}

func seedDevelopmentProviders(ctx context.Context, database *sql.DB, publicBucket string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning development seed transaction: %w", err)
	}

	for _, seed := range developmentProviderSeeds {
		if err := seedDevelopmentProvider(ctx, tx, publicBucket, seed); err != nil {
			return rollbackDevelopmentSeedTx(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing development seed transaction: %w", err)
	}

	return nil
}

func seedDevelopmentProvider(ctx context.Context, tx *sql.Tx, publicBucket string, seed developmentProviderSeed) error {
	categoryID, err := upsertSeedCategory(ctx, tx, seed.CategoryName)
	if err != nil {
		return err
	}

	if err := upsertSeedProviderProfilePhoto(ctx, tx, publicBucket, seed); err != nil {
		return err
	}

	userID, err := upsertSeedProviderUser(ctx, tx, seed)
	if err != nil {
		return err
	}

	if err := upsertSeedProvider(ctx, tx, userID, categoryID, seed.ProfilePhotoFileID); err != nil {
		return err
	}

	return nil
}

func upsertSeedCategory(ctx context.Context, tx *sql.Tx, categoryName string) (int, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(categoryName))
	var categoryID int
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO categories (name, normalized_name, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (normalized_name) DO UPDATE
		SET name = EXCLUDED.name,
			updated_on = NOW()
		RETURNING id`,
		strings.TrimSpace(categoryName),
		normalizedName,
	).Scan(&categoryID)
	if err != nil {
		return 0, fmt.Errorf("seeding category %q: %w", categoryName, err)
	}

	return categoryID, nil
}

func upsertSeedProviderProfilePhoto(ctx context.Context, tx *sql.Tx, publicBucket string, seed developmentProviderSeed) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, 'image/webp', 1024, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET key = EXCLUDED.key,
			bucket = EXCLUDED.bucket,
			original_name = EXCLUDED.original_name,
			mime_type = EXCLUDED.mime_type,
			size_bytes = EXCLUDED.size_bytes,
			status = EXCLUDED.status,
			visibility = EXCLUDED.visibility,
			purpose = EXCLUDED.purpose,
			uploaded_by_auth_id = EXCLUDED.uploaded_by_auth_id,
			updated_on = NOW()`,
		seed.ProfilePhotoFileID,
		seed.ProfilePhotoKey,
		publicBucket,
		seed.ProfilePhotoName,
		seedPublicFileStatus,
		seedPublicFileVisibility,
		seedProviderProfilePhotoPurpose,
		seed.AuthID,
	)
	if err != nil {
		return fmt.Errorf("seeding provider profile photo %q: %w", seed.ProfilePhotoFileID, err)
	}

	return nil
}

func upsertSeedProviderUser(ctx context.Context, tx *sql.Tx, seed developmentProviderSeed) (int, error) {
	var userID int
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (auth_id) DO UPDATE
		SET email = EXCLUDED.email,
			name = EXCLUDED.name,
			surname = EXCLUDED.surname,
			role = EXCLUDED.role,
			updated_on = NOW()
		RETURNING id`,
		seed.AuthID,
		seed.Email,
		seed.Name,
		seed.Surname,
		seedProviderRole,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("seeding provider user %q: %w", seed.AuthID, err)
	}

	return userID, nil
}

func upsertSeedProvider(ctx context.Context, tx *sql.Tx, userID, categoryID int, profilePhotoFileID string) error {
	var providerID int
	err := tx.QueryRowContext(
		ctx,
		`SELECT id FROM providers WHERE user_id = $1 ORDER BY id ASC LIMIT 1`,
		userID,
	).Scan(&providerID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO providers (user_id, category_id, profile_photo_file_id)
			VALUES ($1, $2, $3)`,
			userID,
			categoryID,
			profilePhotoFileID,
		)
		if err != nil {
			return fmt.Errorf("inserting provider seed for user %d: %w", userID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("finding provider seed for user %d: %w", userID, err)
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE providers
		SET category_id = $2,
			profile_photo_file_id = $3
		WHERE id = $1`,
		providerID,
		categoryID,
		profilePhotoFileID,
	)
	if err != nil {
		return fmt.Errorf("updating provider seed %d: %w", providerID, err)
	}

	return nil
}

func rollbackDevelopmentSeedTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback development seed transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
