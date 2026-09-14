package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
)

const (
	adminSeedAuthIDEnv  = "ADMIN_SEED_AUTH_ID"
	adminSeedEmailEnv   = "ADMIN_SEED_EMAIL"
	adminSeedNameEnv    = "ADMIN_SEED_NAME"
	adminSeedSurnameEnv = "ADMIN_SEED_SURNAME"

	adminSeedIdentityMaxLength = 255
	adminSeedNameMaxLength     = 100
)

var ErrInvalidAdminSeedConfiguration = errors.New("admin seed configuration is invalid")

type adminSeedConfig struct {
	authID  string
	email   string
	name    string
	surname string
}

func SeedAdminFromEnv(ctx context.Context, database *sql.DB) error {
	config, err := adminSeedConfigFromEnv()
	if err != nil {
		return err
	}

	configuredAdmin, err := admin.NewAdmin(config.authID, config.email, config.name, config.surname, nil)
	if err != nil {
		return fmt.Errorf("%w: %s is invalid", ErrInvalidAdminSeedConfiguration, adminSeedEmailEnv)
	}

	repository := repositories.NewUserRepository(database)
	if err := repository.EnsureAdmin(ctx, configuredAdmin); err != nil {
		return fmt.Errorf("ensuring mandatory administrator: %w", err)
	}
	return nil
}

func adminSeedConfigFromEnv() (adminSeedConfig, error) {
	config := adminSeedConfig{
		authID:  strings.TrimSpace(os.Getenv(adminSeedAuthIDEnv)),
		email:   strings.TrimSpace(os.Getenv(adminSeedEmailEnv)),
		name:    strings.TrimSpace(os.Getenv(adminSeedNameEnv)),
		surname: strings.TrimSpace(os.Getenv(adminSeedSurnameEnv)),
	}

	requiredValues := []struct {
		name      string
		value     string
		maxLength int
	}{
		{name: adminSeedAuthIDEnv, value: config.authID, maxLength: adminSeedIdentityMaxLength},
		{name: adminSeedEmailEnv, value: config.email, maxLength: adminSeedIdentityMaxLength},
		{name: adminSeedNameEnv, value: config.name, maxLength: adminSeedNameMaxLength},
		{name: adminSeedSurnameEnv, value: config.surname, maxLength: adminSeedNameMaxLength},
	}
	for _, required := range requiredValues {
		if required.value == "" {
			return adminSeedConfig{}, fmt.Errorf("%w: %s is required", ErrInvalidAdminSeedConfiguration, required.name)
		}
		if utf8.RuneCountInString(required.value) > required.maxLength {
			return adminSeedConfig{}, fmt.Errorf("%w: %s exceeds its maximum length", ErrInvalidAdminSeedConfiguration, required.name)
		}
	}

	return config, nil
}
