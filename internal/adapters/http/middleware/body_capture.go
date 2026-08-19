package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maximumLoggedBodyBytes = 8 * 1024

var errAdditionalJSONValue = errors.New("additional JSON value")

var sensitiveJSONFields = map[string]struct{}{
	"access_token":  {},
	"authorization": {},
	"client_secret": {},
	"code":          {},
	"code_verifier": {},
	"headers":       {},
	"id_token":      {},
	"password":      {},
	"refresh_token": {},
	"signature":     {},
	"state":         {},
	"token":         {},
	"upload_url":    {},
}

type limitedBodyCapture struct {
	buffer    bytes.Buffer
	truncated bool
}

type capturedRequestBody struct {
	io.Reader
	io.Closer
}

type capturedResponseWriter struct {
	gin.ResponseWriter
	body *limitedBodyCapture
}

func captureRequestBody(request *http.Request) *limitedBodyCapture {
	if request.Body == nil || !isJSONContentType(request.Header.Get("Content-Type")) {
		return nil
	}

	capture := &limitedBodyCapture{}
	request.Body = &capturedRequestBody{
		Reader: io.TeeReader(request.Body, capture),
		Closer: request.Body,
	}
	return capture
}

func captureResponseBody(writer gin.ResponseWriter) (*capturedResponseWriter, *limitedBodyCapture) {
	capture := &limitedBodyCapture{}
	return &capturedResponseWriter{ResponseWriter: writer, body: capture}, capture
}

func (capture *limitedBodyCapture) Write(data []byte) (int, error) {
	remaining := maximumLoggedBodyBytes - capture.buffer.Len()
	if remaining <= 0 {
		capture.truncated = true
		return len(data), nil
	}
	if len(data) > remaining {
		_, _ = capture.buffer.Write(data[:remaining])
		capture.truncated = true
		return len(data), nil
	}
	_, _ = capture.buffer.Write(data)
	return len(data), nil
}

func (writer *capturedResponseWriter) Write(data []byte) (int, error) {
	_, _ = writer.body.Write(data)
	return writer.ResponseWriter.Write(data)
}

func (writer *capturedResponseWriter) WriteString(data string) (int, error) {
	_, _ = writer.body.Write([]byte(data))
	return writer.ResponseWriter.WriteString(data)
}

func capturedBodyAttributes(prefix string, capture *limitedBodyCapture, contentType string) []any {
	if capture == nil || capture.buffer.Len() == 0 || !isJSONContentType(contentType) {
		return nil
	}
	if capture.truncated {
		return []any{prefix + "_body_truncated", true}
	}

	decoder := json.NewDecoder(bytes.NewReader(capture.buffer.Bytes()))
	decoder.UseNumber()
	var body any
	if err := decoder.Decode(&body); err != nil {
		return []any{prefix + "_body_invalid_json", true}
	}
	if err := ensureJSONDocumentEnded(decoder); err != nil {
		return []any{prefix + "_body_invalid_json", true}
	}
	return []any{prefix + "_body", redactJSONValue(body)}
}

func ensureJSONDocumentEnded(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errAdditionalJSONValue
	}
	return err
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, child := range typed {
			if _, sensitive := sensitiveJSONFields[normalizeJSONField(key)]; sensitive {
				redacted[key] = "[REDACTED]"
				continue
			}
			redacted[key] = redactJSONValue(child)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for index, child := range typed {
			redacted[index] = redactJSONValue(child)
		}
		return redacted
	default:
		return value
	}
}

func normalizeJSONField(field string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(field)))
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}
