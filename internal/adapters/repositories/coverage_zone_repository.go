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

func (repository *CoverageZoneRepository) SaveMarket(ctx context.Context, market coveragezone.Market) (*coveragezone.Market, error) {
	var savedMarket coveragezone.Market
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO coverage_markets (code, name, enabled, created_on, updated_on)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, code, name, enabled`,
		market.Code,
		market.Name,
		market.Enabled,
	).Scan(&savedMarket.ID, &savedMarket.Code, &savedMarket.Name, &savedMarket.Enabled)
	if err != nil {
		return nil, fmt.Errorf("saving coverage market: %w", err)
	}

	return &savedMarket, nil
}

func (repository *CoverageZoneRepository) FindMarketByCode(ctx context.Context, code string) (*coveragezone.Market, error) {
	var market coveragezone.Market
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, code, name, enabled FROM coverage_markets WHERE code = $1`,
		strings.ToUpper(strings.TrimSpace(code)),
	).Scan(&market.ID, &market.Code, &market.Name, &market.Enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage market by code: %w", err)
	}

	return &market, nil
}

func (repository *CoverageZoneRepository) Save(ctx context.Context, zone coveragezone.CoverageZone) (*coveragezone.CoverageZone, error) {
	var savedZone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO coverage_zones (market_id, code, name, normalized_name, kind, parent_zone_id, enabled, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, market_id, code, name, normalized_name, kind, parent_zone_id, enabled`,
		zone.MarketID,
		zone.Code,
		zone.Name,
		zone.NormalizedName,
		zone.Kind,
		zone.ParentZoneID,
		zone.Enabled,
	).Scan(&savedZone.ID, &savedZone.MarketID, &savedZone.Code, &savedZone.Name, &savedZone.NormalizedName, &savedZone.Kind, &savedZone.ParentZoneID, &savedZone.Enabled)
	if err != nil {
		return nil, fmt.Errorf("saving coverage zone: %w", err)
	}

	return &savedZone, nil
}

func (repository *CoverageZoneRepository) FindByMarketCodeAndName(ctx context.Context, marketCode, name string) (*coveragezone.CoverageZone, error) {
	var zone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT coverage_zones.id, coverage_zones.market_id, coverage_zones.code, coverage_zones.name,
			coverage_zones.normalized_name, coverage_zones.kind, coverage_zones.parent_zone_id, coverage_zones.enabled
		FROM coverage_zones
		INNER JOIN coverage_markets ON coverage_markets.id = coverage_zones.market_id
		WHERE coverage_markets.code = $1 AND coverage_zones.normalized_name = $2`,
		strings.ToUpper(strings.TrimSpace(marketCode)),
		strings.ToLower(strings.TrimSpace(name)),
	).Scan(&zone.ID, &zone.MarketID, &zone.Code, &zone.Name, &zone.NormalizedName, &zone.Kind, &zone.ParentZoneID, &zone.Enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage zone by name: %w", err)
	}

	return &zone, nil
}

func (repository *CoverageZoneRepository) FindByID(ctx context.Context, id int) (*coveragezone.CoverageZone, error) {
	var zone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, market_id, code, name, normalized_name, kind, parent_zone_id, enabled
		FROM coverage_zones
		WHERE id = $1`,
		id,
	).Scan(&zone.ID, &zone.MarketID, &zone.Code, &zone.Name, &zone.NormalizedName, &zone.Kind, &zone.ParentZoneID, &zone.Enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage zone by id: %w", err)
	}

	return &zone, nil
}

func (repository *CoverageZoneRepository) SaveExternalReference(ctx context.Context, reference coveragezone.ExternalReference) error {
	_, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO coverage_zone_external_references (
			coverage_zone_id, provider, external_id, source_version, created_on, updated_on
		) VALUES ($1, $2, $3, NULLIF($4, ''), NOW(), NOW())
		ON CONFLICT (coverage_zone_id, provider) DO UPDATE
		SET external_id = EXCLUDED.external_id,
			source_version = EXCLUDED.source_version,
			updated_on = NOW()`,
		reference.CoverageZoneID,
		reference.Provider,
		reference.ExternalID,
		reference.SourceVersion,
	)
	if err != nil {
		return fmt.Errorf("saving coverage zone external reference: %w", err)
	}

	return nil
}

