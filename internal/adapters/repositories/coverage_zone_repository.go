package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
)

type CoverageZoneRepository struct {
	db *sql.DB
}

func NewCoverageZoneRepository(db *sql.DB) *CoverageZoneRepository {
	return &CoverageZoneRepository{db: db}
}

func (repository *CoverageZoneRepository) Save(ctx context.Context, zone coveragezone.CoverageZone) (*coveragezone.CoverageZone, error) {
	var savedZone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO coverage_zones (name, normalized_name, enabled, created_on, updated_on)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, name, normalized_name, enabled`,
		zone.Name,
		zone.NormalizedName,
		zone.Enabled,
	).Scan(&savedZone.ID, &savedZone.Name, &savedZone.NormalizedName, &savedZone.Enabled)
	if err != nil {
		return nil, fmt.Errorf("saving coverage zone: %w", err)
	}

	return &savedZone, nil
}

func (repository *CoverageZoneRepository) FindByName(ctx context.Context, name string) (*coveragezone.CoverageZone, error) {
	var zone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, name, normalized_name, enabled
		FROM coverage_zones
		WHERE normalized_name = $1`,
		strings.ToLower(strings.TrimSpace(name)),
	).Scan(&zone.ID, &zone.Name, &zone.NormalizedName, &zone.Enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage zone by name: %w", err)
	}

	return &zone, nil
}

func (repository *CoverageZoneRepository) FindByProviderID(ctx context.Context, providerID int) ([]coveragezone.CoverageZone, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT coverage_zones.id, coverage_zones.name, coverage_zones.normalized_name, coverage_zones.enabled
		FROM provider_coverage_zones
		INNER JOIN coverage_zones ON coverage_zones.id = provider_coverage_zones.coverage_zone_id
		WHERE provider_coverage_zones.provider_id = $1
		ORDER BY provider_coverage_zones.coverage_zone_id ASC`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider coverage zones: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	zones := make([]coveragezone.CoverageZone, 0)
	for rows.Next() {
		var zone coveragezone.CoverageZone
		if err := rows.Scan(&zone.ID, &zone.Name, &zone.NormalizedName, &zone.Enabled); err != nil {
			return nil, fmt.Errorf("scanning provider coverage zone: %w", err)
		}
		zones = append(zones, zone)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating provider coverage zones: %w", err)
	}

	return zones, nil
}

func (repository *CoverageZoneRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM coverage_zones`)
	return err
}
