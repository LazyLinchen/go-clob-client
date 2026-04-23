package clobclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// HeaderPolyAddress 是 Polymarket 鉴权请求中的钱包地址头。
	HeaderPolyAddress = "POLY_ADDRESS"
	// HeaderPolySignature 是 Polymarket 鉴权请求中的签名头。
	HeaderPolySignature = "POLY_SIGNATURE"
	// HeaderPolyTimestamp 是 Polymarket 鉴权请求中的时间戳头。
	HeaderPolyTimestamp = "POLY_TIMESTAMP"
	// HeaderPolyNonce 是 L1 鉴权请求中的 nonce 头。
	HeaderPolyNonce = "POLY_NONCE"
	// HeaderPolyAPIKey 是 L2 鉴权请求中的 API Key 头。
	HeaderPolyAPIKey = "POLY_API_KEY"
	// HeaderPolyPassphrase 是 L2 鉴权请求中的口令头。
	HeaderPolyPassphrase = "POLY_PASSPHRASE"
)

// APICredentials 表示 Polymarket L2 API 使用的一组凭证。
type APICredentials struct {
	// Key 是 SDK 中常见的 API Key 字段名。
	Key string
	// APIKey 是部分接口响应中使用的别名字段。
	APIKey string
	// Secret 是生成 L2 HMAC 签名所需的密钥。
	Secret string
	// Passphrase 是 L2 鉴权请求中的口令字段。
	Passphrase string
}

