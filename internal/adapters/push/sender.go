package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
)

var ErrInvalidToken = errors.New("FCM registration token invalid")

type Sender struct {
	Client  *http.Client
	URL     string
	Timeout time.Duration
}

func NewSenderFromEnv() (*Sender, error) {
	enabled := strings.TrimSpace(os.Getenv("FCM_ENABLED"))
	if enabled == "" || enabled == "false" {
		return nil, nil
	}
	if enabled != "true" {
		return nil, fmt.Errorf("FCM_ENABLED must be true or false")
	}
	project := strings.TrimSpace(os.Getenv("FCM_PROJECT_ID"))
	if project == "" || strings.ContainsAny(project, "/ ?#") {
		return nil, fmt.Errorf("FCM_PROJECT_ID is required and must identify a project")
	}
	timeout := 5 * time.Second
	if value := os.Getenv("FCM_TIMEOUT"); value != "" {
		var err error
		timeout, err = time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid FCM_TIMEOUT")
		}
	}
	if timeout < time.Second || timeout > 30*time.Second {
		return nil, fmt.Errorf("FCM_TIMEOUT must be between 1s and 30s")
	}
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{Scopes: []string{"https://www.googleapis.com/auth/firebase.messaging"}})
	if err != nil {
		return nil, fmt.Errorf("FCM credentials unavailable")
	}
	client, err := httptransport.NewClient(&httptransport.Options{Credentials: creds, DisableTelemetry: true})
	if err != nil {
		return nil, fmt.Errorf("configuring FCM authentication")
	}
	client.Timeout = timeout
	return &Sender{Client: client, URL: "https://fcm.googleapis.com/v1/projects/" + project + "/messages:send", Timeout: timeout}, nil
}
func (s *Sender) Send(ctx context.Context, token string, data map[string]string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()
	body, err := json.Marshal(map[string]any{"message": map[string]any{"token": token, "data": data, "android": map[string]string{"priority": "high", "ttl": strconv.FormatInt(int64(ttl/time.Second), 10) + "s"}}})
	if err != nil {
		return fmt.Errorf("encoding FCM request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating FCM request")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.Client.Do(request)
	if err != nil {
		return fmt.Errorf("FCM transport failed")
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, 65536)); err != nil {
			return fmt.Errorf("reading FCM response")
		}
		return nil
	}
	body, err = io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil {
		return fmt.Errorf("reading FCM response")
	}
	if len(body) <= 65536 && invalidRegistrationToken(response.StatusCode, body) {
		return ErrInvalidToken
	}
	return fmt.Errorf("FCM rejected request: status %d", response.StatusCode)
}
func (s *Sender) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 5 * time.Second
}
func invalidRegistrationToken(status int, body []byte) bool {
	var response struct {
		Error struct {
			Details []struct {
				Type string `json:"@type"`
				Code string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false
	}
	invalid := false
	for _, detail := range response.Error.Details {
		if detail.Type == "type.googleapis.com/google.rpc.BadRequest" {
			return false
		}
		if detail.Type == "type.googleapis.com/google.firebase.fcm.v1.FcmError" && ((status == http.StatusNotFound && detail.Code == "UNREGISTERED") || (status == http.StatusBadRequest && detail.Code == "INVALID_ARGUMENT")) {
			invalid = true
		}
	}
	return invalid
}
