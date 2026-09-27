package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// operationDetailSQL resolves only stable inbox operation identities. The first
// proposal remains in its job request operation; later proposals are independent.
const operationDetailSQL = `WITH selected AS (
 SELECT 'jr'::text AS kind, jr.id AS resource_id, jr.created_on AS started_on,
   jr.id AS job_request_id,
   (SELECT sp.id FROM service_proposals sp WHERE sp.conversation_id = jr.conversation_id ORDER BY sp.id LIMIT 1) AS proposal_id,
   jr.consumer_id, jr.provider_id
 FROM job_requests jr WHERE $1::text = 'jr' AND jr.id = $2
 UNION ALL
 SELECT 'sp'::text, sp.id, sp.created_on, jr.id, sp.id, sp.consumer_id, sp.provider_id
 FROM service_proposals sp
 LEFT JOIN job_requests jr ON jr.conversation_id = sp.conversation_id
 WHERE $1::text = 'sp' AND sp.id = $2
   AND (jr.id IS NULL OR sp.id <> (SELECT first.id FROM service_proposals first WHERE first.conversation_id = sp.conversation_id ORDER BY first.id LIMIT 1))
)
SELECT o.started_on,
 jr.id, jr.status, jr.title, jr.description, jr.created_on,
 sp.id, sp.status, sp.created_on,
 consumer_user.id, consumer_user.name, consumer_user.surname,
 provider_user.id, provider_user.name, provider_user.surname, provider_category.id, provider_category.name,
 address.street, address.street_number, address.floor, address.unit,
 assessment.id, assessment.version, assessment.outcome, category.id, category.name,
 assessment.problem_title, assessment.problem_description, assessment.based_on_message_id, assessment.created_on
FROM selected o
LEFT JOIN job_requests jr ON jr.id = o.job_request_id
LEFT JOIN service_proposals sp ON sp.id = o.proposal_id
JOIN users consumer_user ON consumer_user.id = o.consumer_id
JOIN users provider_user ON provider_user.id = o.provider_id
JOIN providers provider ON provider.user_id = provider_user.id
LEFT JOIN categories provider_category ON provider_category.id = provider.category_id
LEFT JOIN consumer_addresses address ON address.consumer_id = o.consumer_id
LEFT JOIN problem_assessments assessment ON assessment.id = jr.source_assessment_id
LEFT JOIN categories category ON category.id = assessment.problem_category_id`

type OperationDetailReader struct{ db *sql.DB }

func NewOperationDetailReader(db *sql.DB) *OperationDetailReader {
	return &OperationDetailReader{db: db}
}

func (reader *OperationDetailReader) FindByID(ctx context.Context, id readmodel.ID) (*readmodel.OperationDetail, error) {
	var found readmodel.OperationDetail
	found.ID = id
	var jrID, spID, assessmentID, categoryID, providerCategoryID, assessmentVersion, baseMessageID sql.NullInt64
	var jrStatus, jrTitle, jrDescription, spStatus sql.NullString
	var street, streetNumber, floor, unit sql.NullString
	var outcome, categoryName, providerCategoryName, assessmentTitle, assessmentDescription sql.NullString
	var jrCreatedOn, spCreatedOn, assessmentCreatedOn sql.NullTime
	err := reader.db.QueryRowContext(ctx, operationDetailSQL, string(id.Kind), id.ResourceID).Scan(
		&found.StartedOn,
		&jrID, &jrStatus, &jrTitle, &jrDescription, &jrCreatedOn,
		&spID, &spStatus, &spCreatedOn,
		&found.Consumer.ID, &found.Consumer.Name, &found.Consumer.Surname,
		&found.Provider.ID, &found.Provider.Name, &found.Provider.Surname, &providerCategoryID, &providerCategoryName,
		&street, &streetNumber, &floor, &unit,
		&assessmentID, &assessmentVersion, &outcome, &categoryID, &categoryName,
		&assessmentTitle, &assessmentDescription, &baseMessageID, &assessmentCreatedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, operation.ErrOperationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying operation detail: %w", err)
	}
	found.StartedOn = found.StartedOn.UTC()
	if jrID.Valid {
		found.JobRequest = &readmodel.DetailJobRequest{ID: int(jrID.Int64), Status: jrStatus.String, Title: jrTitle.String, Description: jrDescription.String, CreatedOn: jrCreatedOn.Time.UTC()}
	}
	if spID.Valid {
		found.ServiceProposal = &readmodel.DetailProposal{ID: int(spID.Int64), Status: spStatus.String, CreatedOn: spCreatedOn.Time.UTC()}
	}
	if providerCategoryID.Valid {
		found.Category = &readmodel.Category{ID: int(providerCategoryID.Int64), Name: providerCategoryName.String}
	}
	if street.Valid {
		found.Address = &readmodel.CurrentConsumerAddress{Street: street.String, StreetNumber: streetNumber.String, Floor: nullStringPointer(floor), Unit: nullStringPointer(unit)}
	}
	if assessmentID.Valid {
		found.SourceAssessment = &readmodel.SourceAssessment{ID: int(assessmentID.Int64), Version: int(assessmentVersion.Int64), Outcome: outcome.String, Title: assessmentTitle.String, Description: assessmentDescription.String, BasedOnMessageID: int(baseMessageID.Int64), CreatedOn: assessmentCreatedOn.Time.UTC()}
		if categoryID.Valid {
			found.SourceAssessment.Category = &readmodel.Category{ID: int(categoryID.Int64), Name: categoryName.String}
		}
	}
	return &found, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
