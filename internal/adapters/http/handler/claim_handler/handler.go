package claim_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
)

type service interface {
	Submit(context.Context, string, string, claim.Submission) (*claim.SubmissionResult, error)
	List(context.Context, string, claim.ListCriteria) (*claim.Page, error)
	Get(context.Context, string, int) (*claim.GetResult, error)
}
type Handler struct{ service service }

func NewHandler(s service) *Handler { return &Handler{service: s} }
func respondError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	for _, entry := range []struct {
		err    error
		status int
	}{
		{claim.ErrInvalidResolution, 400}, {claim.ErrInvalidTransition, 409}, {claim.ErrAdministrationKeyConflict, 409}, {claim.ErrEvidenceAccessUnavailable, 500}, {filedomain.ErrClaimEvidenceImageNotAvailable, 400}, {claim.ErrInvalidSubmission, 400}, {claim.ErrInvalidIdempotencyKey, 400}, {claim.ErrInvalidCriteria, 400}, {claim.ErrInvalidEvidence, 400}, {claim.ErrForbidden, 403}, {claim.ErrNotFound, 404}, {claim.ErrSubmissionKeyConflict, 409}, {claim.ErrOpenClaimConflict, 409},
	} {
		if errors.Is(err, entry.err) {
			status = entry.status
			if status != http.StatusInternalServerError {
				message = entry.err.Error()
			}
			break
		}
	}
	httphandler.RespondError(c, status, message)
	c.Abort()
}

type referenceRequest struct {
	JobRequestID      *int    `json:"job_request_id"`
	ServiceProposalID *int    `json:"service_proposal_id"`
	WorkOrderID       *int    `json:"work_order_id"`
	PaymentIntentID   *string `json:"payment_intent_id"`
}

// strictObject preserves the distinction between absent and null fields and rejects ambiguous duplicate members.
func strictObject(data []byte, allowed ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, claim.ErrInvalidSubmission
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, claim.ErrInvalidSubmission
		}
		key, ok := token.(string)
		if !ok || !slices.Contains(allowed, key) {
			return nil, claim.ErrInvalidSubmission
		}
		if _, exists := fields[key]; exists {
			return nil, claim.ErrInvalidSubmission
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, claim.ErrInvalidSubmission
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, claim.ErrInvalidSubmission
	}
	return fields, nil
}
func (r *referenceRequest) UnmarshalJSON(data []byte) error {
	fields, err := strictObject(data, "job_request_id", "service_proposal_id", "work_order_id", "payment_intent_id")
	if err != nil || len(fields) != 1 {
		return claim.ErrInvalidSubmission
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return claim.ErrInvalidSubmission
		}
		switch key {
		case "job_request_id":
			err = json.Unmarshal(value, &r.JobRequestID)
		case "service_proposal_id":
			err = json.Unmarshal(value, &r.ServiceProposalID)
		case "work_order_id":
			err = json.Unmarshal(value, &r.WorkOrderID)
		case "payment_intent_id":
			err = json.Unmarshal(value, &r.PaymentIntentID)
		}
		if err != nil {
			return claim.ErrInvalidSubmission
		}
	}
	return nil
}

type submissionRequest struct {
	Reference    referenceRequest `json:"reference"`
	Reason       claim.Reason     `json:"reason"`
	Description  string           `json:"description"`
	ImageFileIDs json.RawMessage  `json:"image_file_ids"`
}

