package claim_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/gin-gonic/gin"
)

type adminService interface {
	List(context.Context, claim.AdminCriteria) (*claim.AdminPage, error)
	Get(context.Context, string, int, string) (*claim.AdminDetail, error)
	StartReview(context.Context, string, int, string, string) (*claim.AdministrationResult, error)
	Resolve(context.Context, string, int, string, string, claim.ResolutionInput) (*claim.AdministrationResult, error)
}
type AdminHandler struct{ service adminService }

func NewAdminHandler(s adminService) *AdminHandler { return &AdminHandler{service: s} }

func (h *AdminHandler) List(c *gin.Context) {
	if _, ok := httphandler.GetAuthenticatedUserID(c); !ok {
		return
	}
	values := c.Request.URL.Query()
	queryValues := values["q"]
	query := values.Get("q")
	values.Del("q")
	criteria, err := parseCriteriaValues(values)
	if len(queryValues) > 1 {
		err = claim.ErrInvalidCriteria
	}
	if err != nil {
		respondError(c, err)
		return
	}
	page, err := h.service.List(c.Request.Context(), claim.AdminCriteria{ListCriteria: criteria, Query: query})
	if err != nil {
		respondError(c, err)
		return
	}
	items := make([]adminSummaryResponse, 0, len(page.Claims))
	for _, item := range page.Claims {
		items = append(items, adminSummaryResponse{summaryResponse: summary(item.ClaimSummary), ClaimantID: item.ClaimantID, ClaimantParty: item.ClaimantParty, ClaimantEmail: item.ClaimantEmail, CategoryID: item.CategoryID, CategoryName: item.CategoryName, AgeSeconds: item.AgeSeconds})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page.Page, "limit": page.Limit, "total": page.Total})
}

type adminSummaryResponse struct {
	summaryResponse
	ClaimantID    int     `json:"claimant_id"`
	ClaimantParty string  `json:"claimant_party"`
	ClaimantEmail string  `json:"claimant_email"`
	CategoryID    *int    `json:"category_id"`
	CategoryName  *string `json:"category_name"`
	AgeSeconds    int64   `json:"age_seconds"`
}
type claimantResponse struct {
	ID    int         `json:"id"`
	Party claim.Party `json:"party"`
	Email string      `json:"email"`
}

func adminRequestIdentity(c *gin.Context) (string, int, string, bool) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return "", 0, "", false
	}
	id, err := claimID(c)
	if err != nil {
		respondError(c, err)
		return "", 0, "", false
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return "", 0, "", false
	}
	return auth, id, correlation, true
}

type actionResponse struct {
	ID         int         `json:"id"`
	Type       string      `json:"type"`
	ActorID    int         `json:"actor_id"`
	ActorParty claim.Party `json:"actor_party"`
	CreatedOn  time.Time   `json:"created_on"`
}

func actionDTO(action claim.Action) actionResponse {
	return actionResponse{ID: action.ID, Type: action.Type, ActorID: action.ActorID, ActorParty: action.ActorParty, CreatedOn: action.CreatedOn}
}

func (h *AdminHandler) Get(c *gin.Context) {
	auth, id, correlation, ok := adminRequestIdentity(c)
	if !ok {
		return
	}
	found, err := h.service.Get(c.Request.Context(), auth, id, correlation)
	if err != nil {
		respondError(c, err)
		return
	}
	actions := make([]actionResponse, 0, len(found.Claim.Actions))
	for _, action := range found.Claim.Actions {
		actions = append(actions, actionDTO(action))
	}
	c.JSON(http.StatusOK, struct {
		detailResponse
		Claimant     claimantResponse `json:"claimant"`
		CategoryID   *int             `json:"category_id"`
		CategoryName *string          `json:"category_name"`
		Actions      []actionResponse `json:"actions"`
	}{detail(&found.GetResult), claimantResponse{ID: found.Claim.ClaimantID, Party: found.Claim.ClaimantParty, Email: found.ClaimantEmail}, found.CategoryID, found.CategoryName, actions})
}

func administrationKey(c *gin.Context) (string, error) {
	if len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		return "", claim.ErrInvalidIdempotencyKey
	}
	return claim.NormalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
}

func readAdministrativeBody(c *gin.Context) ([]byte, error) {
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024))
	if err != nil || !utf8.Valid(data) {
		return nil, claim.ErrInvalidSubmission
	}
	return data, nil
}

func (h *AdminHandler) StartReview(c *gin.Context) {
	auth, id, correlation, ok := adminRequestIdentity(c)
	if !ok {
		return
	}
	key, err := administrationKey(c)
	if err != nil {
		respondError(c, err)
		return
	}
	data, err := readAdministrativeBody(c)
	if err != nil {
		respondError(c, err)
		return
	}
	if len(bytes.TrimSpace(data)) != 0 {
		fields, err := httphandler.StrictObject(data)
		if err != nil || len(fields) != 0 || !json.Valid(data) {
			respondError(c, claim.ErrInvalidSubmission)
			return
		}
	}
	result, err := h.service.StartReview(c.Request.Context(), auth, id, key, correlation)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, administrationResponse(result))
}

func decodeResolution(c *gin.Context) (claim.ResolutionInput, error) {
	data, err := readAdministrativeBody(c)
	if err != nil {
		return claim.ResolutionInput{}, err
	}
	if !json.Valid(data) {
		return claim.ResolutionInput{}, claim.ErrInvalidResolution
	}
	fields, err := httphandler.StrictObject(data, "type", "reasoning", "suggested_compensation")
	if err != nil {
		return claim.ResolutionInput{}, claim.ErrInvalidResolution
	}
	var input claim.ResolutionInput
	for name, value := range fields {
		switch name {
		case "type":
			err = json.Unmarshal(value, &input.Type)
		case "reasoning":
			err = json.Unmarshal(value, &input.Reasoning)
		case "suggested_compensation":
			var amountFields map[string]json.RawMessage
			amountFields, err = httphandler.StrictObject(value, "amount_minor")
			if err == nil {
				amount, exists := amountFields["amount_minor"]
				if !exists || bytes.Equal(bytes.TrimSpace(amount), []byte("null")) {
					return claim.ResolutionInput{}, claim.ErrInvalidResolution
				}
				var number int64
				err = json.Unmarshal(amount, &number)
				input.SuggestedAmountMinor = &number
			}
		}
		if err != nil {
			return claim.ResolutionInput{}, claim.ErrInvalidResolution
		}
	}
	return input.Normalize()
}

func (h *AdminHandler) Resolve(c *gin.Context) {
	auth, id, correlation, ok := adminRequestIdentity(c)
	if !ok {
		return
	}
	key, err := administrationKey(c)
	if err != nil {
		respondError(c, err)
		return
	}
	input, err := decodeResolution(c)
	if err != nil {
		respondError(c, err)
		return
	}
	result, err := h.service.Resolve(c.Request.Context(), auth, id, key, correlation, input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, administrationResponse(result))
}

func administrationResponse(result *claim.AdministrationResult) gin.H {
	found := detail(&claim.GetResult{Claim: result.Claim})
	return gin.H{"id": result.Claim.ID, "status": result.Claim.Status, "action": actionDTO(result.Action), "resolution": found.Resolution}
}
