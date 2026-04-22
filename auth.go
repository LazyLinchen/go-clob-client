package clobclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	case wire.APIKey != "":
		c.APIKey = wire.APIKey
	case wire.APIKeyAlt != "":
		c.APIKey = wire.APIKeyAlt
	default:
		c.APIKey = wire.Key
	}
	c.Secret = wire.Secret
	c.Passphrase = wire.Passphrase
	return nil
}

type L1AuthParams struct {
	Nonce     uint64
	Timestamp int64
}

func (c *Client) BuildL1Headers(ctx context.Context, params L1AuthParams) (http.Header, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("L1 signer is not configured")
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
