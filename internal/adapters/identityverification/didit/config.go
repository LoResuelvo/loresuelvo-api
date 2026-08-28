package didit

import (
	"os"
	"time"

	"github.com/google/uuid"
)

const defaultBaseURL = "https://verification.didit.me"

func NewClientFromEnv() (*Client, error) {
	workflowID, err := uuid.Parse(os.Getenv("DIDIT_WORKFLOW_ID"))
	if err != nil {
		return nil, err
	}
	timeout, err := time.ParseDuration(os.Getenv("DIDIT_HTTP_TIMEOUT"))
	if err != nil {
		return nil, err
	}
	return NewClient(Config{APIKey: os.Getenv("DIDIT_API_KEY"), WorkflowID: workflowID, BaseURL: defaultBaseURL, Timeout: timeout})
}

func NewWebhookAdapterFromEnv() (*WebhookAdapter, error) {
	return NewWebhookAdapter(os.Getenv("DIDIT_WEBHOOK_SECRET"))
}