// UnmarshalJSON 兼容 apiKey、api_key 和 key 等不同返回字段名。
func (c *APICredentials) UnmarshalJSON(data []byte) error {
	var wire struct {
		APIKey     string `json:"apiKey"`
		APIKeyAlt  string `json:"api_key"`
		Key        string `json:"key"`
		Secret     string `json:"secret"`
		Passphrase string `json:"passphrase"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.Key != "":
		c.Key = wire.Key
	case wire.APIKey != "":
		c.Key = wire.APIKey
	default:
		c.Key = wire.APIKeyAlt
	}
	c.APIKey = c.Key
	c.Secret = wire.Secret
	c.Passphrase = wire.Passphrase
	return nil
}

func (c *APICredentials) credentialKey() string {
	if c == nil {
		return ""
	}
	if c.Key != "" {
		return c.Key
	}
	return c.APIKey
}

// normalizedCredentials 统一 Key 与 APIKey 的别名，避免后续使用方分支判断。
func normalizedCredentials(creds *APICredentials) *APICredentials {
	if creds == nil {
		return nil
	}
	cloned := *creds
	if cloned.Key == "" {
		cloned.Key = cloned.APIKey
	}
	if cloned.APIKey == "" {
		cloned.APIKey = cloned.Key
	}
	return &cloned
}

// L1AuthParams 描述生成 L1 鉴权头时需要的附加参数。
type L1AuthParams struct {
	// Nonce 会写入 EIP-712 消息体，用于防重放。
	Nonce uint64
	// Timestamp 允许调用方显式指定签名时间，留空时按客户端配置生成。
	Timestamp int64
}

// L2HeaderArgs 描述生成 L2 鉴权头时需要参与签名的请求信息。
type L2HeaderArgs struct {
	// Method 是 HTTP 方法，例如 GET、POST。
	Method string
	// RequestPath 是参与签名的路径，必须以 / 开头。
	RequestPath string
	// Body 是参与 HMAC 签名的原始请求体。
	Body []byte
	// Timestamp 允许调用方显式指定签名时间，留空时按客户端配置生成。
	Timestamp int64
}

// BuildL1Headers 根据钱包签名生成 L1 鉴权请求头。
func (c *Client) BuildL1Headers(ctx context.Context, params L1AuthParams) (http.Header, error) {
	if c.signer == nil {
		return nil, errors.New("L1 signer is not configured")
	}

	timestamp := params.Timestamp
	if timestamp == 0 {
		if c.useServerTime {
			var err error
			// 使用服务端时间可以减少本地时钟偏差导致的鉴权失败。
			timestamp, err = c.GetServerTime(ctx)
			if err != nil {
				return nil, fmt.Errorf("get server time for L1 auth: %w", err)
			}
		} else {
			timestamp = time.Now().Unix()
		}
	}

	typedData := BuildClobAuthTypedData(c.signer.Address(), c.chainID, timestamp, params.Nonce)
	signature, err := c.signer.SignTypedData(ctx, typedData)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header, 4)
	headers.Set(HeaderPolyAddress, c.signer.Address())
	headers.Set(HeaderPolySignature, signature)
	headers.Set(HeaderPolyTimestamp, fmt.Sprintf("%d", timestamp))
	headers.Set(HeaderPolyNonce, fmt.Sprintf("%d", params.Nonce))
	return headers, nil
}

// CreateAPIKey 创建一组新的 L2 API 凭证。
func (c *Client) CreateAPIKey(ctx context.Context, params L1AuthParams) (*APICredentials, error) {
	headers, err := c.BuildL1Headers(ctx, params)
	if err != nil {
		return nil, err
	}

	var creds APICredentials
	if err := c.doJSON(ctx, http.MethodPost, "/auth/api-key", nil, nil, headers, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// DeriveAPIKey 派生当前钱包对应的 L2 API 凭证。
func (c *Client) DeriveAPIKey(ctx context.Context, params L1AuthParams) (*APICredentials, error) {
	headers, err := c.BuildL1Headers(ctx, params)
	if err != nil {
		return nil, err
	}

	var creds APICredentials
	if err := c.doJSON(ctx, http.MethodGet, "/auth/derive-api-key", nil, nil, headers, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// CreateOrDeriveAPIKey 优先创建 API Key，失败后在可恢复错误上回退到派生流程。
func (c *Client) CreateOrDeriveAPIKey(ctx context.Context, params L1AuthParams) (*APICredentials, error) {
	creds, err := c.CreateAPIKey(ctx, params)
	if err == nil {
		return creds, nil
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) || !shouldFallbackToDerive(apiErr) {
		return nil, err
	}

	derivedCreds, deriveErr := c.DeriveAPIKey(ctx, params)
	if deriveErr != nil {
		return nil, errors.Join(err, deriveErr)
	}
	return derivedCreds, nil
}

func shouldFallbackToDerive(err *APIError) bool {
	return err.StatusCode >= http.StatusBadRequest && err.StatusCode < http.StatusInternalServerError
}

// BuildL2Headers 根据 API 凭证和请求信息生成 L2 鉴权请求头。
func (c *Client) BuildL2Headers(ctx context.Context, args L2HeaderArgs) (http.Header, error) {
	if c.signer == nil {
		return nil, errors.New("l1 signer is not configured")
	}
	if c.creds == nil {
		return nil, errors.New("L2 credentials are not configured")
	}

	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		return nil, errors.New("L2 method is required")
	}

	requestPath := strings.TrimSpace(args.RequestPath)
	if requestPath == "" {
		return nil, errors.New("L2 request path is required")
	}
	if !strings.HasPrefix(requestPath, "/") {
		return nil, fmt.Errorf("L2 request path %q must start with /", requestPath)
	}

	timestamp := args.Timestamp
	if timestamp == 0 {
		if c.useServerTime {
			var err error
			// L2 鉴权同样可能受到本地时钟漂移影响。
			timestamp, err = c.GetServerTime(ctx)
			if err != nil {
				return nil, fmt.Errorf("get server time for L2 auth: %w", err)
			}
		} else {
			timestamp = time.Now().Unix()
		}
	}

	signature, err := buildPolyHMACSignature(c.creds.Secret, timestamp, method, requestPath, args.Body)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header, 5)
	headers.Set(HeaderPolyAddress, c.signer.Address())
	headers.Set(HeaderPolySignature, signature)
	headers.Set(HeaderPolyTimestamp, fmt.Sprintf("%d", timestamp))
	headers.Set(HeaderPolyAPIKey, c.creds.credentialKey())
	headers.Set(HeaderPolyPassphrase, c.creds.Passphrase)
	return headers, nil
}

func (c *Client) doL2JSON(
	ctx context.Context,
	method string,
	requestPath string,
	query url.Values,
	body []byte,
	dest any,
) error {
	headers, err := c.BuildL2Headers(ctx, L2HeaderArgs{
		Method:      method,
		RequestPath: requestPath,
		Body:        body,
	})
	if err != nil {
		return err
	}

	var requestBody any
	if body != nil {
		requestBody = body
	}

	return c.doJSON(ctx, method, requestPath, query, requestBody, headers, dest)
}

func buildPolyHMACSignature(secret string, timestamp int64, method string, requestPath string, body []byte) (string, error) {
	decodedSecret, err := decodePolySecret(secret)
	if err != nil {
		return "", fmt.Errorf("decode L2 secret: %w", err)
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("%d", timestamp))
	messageBuilder.WriteString(method)
	messageBuilder.WriteString(requestPath)
	if body != nil {
		messageBuilder.Write(body)
	}

	mac := hmac.New(sha256.New, decodedSecret)
	if _, err := mac.Write([]byte(messageBuilder.String())); err != nil {
		return "", fmt.Errorf("write HMAC payload: %w", err)
	}

	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	// Polymarket 的签名头使用 URL-safe 变体，因此这里手动替换字符。
	signature = strings.ReplaceAll(signature, "+", "-")
	signature = strings.ReplaceAll(signature, "/", "_")
	return signature, nil
}

// decodePolySecret 尝试按多种 Base64 变体解码服务端返回的 secret。
func decodePolySecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("secret is empty")
	}

	var decodeErr error
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(secret)
		if err == nil {
			return decoded, nil
		}
		decodeErr = err
	}

	return nil, decodeErr
}
