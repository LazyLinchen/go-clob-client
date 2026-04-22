package clobclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type APIError struct {
	StatusCode int
	Method     string
	URL        string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("polymarket API error: %s %s returned %d: %s", e.Method, e.URL, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("polymarket API error: %s %s returned %d", e.Method, e.URL, e.StatusCode)
}

func newAPIError(req *http.Request, resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	body := strings.TrimSpace(string(bodyBytes))

	return &APIError{
		StatusCode: resp.StatusCode,
		Method:     req.Method,
		URL:        req.URL.Redacted(),
		Message:    extractErrorMessage(body, resp.Status),
		Body:       body,
	}
}

func extractErrorMessage(body string, fallback string) string {
	if body == "" {
		return fallback
	}

	var stringBody string
	if err := json.Unmarshal([]byte(body), &stringBody); err == nil {
		stringBody = strings.TrimSpace(stringBody)
		if stringBody != "" {
			return stringBody
		}
	}

	var objectBody map[string]any
	if err := json.Unmarshal([]byte(body), &objectBody); err == nil {
		for _, key := range []string{"error", "message", "msg", "detail"} {
			if value, ok := objectBody[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}

	return body
}
