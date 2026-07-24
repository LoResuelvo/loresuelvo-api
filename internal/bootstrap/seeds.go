package bootstrap

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	"gopkg.in/yaml.v3"
)

const (
	seedRetryInterval = 2 * time.Second
	seedMaxAttempts   = 60
	defaultSeedsFile  = "seeds/default.yaml"

	seedProviderRole         = "provider"
	seedProfilePhotoPurpose  = "profile_photo"
	seedPublicFileStatus     = "confirmed"
	seedPublicFileVisibility = "public"
)

type seedData struct {
	Categories []categorySeed `yaml:"categories"`
	Providers  []providerSeed `yaml:"providers"`
}

type categorySeed struct {
	Name string `yaml:"name"`
}

type providerSeed struct {
	AuthID             string `yaml:"auth_id"`
	Email              string `yaml:"email"`
	Name               string `yaml:"name"`
	Surname            string `yaml:"surname"`
	CategoryName       string `yaml:"category"`
	ProfilePhotoFileID string `yaml:"profile_photo_file_id"`
	ProfilePhotoName   string `yaml:"profile_photo_name"`
	ProfilePhotoKey    string `yaml:"profile_photo_key"`
}

func StartDefaultDataSeederFromEnv(ctx context.Context, database *sql.DB) {
	if !seedsEnabledFromEnv() {
		return
	}

	data, found, err := loadSeedDataFromEnv()
	if err != nil {
		slog.Error("default seed data could not be loaded", "error", err)
		return
	}
	if !found {
		slog.Info("default seed data file was not found; skipping seeds", "file", seedsFileFromEnv())
		return
	}
	if data.IsEmpty() {
		slog.Info("default seed data is empty; skipping seeds", "file", seedsFileFromEnv())
		return
	}

	go func() {
		storageConfig := storage.NewConfigFromEnv()
		for attempt := 1; attempt <= seedMaxAttempts; attempt++ {
			if err := seedDefaultData(ctx, database, storageConfig.PublicBucket, data); err != nil {
				slog.Warn(
					"default seeds are not ready yet; will retry",
					"attempt",
					attempt,
					"max_attempts",
					seedMaxAttempts,
					"error",
					err,
				)

				select {
				case <-ctx.Done():
					return
				case <-time.After(seedRetryInterval):
					continue
				}
			}

			slog.Info("default seeds are ready", "categories", len(data.Categories), "providers", len(data.Providers))
			return
		}

		slog.Error("default seeds could not be applied after retries", "max_attempts", seedMaxAttempts)
	}()
}

func SeedDefaultDataFromEnv(ctx context.Context, database *sql.DB) error {
	if !seedsEnabledFromEnv() {
		return nil
	}

	data, found, err := loadSeedDataFromEnv()
	if err != nil {
		return err
	}
	if !found {
		slog.Info("default seed data file was not found; skipping seeds", "file", seedsFileFromEnv())
		return nil
	}
	if data.IsEmpty() {
		slog.Info("default seed data is empty; skipping seeds", "file", seedsFileFromEnv())
		return nil
	}

	storageConfig := storage.NewConfigFromEnv()
	if err := seedDefaultData(ctx, database, storageConfig.PublicBucket, data); err != nil {
		return err
	}

	slog.Info("default seeds are ready", "categories", len(data.Categories), "providers", len(data.Providers))
	return nil
}

func seedsEnabledFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("SEEDS_ENABLED")), "true")
}

func seedsFileFromEnv() string {
	path := strings.TrimSpace(os.Getenv("SEEDS_FILE"))
	if path == "" {
		return defaultSeedsFile
	}

	return path
}

func loadSeedDataFromEnv() (seedData, bool, error) {
	return loadSeedData(seedsFileFromEnv())
}

func loadSeedData(path string) (seedData, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return seedData{}, false, nil
	}
	if err != nil {
		return seedData{}, false, fmt.Errorf("reading seed data file %q: %w", path, err)
	}

	var data seedData
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&data); err != nil {
		return seedData{}, false, fmt.Errorf("parsing seed data file %q: %w", path, err)
	}

	return data, true, nil
}

func (data seedData) IsEmpty() bool {
	return len(data.Categories) == 0 && len(data.Providers) == 0
}

func seedDefaultData(ctx context.Context, database *sql.DB, publicBucket string, data seedData) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning seed transaction: %w", err)
	}

	for _, seed := range data.Categories {
		if _, err := upsertSeedCategory(ctx, tx, seed.Name); err != nil {
			return rollbackSeedTx(tx, err)
		}
	}

	for _, seed := range data.Providers {
		if err := seedDefaultProvider(ctx, tx, publicBucket, seed); err != nil {
			return rollbackSeedTx(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing seed transaction: %w", err)
	}

	return nil
}

func seedDefaultProvider(ctx context.Context, tx *sql.Tx, publicBucket string, seed providerSeed) error {
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

	if err := upsertSeedProvider(ctx, tx, userID, categoryID); err != nil {
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

func upsertSeedProviderProfilePhoto(ctx context.Context, tx *sql.Tx, publicBucket string, seed providerSeed) error {
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
		seedProfilePhotoPurpose,
		seed.AuthID,
	)
	if err != nil {
		return fmt.Errorf("seeding provider profile photo %q: %w", seed.ProfilePhotoFileID, err)
	}

	return nil
}

func upsertSeedProviderUser(ctx context.Context, tx *sql.Tx, seed providerSeed) (int, error) {
	var userID int
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO users (auth_id, email, name, surname, role, profile_photo_file_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (auth_id) DO UPDATE
		SET email = EXCLUDED.email,
			name = EXCLUDED.name,
			surname = EXCLUDED.surname,
			role = EXCLUDED.role,
			profile_photo_file_id = EXCLUDED.profile_photo_file_id,
			updated_on = NOW()
		RETURNING id`,
		seed.AuthID,
		seed.Email,
		seed.Name,
		seed.Surname,
		seedProviderRole,
		seed.ProfilePhotoFileID,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("seeding provider user %q: %w", seed.AuthID, err)
	}

	return userID, nil
}

func upsertSeedProvider(ctx context.Context, tx *sql.Tx, userID, categoryID int) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO providers (user_id, category_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET category_id = EXCLUDED.category_id`,
		userID,
		categoryID,
	)
	if err != nil {
		return fmt.Errorf("seeding provider for user %d: %w", userID, err)
	}

	return nil
}

func rollbackSeedTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback seed transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
