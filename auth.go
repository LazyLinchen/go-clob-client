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
	HeaderPolyAddress    = "POLY_ADDRESS"
	HeaderPolySignature  = "POLY_SIGNATURE"
	HeaderPolyTimestamp  = "POLY_TIMESTAMP"
	HeaderPolyNonce      = "POLY_NONCE"
	HeaderPolyAPIKey     = "POLY_API_KEY"
	HeaderPolyPassphrase = "POLY_PASSPHRASE"
)

type APICredentials struct {
	Key        string
	APIKey     string
	Secret     string
	Passphrase string
}

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

type L1AuthParams struct {
	Nonce     uint64
	Timestamp int64
}

type L2HeaderArgs struct {
	Method      string
	RequestPath string
	Body        []byte
	Timestamp   int64
}

func (c *Client) BuildL1Headers(ctx context.Context, params L1AuthParams) (http.Header, error) {
	if c.signer == nil {
		return nil, errors.New("L1 signer is not configured")
	}

	timestamp := params.Timestamp
	if timestamp == 0 {
		if c.useServerTime {
			var err error
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
	signature = strings.ReplaceAll(signature, "+", "-")
	signature = strings.ReplaceAll(signature, "/", "_")
	return signature, nil
}

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
