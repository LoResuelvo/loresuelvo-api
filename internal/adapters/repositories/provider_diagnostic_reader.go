package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"time"
)

type ProviderDiagnosticReader struct{ db *sql.DB }

func NewProviderDiagnosticReader(db *sql.DB) *ProviderDiagnosticReader {
	return &ProviderDiagnosticReader{db}
}

const diagnosticProfileSQL = `SELECT u.id,u.name,u.surname,u.email,COALESCE(u.profile_photo_file_id::text,''),u.created_on,c.id,c.name,
 v.status,v.verified_on,v.last_result_on
 FROM providers p JOIN users u ON u.id=p.user_id JOIN categories c ON c.id=p.category_id
 LEFT JOIN LATERAL (SELECT status,verified_on,last_result_on FROM identity_verification_sessions WHERE provider_id=p.user_id ORDER BY created_on DESC,external_session_id DESC LIMIT 1) v ON true
 WHERE p.user_id=$1 AND u.role='provider'`
const diagnosticZonesSQL = `SELECT z.id,z.market_id,z.code,z.name,z.kind,z.parent_zone_id,z.enabled FROM provider_coverage_zones p JOIN coverage_zones z ON z.id=p.coverage_zone_id WHERE p.provider_id=$1 ORDER BY z.id`
const diagnosticPaymentSQL = `SELECT connected_on,token_expires_on FROM provider_payment_accounts WHERE provider_id=$1 AND payment_provider='mercado_pago'`
const diagnosticCalendarSQL = `SELECT status,connected_on,updated_on FROM google_calendar_connections WHERE user_id=$1`

func (r *ProviderDiagnosticReader) FindByProviderID(ctx context.Context, id int) (diagnostic *readmodel.ProviderDiagnostic, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning provider diagnostic read: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rolling back provider diagnostic read: %w", e))
			}
		}
	}()
	d := &readmodel.ProviderDiagnostic{}
	var status sql.NullString
	var verified, result sql.NullTime
	err = tx.QueryRowContext(ctx, diagnosticProfileSQL, id).Scan(&d.Provider.ID, &d.Provider.Name, &d.Provider.Surname, &d.Provider.Email, &d.Provider.ProfilePhotoFileID, &d.Provider.CreatedOn, &d.Provider.Category.ID, &d.Provider.Category.Name, &status, &verified, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading diagnostic profile: %w", err)
	}
	d.Provider.CreatedOn = d.Provider.CreatedOn.UTC()
	verificationStatus := identityverification.StatusUnverified
	if status.Valid {
		verificationStatus = identityverification.VerificationStatus(status.String)
	}
	d.RecordIdentityEvidence(verificationStatus, diagnosticTime(verified), diagnosticTime(result))
	d.Provider.CoverageZones, err = readDiagnosticZones(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	var connected, expires sql.NullTime
	err = tx.QueryRowContext(ctx, diagnosticPaymentSQL, id).Scan(&connected, &expires)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reading diagnostic payment evidence: %w", err)
	}
	d.Payment = readmodel.PaymentEvidence{ConnectedOn: diagnosticTime(connected), TokenExpiresOn: diagnosticTime(expires)}
	var calendarStatus string
	var calendarConnected, updated sql.NullTime
	err = tx.QueryRowContext(ctx, diagnosticCalendarSQL, id).Scan(&calendarStatus, &calendarConnected, &updated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reading diagnostic Calendar evidence: %w", err)
	}
	d.Calendar = readmodel.CalendarEvidence{Status: calendarStatus, ConnectedOn: diagnosticTime(calendarConnected), UpdatedOn: diagnosticTime(updated)}
	if err = readDiagnosticActivity(ctx, tx, id, d); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider diagnostic read: %w", err)
	}
	committed = true
	return d, nil
}
func diagnosticTime(t sql.NullTime) *time.Time {
	if !t.Valid || t.Time.IsZero() {
		return nil
	}
	utc := t.Time.UTC()
	return &utc
}
func readDiagnosticZones(ctx context.Context, tx *sql.Tx, id int) ([]readmodel.ProviderCoverageZone, error) {
	rows, err := tx.QueryContext(ctx, diagnosticZonesSQL, id)
	if err != nil {
		return nil, fmt.Errorf("reading diagnostic coverage: %w", err)
	}
	defer rows.Close()
	zones := make([]readmodel.ProviderCoverageZone, 0)
	for rows.Next() {
		var z readmodel.ProviderCoverageZone
		var parent sql.NullInt64
		var kind string
		if err := rows.Scan(&z.ID, &z.MarketID, &z.Code, &z.Name, &kind, &parent, &z.Enabled); err != nil {
			return nil, fmt.Errorf("scanning diagnostic coverage: %w", err)
		}
		z.Kind = coveragezone.Kind(kind)
		if parent.Valid {
			value := int(parent.Int64)
			z.ParentZoneID = &value
		}
		zones = append(zones, z)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating diagnostic coverage: %w", err)
	}
	return zones, nil
}