func (r *submissionRequest) UnmarshalJSON(data []byte) error {
	fields, err := strictObject(data, "reference", "reason", "description", "image_file_ids")
	if err != nil {
		return err
	}
	for key, value := range fields {
		switch key {
		case "reference":
			err = json.Unmarshal(value, &r.Reference)
		case "reason":
			err = json.Unmarshal(value, &r.Reason)
		case "description":
			err = json.Unmarshal(value, &r.Description)
		case "image_file_ids":
			r.ImageFileIDs = value
		}
		if err != nil {
			return claim.ErrInvalidSubmission
		}
	}
	return nil
}
func (r referenceRequest) domain() (claim.Reference, error) {
	count := 0
	var kind claim.ReferenceKind
	var id string
	for _, entry := range []struct {
		kind  claim.ReferenceKind
		value *int
	}{{claim.ReferenceKindJobRequest, r.JobRequestID}, {claim.ReferenceKindServiceProposal, r.ServiceProposalID}, {claim.ReferenceKindWorkOrder, r.WorkOrderID}} {
		if entry.value != nil {
			count++
			kind = entry.kind
			id = strconv.Itoa(*entry.value)
		}
	}
	if r.PaymentIntentID != nil {
		count++
		kind = claim.ReferenceKindPaymentIntent
		id = *r.PaymentIntentID
	}
	if count != 1 {
		return claim.Reference{}, claim.ErrInvalidSubmission
	}
	return claim.NewReference(kind, id)
}
func decodeSubmission(c *gin.Context) (claim.Submission, error) {
	var request submissionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return claim.Submission{}, claim.ErrInvalidSubmission
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return claim.Submission{}, claim.ErrInvalidSubmission
	}
	ref, err := request.Reference.domain()
	if err != nil {
		return claim.Submission{}, err
	}
	images := []string{}
	if request.ImageFileIDs != nil {
		if bytes.Equal(bytes.TrimSpace(request.ImageFileIDs), []byte("null")) {
			return claim.Submission{}, claim.ErrInvalidSubmission
		}
		if err := json.Unmarshal(request.ImageFileIDs, &images); err != nil {
			return claim.Submission{}, claim.ErrInvalidSubmission
		}
	}
	return (claim.Submission{Reference: ref, Reason: request.Reason, Description: request.Description, ImageFileIDs: images}).Normalize()
}
func (h *Handler) Submit(c *gin.Context) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	if len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		respondError(c, claim.ErrInvalidIdempotencyKey)
		return
	}
	key, err := claim.NormalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	if err != nil {
		respondError(c, err)
		return
	}
	input, err := decodeSubmission(c)
	if err != nil {
		respondError(c, err)
		return
	}
	result, err := h.service.Submit(c.Request.Context(), auth, key, input)
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		c.Header("Location", "/claims/"+strconv.Itoa(result.Claim.ID))
	}
	c.JSON(status, acknowledge(result.Claim))
}
func (h *Handler) List(c *gin.Context) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	criteria, err := parseCriteria(c)
	if err != nil {
		respondError(c, err)
		return
	}
	page, err := h.service.List(c.Request.Context(), auth, criteria)
	if err != nil {
		respondError(c, err)
		return
	}
	items := make([]summaryResponse, 0, len(page.Claims))
	for _, item := range page.Claims {
		items = append(items, summary(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page.Page, "limit": page.Limit, "total": page.Total})
}
func parseCriteria(c *gin.Context) (claim.ListCriteria, error) {
	return parseCriteriaValues(c.Request.URL.Query())
}
func parseCriteriaValues(values url.Values) (claim.ListCriteria, error) {
	criteria := claim.ListCriteria{Page: 1, Limit: 20}
	for key, values := range values {
		if len(values) != 1 {
			return criteria, claim.ErrInvalidCriteria
		}
		switch key {
		case "page", "limit":
			v, err := strconv.Atoi(values[0])
			if err != nil || v < 1 {
				return criteria, claim.ErrInvalidCriteria
			}
			if key == "page" {
				criteria.Page = v
			} else {
				criteria.Limit = v
			}
		case "status":
			v := claim.Status(values[0])
			criteria.Status = &v
		default:
			return criteria, claim.ErrInvalidCriteria
		}
	}
	return criteria.Normalize()
}
func claimID(c *gin.Context) (int, error) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != raw {
		return 0, claim.ErrNotFound
	}

	return int(id), nil
}
func (h *Handler) Get(c *gin.Context) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	id, err := claimID(c)
	if err != nil {
		respondError(c, err)
		return
	}
	found, err := h.service.Get(c.Request.Context(), auth, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail(found))
}

type acknowledgment struct {
	ID          int            `json:"id"`
	OperationID string         `json:"operation_id"`
	Reference   map[string]any `json:"reference"`
	Status      claim.Status   `json:"status"`
	CreatedOn   time.Time      `json:"created_on"`
}

func reference(kind, id string) map[string]any {
	var value any = id
	if kind != string(claim.ReferenceKindPaymentIntent) {
		number, err := strconv.Atoi(id)
		if err == nil {
			value = number
		}
	}
	return map[string]any{kind + "_id": value}
}
func acknowledge(c *claim.Claim) acknowledgment {
	return acknowledgment{ID: c.ID, OperationID: fmt.Sprintf("%s-%d", c.OperationID.Kind, c.OperationID.ResourceID), Reference: reference(string(c.Reference.Kind()), c.Reference.ID()), Status: c.Status, CreatedOn: c.CreatedOn}
}

type summaryResponse struct {
	acknowledgment
	Reason          string     `json:"reason"`
	ReviewStartedOn *time.Time `json:"review_started_on"`
	ClosedOn        *time.Time `json:"closed_on"`
}

func summary(c readmodel.ClaimSummary) summaryResponse {
	return summaryResponse{acknowledgment: acknowledgment{ID: c.ID, OperationID: fmt.Sprintf("%s-%d", c.OperationID.Kind, c.OperationID.ResourceID), Reference: reference(c.ReferenceKind, c.ReferenceID), Status: claim.Status(c.Status), CreatedOn: c.CreatedOn}, Reason: c.Reason, ReviewStartedOn: c.ReviewStartedOn, ClosedOn: c.ClosedOn}
}

type compensationResponse struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Unit        string `json:"unit"`
}
type resolutionResponse struct {
	Type                  claim.ResolutionType  `json:"type"`
	Reasoning             string                `json:"reasoning"`
	ResolvedOn            time.Time             `json:"resolved_on"`
	SuggestedCompensation *compensationResponse `json:"suggested_compensation"`
}
type detailResponse struct {
	summaryResponse
	Description string                  `json:"description"`
	Images      []evidenceImageResponse `json:"images"`
	Resolution  *resolutionResponse     `json:"resolution"`
}

type evidenceImageResponse struct {
	FileID string `json:"file_id"`
	URL    string `json:"url"`
}

func detail(found *claim.GetResult) detailResponse {
	c := found.Claim
	images := make([]evidenceImageResponse, 0, len(found.Images))
	for _, image := range found.Images {
		images = append(images, evidenceImageResponse{FileID: image.FileID, URL: image.URL})
	}
	result := detailResponse{summaryResponse: summaryResponse{acknowledgment: acknowledge(c), Reason: string(c.Reason), ReviewStartedOn: c.ReviewStartedOn, ClosedOn: c.ClosedOn}, Description: c.Description, Images: images}
	if c.Resolution != nil {
		resolution := c.Resolution
		result.Resolution = &resolutionResponse{Type: resolution.Type, Reasoning: resolution.Reasoning, ResolvedOn: resolution.ResolvedOn}
		if compensation := resolution.SuggestedCompensation; compensation != nil {
			result.Resolution.SuggestedCompensation = &compensationResponse{AmountMinor: compensation.AmountMinor, Currency: compensation.Currency, Unit: compensation.Unit}
		}
	}
	return result
}
