package operation_detail_handler

import (
	"encoding/json"
	"testing"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestResponseKeepsRelatedProposalTermsAndEmptyCollections(t *testing.T) {
	created := time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)
	proposal := readmodel.DetailProposal{ID: 2, Status: "pending", Description: "Repair", AmountCents: 12000000,
		Currency: "ARS", CreatedOn: created, ScheduledOn: created.Add(time.Hour), EstimatedDurationMinutes: 120,
		DepositCents: 2400000, PlatformFeeTotalCents: 500000, PlatformFeeDueNowCents: 100000}
	result := responseFromDomain(&readmodel.OperationDetail{ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 2},
		StartedOn: created, ServiceProposal: &proposal, RelatedProposals: []readmodel.RelatedProposal{
			{OperationID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 2}, Proposal: proposal},
		}})
	data, err := json.Marshal(result)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	related := body["related_proposals"].([]any)[0].(map[string]any)
	require.Equal(t, "sp-2", related["operation_id"])
	require.Equal(t, float64(12000000), related["amount_cents"])
	require.Equal(t, float64(9600000), related["service_balance_cents"])
	require.Equal(t, []any{}, body["payment_milestones"])
	require.Equal(t, []any{}, body["timeline"])
}

func TestResponseExposesPrivateImageMetadataAndReportWithoutReviewTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	image := readmodel.PrivateImage{ID: "file-1", OriginalName: "evidence.jpg", MimeType: "image/jpeg", Purpose: "job_request_image", CreatedOn: now}
	found := &readmodel.OperationDetail{ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 1}, StartedOn: now,
		JobRequest: &readmodel.DetailJobRequest{ID: 1, Images: []readmodel.PrivateImage{image}},
		WorkOrder:  &readmodel.DetailWorkOrder{ID: 2, CompletionReport: &readmodel.CompletionReport{Description: "Done", ReportedOn: now, Images: []readmodel.PrivateImage{image}}, Review: &readmodel.WorkOrderReview{Rating: 5, Description: "Great"}}}
	data, err := json.Marshal(responseFromDomain(found))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	request := body["job_request"].(map[string]any)
	images := request["images"].([]any)
	require.Len(t, images, 1)
	require.Equal(t, map[string]any{"file_id": "file-1", "original_name": "evidence.jpg", "mime_type": "image/jpeg", "purpose": "job_request_image", "created_on": now.Format(time.RFC3339)}, images[0])
	order := body["work_order"].(map[string]any)
	require.Len(t, order["completion_report"].(map[string]any)["images"].([]any), 1)
	require.Equal(t, map[string]any{"rating": float64(5), "description": "Great"}, order["review"])
	require.NotContains(t, string(data), "storage")
	require.NotContains(t, string(data), "url")
}