func (repository *CoverageZoneRepository) FindByExternalReference(ctx context.Context, provider, externalID string) (*coveragezone.CoverageZone, error) {
	var zone coveragezone.CoverageZone
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT coverage_zones.id, coverage_zones.market_id, coverage_zones.code, coverage_zones.name,
			coverage_zones.normalized_name, coverage_zones.kind, coverage_zones.parent_zone_id, coverage_zones.enabled
		FROM coverage_zone_external_references
		INNER JOIN coverage_zones ON coverage_zones.id = coverage_zone_external_references.coverage_zone_id
		WHERE coverage_zone_external_references.provider = $1
			AND coverage_zone_external_references.external_id = $2`,
		strings.ToUpper(strings.TrimSpace(provider)),
		strings.TrimSpace(externalID),
	).Scan(&zone.ID, &zone.MarketID, &zone.Code, &zone.Name, &zone.NormalizedName, &zone.Kind, &zone.ParentZoneID, &zone.Enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, coveragezone.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding coverage zone by external reference: %w", err)
	}

	return &zone, nil
}

func (repository *CoverageZoneRepository) Update(ctx context.Context, zone coveragezone.CoverageZone) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE coverage_zones
		SET market_id = $1, code = $2, name = $3, normalized_name = $4, kind = $5,
			parent_zone_id = $6, enabled = $7, updated_on = NOW()
		WHERE id = $8`,
		zone.MarketID,
		zone.Code,
		zone.Name,
		zone.NormalizedName,
		zone.Kind,
		zone.ParentZoneID,
		zone.Enabled,
		zone.ID,
	)
	if err != nil {
		return fmt.Errorf("updating coverage zone: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated coverage zone: %w", err)
	}
	if rowsAffected == 0 {
		return coveragezone.ErrDoesNotExist
	}

	return nil
}

func (repository *CoverageZoneRepository) FindByProviderID(ctx context.Context, providerID int) ([]coveragezone.CoverageZone, error) {
	zonesByProviderID, err := repository.FindByProviderIDs(ctx, []int{providerID})
	if err != nil {
		return nil, err
	}

	return zonesByProviderID[providerID], nil
}

func (repository *CoverageZoneRepository) FindByProviderIDs(ctx context.Context, providerIDs []int) (map[int][]coveragezone.CoverageZone, error) {
	zonesByProviderID := make(map[int][]coveragezone.CoverageZone, len(providerIDs))
	if len(providerIDs) == 0 {
		return zonesByProviderID, nil
	}

	placeholders := make([]string, 0, len(providerIDs))
	args := make([]any, 0, len(providerIDs))
	for index, providerID := range providerIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		args = append(args, providerID)
		zonesByProviderID[providerID] = []coveragezone.CoverageZone{}
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT provider_coverage_zones.provider_id,
			coverage_zones.id,
			coverage_zones.market_id,
			coverage_zones.code,
			coverage_zones.name,
			coverage_zones.normalized_name,
			coverage_zones.kind,
			coverage_zones.parent_zone_id,
			coverage_zones.enabled
		FROM provider_coverage_zones
		INNER JOIN coverage_zones ON coverage_zones.id = provider_coverage_zones.coverage_zone_id
		WHERE provider_coverage_zones.provider_id IN (`+strings.Join(placeholders, ", ")+`)
		ORDER BY provider_coverage_zones.provider_id ASC, provider_coverage_zones.coverage_zone_id ASC`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("finding coverage zones by provider ids: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var providerID int
		var zone coveragezone.CoverageZone
		if err := rows.Scan(&providerID, &zone.ID, &zone.MarketID, &zone.Code, &zone.Name, &zone.NormalizedName, &zone.Kind, &zone.ParentZoneID, &zone.Enabled); err != nil {
			return nil, fmt.Errorf("scanning provider coverage zone: %w", err)
		}
		zonesByProviderID[providerID] = append(zonesByProviderID[providerID], zone)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating provider coverage zones: %w", err)
	}

	return zonesByProviderID, nil
}

func (repository *CoverageZoneRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM coverage_zones`)
	return err
}
