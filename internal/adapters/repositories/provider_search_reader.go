package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ProviderSearchReader struct{ db *sql.DB }

func NewProviderSearchReader(database *sql.DB) *ProviderSearchReader {
	return &ProviderSearchReader{db: database}
}

const providerSearchSQL = `WITH selected_providers AS (
 SELECT user_id FROM providers WHERE category_id = $1
), zones AS (
 SELECT pc.provider_id, json_agg(z ORDER BY z."ID") AS coverage_zones
 FROM selected_providers p
 JOIN provider_coverage_zones pc ON pc.provider_id = p.user_id
 JOIN (
  SELECT id AS "ID", market_id AS "MarketID", code AS "Code", name AS "Name",
   normalized_name AS "NormalizedName", kind AS "Kind",
   parent_zone_id AS "ParentZoneID", enabled AS "Enabled"
  FROM coverage_zones
 ) z ON z."ID" = pc.coverage_zone_id
 GROUP BY pc.provider_id
), ratings AS (
 SELECT sp.provider_id, SUM(review.rating) AS total, COUNT(review.work_order_id) AS count
 FROM selected_providers p
 JOIN service_proposals sp ON sp.provider_id = p.user_id
 JOIN work_orders wo ON wo.service_proposal_id = sp.id
 JOIN work_order_reviews review ON review.work_order_id = wo.id
 GROUP BY sp.provider_id
), identities AS (
 SELECT DISTINCT ON (s.provider_id) s.provider_id, s.status
 FROM selected_providers p
 JOIN identity_verification_sessions s ON s.provider_id = p.user_id
 ORDER BY s.provider_id, s.created_on DESC, s.external_session_id DESC
)
SELECT p.user_id, u.name, u.surname, c.name,
 COALESCE(u.profile_photo_file_id::text, ''), COALESCE(photo.original_name, ''),
 COALESCE(z.coverage_zones, '[]'::json), COALESCE(r.total, 0), COALESCE(r.count, 0),
 COALESCE(i.status, '')
FROM selected_providers p
JOIN users u ON u.id = p.user_id
JOIN categories c ON c.id = $1
LEFT JOIN files photo ON photo.id = u.profile_photo_file_id
LEFT JOIN zones z ON z.provider_id = p.user_id
LEFT JOIN ratings r ON r.provider_id = p.user_id
LEFT JOIN identities i ON i.provider_id = p.user_id
ORDER BY u.name ASC, u.surname ASC`

func (reader *ProviderSearchReader) FindByCategoryID(ctx context.Context, categoryID int) ([]readmodel.ProviderSearchResult, error) {
	rows, err := reader.db.QueryContext(ctx, providerSearchSQL, categoryID)
	if err != nil {
		return nil, fmt.Errorf("finding providers for search: %w", err)
	}
	defer rows.Close()
	results := make([]readmodel.ProviderSearchResult, 0)
	for rows.Next() {
		result := readmodel.ProviderSearchResult{ProfilePhoto: &filedomain.Image{}}
		var zones []byte
		var stats provider.RatingStats
		var status identityverification.VerificationStatus
		if err := rows.Scan(&result.ID, &result.Name, &result.Surname, &result.CategoryName,
			&result.ProfilePhoto.FileID, &result.ProfilePhoto.OriginalName, &zones, &stats.Total, &stats.Count, &status); err != nil {
			return nil, fmt.Errorf("scanning provider search result: %w", err)
		}
		if err := json.Unmarshal(zones, &result.CoverageZones); err != nil {
			return nil, fmt.Errorf("decoding provider coverage zones: %w", err)
		}
		summary := stats.Summary()
		result.RatingAverage, result.RatingCount = summary.Average, summary.Count
		result.IdentityVerified = status == identityverification.StatusApproved
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating provider search results: %w", err)
	}
	return results, nil
}
