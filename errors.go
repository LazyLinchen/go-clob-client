package clobclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError 表示服务端返回的非 2xx HTTP 错误。
type APIError struct {
	StatusCode int
	Method     string
	URL        string
	Message    string
	Body       string
}

// Error 返回格式化后的 API 错误信息。
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("polymarket API error: %s %s returned %d: %s", e.Method, e.URL, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("polymarket API error: %s %s returned %d", e.Method, e.URL, e.StatusCode)
}

// newAPIError 将失败响应转换为结构化错误，便于上层保留状态码和响应体。
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

// extractErrorMessage 尝试从多种常见 JSON 错误体结构中提取可读消息。
func extractErrorMessage(body string, fallback string) string {
	if body == "" {
		return fallback
	}
	if strings.HasPrefix(strings.TrimSpace(body), "<") {
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
