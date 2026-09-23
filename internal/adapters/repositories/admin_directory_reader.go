package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type AdminDirectoryReader struct {
	db *sql.DB
}

func NewAdminDirectoryReader(database *sql.DB) *AdminDirectoryReader {
	return &AdminDirectoryReader{db: database}
}

func (reader *AdminDirectoryReader) FindConsumers(ctx context.Context, query string) ([]readmodel.Consumer, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT users.id, users.name, users.surname, users.email,
			COALESCE(users.profile_photo_file_id::text, ''), users.created_on
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE users.role = $1
			AND (STRPOS(LOWER(users.name), LOWER($2)) > 0
				OR STRPOS(LOWER(users.surname), LOWER($2)) > 0
				OR STRPOS(LOWER(users.email), LOWER($2)) > 0)
		ORDER BY users.created_on ASC, users.id ASC`,
		consumer.Role,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("querying administrative consumer directory: %w", err)
	}
	defer rows.Close()

	consumers := make([]readmodel.Consumer, 0)
	for rows.Next() {
		var found readmodel.Consumer
		if err := rows.Scan(
			&found.ID,
			&found.Name,
			&found.Surname,
			&found.Email,
			&found.ProfilePhotoFileID,
			&found.CreatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning administrative consumer directory: %w", err)
		}
		consumers = append(consumers, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating administrative consumer directory: %w", err)
	}

	return consumers, nil
}

const adminProviderDirectorySQL = `WITH latest_verifications AS (
	SELECT DISTINCT ON (session.provider_id)
		session.provider_id, session.status, session.verified_on
	FROM identity_verification_sessions session
	ORDER BY session.provider_id, session.created_on DESC, session.external_session_id DESC
)
SELECT users.id, users.name, users.surname, users.email,
	COALESCE(users.profile_photo_file_id::text, ''), users.created_on,
	categories.id, categories.name,
	zones.id, zones.market_id, zones.code, zones.name, zones.kind,
	zones.parent_zone_id, zones.enabled,
	COALESCE(latest_verifications.status, 'unverified'),
	CASE WHEN latest_verifications.status = 'approved' THEN latest_verifications.verified_on END
FROM providers
INNER JOIN users ON users.id = providers.user_id
INNER JOIN categories ON categories.id = providers.category_id
LEFT JOIN provider_coverage_zones ON provider_coverage_zones.provider_id = providers.user_id
LEFT JOIN coverage_zones zones ON zones.id = provider_coverage_zones.coverage_zone_id
LEFT JOIN latest_verifications ON latest_verifications.provider_id = providers.user_id
WHERE users.role = $1
	AND (STRPOS(LOWER(users.name), LOWER($2)) > 0
		OR STRPOS(LOWER(users.surname), LOWER($2)) > 0
		OR STRPOS(LOWER(users.email), LOWER($2)) > 0)
ORDER BY users.created_on ASC, users.id ASC, zones.id ASC`

func (reader *AdminDirectoryReader) FindProviders(ctx context.Context, query string) ([]readmodel.Provider, error) {
	rows, err := reader.db.QueryContext(ctx, adminProviderDirectorySQL, provider.Role, query)
	if err != nil {
		return nil, fmt.Errorf("querying administrative provider directory: %w", err)
	}
	defer rows.Close()

	providers := make([]readmodel.Provider, 0)
	providerIndexByID := make(map[int]int)
	for rows.Next() {
		var found readmodel.Provider
		var zoneID, marketID, parentZoneID sql.NullInt64
		var zoneCode, zoneName, zoneKind sql.NullString
		var zoneEnabled sql.NullBool
		var verificationStatus string
		var verifiedOn sql.NullTime
		if err := rows.Scan(
			&found.ID,
			&found.Name,
			&found.Surname,
			&found.Email,
			&found.ProfilePhotoFileID,
			&found.CreatedOn,
			&found.Category.ID,
			&found.Category.Name,
			&zoneID,
			&marketID,
			&zoneCode,
			&zoneName,
			&zoneKind,
			&parentZoneID,
			&zoneEnabled,
			&verificationStatus,
			&verifiedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning administrative provider directory: %w", err)
		}

		index, exists := providerIndexByID[found.ID]
		if !exists {
			found.CoverageZones = make([]readmodel.ProviderCoverageZone, 0)
			found.IdentityVerificationStatus = identityverification.VerificationStatus(verificationStatus)
			if verifiedOn.Valid {
				timestamp := verifiedOn.Time.UTC()
				found.IdentityVerifiedOn = &timestamp
			}
			providers = append(providers, found)
			index = len(providers) - 1
			providerIndexByID[found.ID] = index
		}

		if zoneID.Valid {
			zone := readmodel.ProviderCoverageZone{
				ID:       int(zoneID.Int64),
				MarketID: int(marketID.Int64),
				Code:     zoneCode.String,
				Name:     zoneName.String,
				Kind:     coveragezone.Kind(zoneKind.String),
				Enabled:  zoneEnabled.Bool,
			}
			if parentZoneID.Valid {
				parentID := int(parentZoneID.Int64)
				zone.ParentZoneID = &parentID
			}
			providers[index].CoverageZones = append(providers[index].CoverageZones, zone)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating administrative provider directory: %w", err)
	}

	return providers, nil
}
