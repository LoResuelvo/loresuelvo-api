package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

var errMissingExternalResponse = errors.New("external requester returned no response")

type externalOperationContextKey struct{}

type HTTPRequester interface {
	Do(request *http.Request) (*http.Response, error)
}

type loggingRoundTripper struct {
	service          string
	defaultOperation string
	next             http.RoundTripper
}

type loggingRequester struct {
	service          string
	defaultOperation string
	next             HTTPRequester
}

func ContextWithExternalOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, externalOperationContextKey{}, operation)
}

func NewLoggingHTTPClient(service, defaultOperation string, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &loggingRoundTripper{
			service:          service,
			defaultOperation: defaultOperation,
			next:             http.DefaultTransport,
		},
		Timeout: timeout,
	}
}

func NewLoggingRequester(service, defaultOperation string, next HTTPRequester) HTTPRequester {
	return &loggingRequester{service: service, defaultOperation: defaultOperation, next: next}
}

func (transport *loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return observeExternalRequest(request, transport.service, transport.defaultOperation, transport.next.RoundTrip)
}

func (requester *loggingRequester) Do(request *http.Request) (*http.Response, error) {
	return observeExternalRequest(request, requester.service, requester.defaultOperation, requester.next.Do)
}

func observeExternalRequest(
	request *http.Request,
	service,
	defaultOperation string,
	do func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	startedOn := time.Now()
	response, err := do(request)

	operation := defaultOperation
	if contextualOperation, ok := request.Context().Value(externalOperationContextKey{}).(string); ok && contextualOperation != "" {
		operation = contextualOperation
	}
	attributes := []any{
		"external.service", service,
		"external.operation", operation,
		"http.method", request.Method,
		"server.address", request.URL.Hostname(),
		"duration_ms", time.Since(startedOn).Milliseconds(),
	}
	logger := LoggerFromContext(request.Context())
	if err != nil {
		logger.ErrorContext(request.Context(), "external.request.completed",
			append(attributes, "error.type", fmt.Sprintf("%T", err))...,
		)
		return nil, err
	}
	if response == nil {
		logger.ErrorContext(request.Context(), "external.request.completed",
			append(attributes, "error.type", fmt.Sprintf("%T", errMissingExternalResponse))...,
		)
		return nil, errMissingExternalResponse
	}

	attributes = append(attributes, "http.status_code", response.StatusCode)
	logger.Log(request.Context(), externalHTTPLogLevel(response.StatusCode), "external.request.completed", attributes...)
	return response, nil
}

func externalHTTPLogLevel(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
