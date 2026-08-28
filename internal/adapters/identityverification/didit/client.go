package didit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

const verifierName = "didit"

type Config struct {
	APIKey     string
	WorkflowID uuid.UUID
	BaseURL    string
	Timeout    time.Duration
}

type Client struct {
	apiKey     string
	workflowID uuid.UUID
	baseURL    string
	httpClient *http.Client
}

type createSessionRequest struct {
	WorkflowID      uuid.UUID       `json:"workflow_id"`
	VendorData      string          `json:"vendor_data"`
	ExpectedDetails expectedDetails `json:"expected_details"`
}

type expectedDetails struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type createSessionResponse struct {
	SessionID       uuid.UUID `json:"session_id"`
	SessionToken    string    `json:"session_token"`
	URL             string    `json:"url"`
	Status          string    `json:"status"`
	WorkflowID      uuid.UUID `json:"workflow_id"`
	WorkflowVersion int       `json:"workflow_version"`
}

func NewClient(config Config) (*Client, error) {
	if strings.TrimSpace(config.APIKey) == "" || config.WorkflowID == uuid.Nil || strings.TrimSpace(config.BaseURL) == "" || config.Timeout <= 0 {
		return nil, identityverification.ErrVerifierMisconfigured
	}
	return &Client{apiKey: strings.TrimSpace(config.APIKey), workflowID: config.WorkflowID, baseURL: strings.TrimRight(config.BaseURL, "/"), httpClient: &http.Client{Timeout: config.Timeout}}, nil
}

func (client *Client) CreateSession(ctx context.Context, request identityverification.SessionRequest) (identityverification.SessionCredentials, error) {
	payload, err := json.Marshal(createSessionRequest{WorkflowID: client.workflowID, VendorData: request.VendorData, ExpectedDetails: expectedDetails{FirstName: request.FirstName, LastName: request.LastName}})
	if err != nil {
		return identityverification.SessionCredentials{}, fmt.Errorf("encoding Didit session request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v3/session/", bytes.NewReader(payload))
	if err != nil {
		return identityverification.SessionCredentials{}, fmt.Errorf("creating Didit session request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", client.apiKey)
	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return identityverification.SessionCredentials{}, fmt.Errorf("creating Didit session: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		return identityverification.SessionCredentials{}, identityverification.ErrVerifierUnavailable
	}
	var response createSessionResponse
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 1<<20))
	if err := decoder.Decode(&response); err != nil {
		return identityverification.SessionCredentials{}, identityverification.ErrVerifierMisconfigured
	}
	if response.SessionID == uuid.Nil || strings.TrimSpace(response.SessionToken) == "" || strings.TrimSpace(response.URL) == "" || response.WorkflowID != client.workflowID || response.Status != "Not Started" {
		return identityverification.SessionCredentials{}, identityverification.ErrVerifierMisconfigured
	}
	return identityverification.SessionCredentials{SessionID: response.SessionID, SessionToken: response.SessionToken, VerificationURL: response.URL, Status: identityverification.StatusNotStarted, Verifier: verifierName, WorkflowID: response.WorkflowID, WorkflowVersion: response.WorkflowVersion}, nil
}
