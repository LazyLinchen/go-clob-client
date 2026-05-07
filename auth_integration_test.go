package clobclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var dotEnvLoadOnce sync.Once
var dotEnvLoadErr error

func TestIntegrationCreateOrDeriveAPIKey(t *testing.T) {
	client, params := newLiveAuthTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	creds, err := client.CreateOrDeriveAPIKey(ctx, params)
	if err != nil {
		t.Fatalf("CreateOrDeriveAPIKey() error = %v", err)
	}
	assertNonEmptyCredentials(t, creds)
	t.Log("CreateOrDeriveAPIKey returned non-empty credentials")

	derivedCreds, err := client.DeriveAPIKey(ctx, params)
	if err != nil {
		t.Fatalf("DeriveAPIKey() error = %v", err)
	}
	assertNonEmptyCredentials(t, derivedCreds)
}

func TestLiveAPICredentialsFromEnv(t *testing.T) {
	for _, key := range []string{
		"POLYMARKET_API_KEY",
		"POLY_API_KEY",
		"CLOB_API_KEY",
		"POLYMARKET_API_SECRET",
		"POLYMARKET_SECRET",
		"POLY_SECRET",
		"CLOB_SECRET",
		"POLYMARKET_API_PASSPHRASE",
		"POLYMARKET_PASSPHRASE",
		"POLY_PASSPHRASE",
		"CLOB_API_PASSPHRASE",
		"CLOB_PASS_PHRASE",
		"CLOB_PASSPHRASE",
	} {
		t.Setenv(key, "")
	}

	if creds := liveAPICredentialsFromEnv(t); creds != nil {
		t.Fatalf("liveAPICredentialsFromEnv() = %+v, want nil", creds)
	}

	t.Setenv("POLYMARKET_API_KEY", "key-1")
	t.Setenv("POLYMARKET_API_SECRET", "secret-1")
	t.Setenv("POLYMARKET_API_PASSPHRASE", "pass-1")

	creds := liveAPICredentialsFromEnv(t)
	if creds == nil {
		t.Fatal("liveAPICredentialsFromEnv() returned nil")
	}
	if creds.APIKey != "key-1" || creds.Key != "key-1" || creds.Secret != "secret-1" || creds.Passphrase != "pass-1" {
		t.Fatalf("liveAPICredentialsFromEnv() = %+v", creds)
	}
}

func TestLiveAuthProvisioningBlocked(t *testing.T) {
	err := errors.Join(
		&APIError{
			StatusCode: http.StatusForbidden,
			Method:     http.MethodPost,
			URL:        "https://clob.polymarket.com/auth/api-key",
			Message:    "403 Forbidden",
		},
		&APIError{
			StatusCode: http.StatusBadRequest,
			Method:     http.MethodGet,
			URL:        "https://clob.polymarket.com/auth/derive-api-key",
			Message:    "Could not derive api key!",
		},
	)
	if !liveAuthProvisioningBlocked(err) {
		t.Fatal("liveAuthProvisioningBlocked() = false, want true")
	}

	wrongHostErr := &APIError{
		StatusCode: http.StatusMethodNotAllowed,
		Method:     http.MethodPost,
		URL:        "https://clob-v2.polymarket.com/auth/api-key",
		Message:    "405 Method Not Allowed",
	}
	if liveAuthProvisioningBlocked(wrongHostErr) {
		t.Fatal("liveAuthProvisioningBlocked() = true for retired host error")
	}
}

func newLiveAuthTestClient(t *testing.T) (*Client, L1AuthParams) {
	t.Helper()

	return newLiveClient(t)
}

func newLiveClient(t *testing.T) (*Client, L1AuthParams) {
	t.Helper()

	if err := loadDotEnv(".env"); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	privateKey := strings.TrimSpace(os.Getenv("POLYMARKET_PRIVATE_KEY"))
	if privateKey == "" {
		t.Skip("set POLYMARKET_PRIVATE_KEY to run the live Polymarket integration tests")
	}

	signer, err := NewPrivateKeySigner(privateKey)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	chainID := int64(DefaultChainID)
	if raw := strings.TrimSpace(os.Getenv("POLYMARKET_CHAIN_ID")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			t.Fatalf("parse POLYMARKET_CHAIN_ID: %v", err)
		}
		chainID = value
	}

	nonce := uint64(0)
	if raw := strings.TrimSpace(os.Getenv("POLYMARKET_NONCE")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			t.Fatalf("parse POLYMARKET_NONCE: %v", err)
		}
		nonce = value
	}

	client, err := NewClient(ClientConfig{
		Host:          liveHost(t),
		ChainID:       chainID,
		Signer:        signer,
		Timeout:       20 * time.Second,
		ProxyURL:      liveProxyURL(t),
		SignatureType: liveSignatureType(t),
		FunderAddress: liveFunderAddress(t),
		BuilderCode:   liveBuilderCode(t),
		UseServerTime: liveUseServerTime(t),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return client, L1AuthParams{Nonce: nonce}
}

func newLiveAuthedClient(t *testing.T) *Client {
	t.Helper()

	client, params := newLiveClient(t)
	if creds := liveAPICredentialsFromEnv(t); creds != nil {
		assertNonEmptyCredentials(t, creds)
		return client.WithCredentials(creds)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	creds, err := client.CreateOrDeriveAPIKey(ctx, params)
	if err != nil {
		if liveAuthProvisioningBlocked(err) {
			t.Skipf(
				"CreateOrDeriveAPIKey is blocked by the live API; set POLYMARKET_API_KEY, POLYMARKET_API_SECRET, and POLYMARKET_API_PASSPHRASE or configure POLYMARKET_PROXY_URL: %v",
				err,
			)
		}
		t.Fatalf("CreateOrDeriveAPIKey() error = %v", err)
	}
	assertNonEmptyCredentials(t, creds)
	return client.WithCredentials(creds)
}

func liveAPICredentialsFromEnv(t *testing.T) *APICredentials {
	t.Helper()

	key := liveEnvFirst("POLYMARKET_API_KEY", "POLY_API_KEY", "CLOB_API_KEY")
	secret := liveEnvFirst("POLYMARKET_API_SECRET", "POLYMARKET_SECRET", "POLY_SECRET", "CLOB_SECRET")
	passphrase := liveEnvFirst(
		"POLYMARKET_API_PASSPHRASE",
		"POLYMARKET_PASSPHRASE",
		"POLY_PASSPHRASE",
		"CLOB_API_PASSPHRASE",
		"CLOB_PASS_PHRASE",
		"CLOB_PASSPHRASE",
	)
	if key == "" && secret == "" && passphrase == "" {
		return nil
	}
	if key == "" || secret == "" || passphrase == "" {
		t.Fatalf("set all L2 credential env vars: POLYMARKET_API_KEY, POLYMARKET_API_SECRET, and POLYMARKET_API_PASSPHRASE")
	}
	return &APICredentials{
		Key:        key,
		APIKey:     key,
		Secret:     secret,
		Passphrase: passphrase,
	}
}

func liveHost(t *testing.T) string {
	t.Helper()

	host := strings.TrimSpace(os.Getenv("POLYMARKET_HOST"))
	if host != "" {
		if strings.Contains(strings.ToLower(host), "clob-v2.polymarket.com") {
			t.Fatalf("POLYMARKET_HOST=%q points at the retired V2 preview host; unset it or use %s", host, DefaultHost)
		}
		return host
	}
	return DefaultHost
}

func liveAuthProvisioningBlocked(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusForbidden && strings.Contains(apiErr.URL, "/auth/api-key")
}

func liveProxyURL(t *testing.T) string {
	t.Helper()

	if proxyURL := strings.TrimSpace(os.Getenv("POLYMARKET_PROXY_URL")); proxyURL != "" {
		return proxyURL
	}
	return detectLocalHTTPProxyURL()
}

func liveUseServerTime(t *testing.T) bool {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("POLYMARKET_USE_SERVER_TIME"))
	if raw == "" {
		return true
	}

	return strings.EqualFold(raw, "1") || strings.EqualFold(raw, "true")
}

func liveSignatureType(t *testing.T) SignatureType {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("POLYMARKET_SIGNATURE_TYPE"))
	if raw == "" {
		return SignatureTypeEOA
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse POLYMARKET_SIGNATURE_TYPE: %v", err)
	}

	switch SignatureType(value) {
	case SignatureTypeEOA, SignatureTypePolyProxy, SignatureTypePolyGnosisSafe, SignatureTypePoly1271:
		return SignatureType(value)
	default:
		t.Fatalf("unsupported POLYMARKET_SIGNATURE_TYPE: %d", value)
		return SignatureTypeEOA
	}
}

func liveFunderAddress(t *testing.T) string {
	t.Helper()

	return strings.TrimSpace(os.Getenv("POLYMARKET_FUNDER_ADDRESS"))
}

func liveBuilderCode(t *testing.T) string {
	t.Helper()

	return strings.TrimSpace(os.Getenv("POLYMARKET_BUILDER_CODE"))
}

func liveDefaultUser(client *Client) string {
	if client == nil || client.Signer() == nil {
		return ""
	}
	return strings.TrimSpace(client.Signer().Address())
}

func liveEnvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func requiredEnvAnyForTest(t *testing.T, usage string, keys ...string) string {
	t.Helper()

	if value := liveEnvFirst(keys...); value != "" {
		return value
	}
	t.Skipf("set one of %s %s", strings.Join(keys, ", "), usage)
	return ""
}

func optionalIntEnvAny(t *testing.T, keys ...string) int {
	t.Helper()

	raw := liveEnvFirst(keys...)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", strings.Join(keys, ", "), err)
	}
	return value
}

func liveSideEnvAny(t *testing.T, keys ...string) Side {
	t.Helper()

	raw := strings.ToUpper(liveEnvFirst(keys...))
	if raw == "" {
		t.Skipf("set one of %s to BUY or SELL", strings.Join(keys, ", "))
	}
	side, err := normalizeSide(Side(raw))
	if err != nil {
		t.Fatalf("parse %s: %v", strings.Join(keys, ", "), err)
	}
	return side
}

func liveTickSizeEnvAny(t *testing.T, keys ...string) TickSize {
	t.Helper()

	value := liveEnvFirst(keys...)
	switch TickSize(value) {
	case TickSizeTenth, TickSizeCent, TickSizeMilli, TickSizeTenThousand:
		return TickSize(value)
	default:
		t.Fatalf("unsupported %s: %q", strings.Join(keys, ", "), value)
		return ""
	}
}

func liveOrderTypeEnvAny(t *testing.T, defaultValue OrderType, keys ...string) OrderType {
	t.Helper()

	raw := strings.ToUpper(liveEnvFirst(keys...))
	if raw == "" {
		return defaultValue
	}
	switch OrderType(raw) {
	case OrderTypeGTC, OrderTypeFOK, OrderTypeGTD, OrderTypeFAK:
		return OrderType(raw)
	default:
		t.Fatalf("unsupported %s: %q", strings.Join(keys, ", "), raw)
		return defaultValue
	}
}

func liveBoolEnvAny(t *testing.T, keys ...string) bool {
	t.Helper()

	raw := liveEnvFirst(keys...)
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", strings.Join(keys, ", "), err)
	}
	return value
}

func assertNonEmptyCredentials(t *testing.T, creds *APICredentials) {
	t.Helper()

	if creds == nil {
		t.Fatal("credentials are nil")
	}
	if strings.TrimSpace(creds.APIKey) == "" {
		t.Fatal("APIKey is empty")
	}
	if strings.TrimSpace(creds.Secret) == "" {
		t.Fatal("Secret is empty")
	}
	if strings.TrimSpace(creds.Passphrase) == "" {
		t.Fatal("Passphrase is empty")
	}
}

func loadDotEnv(path string) error {
	dotEnvLoadOnce.Do(func() {
		dotEnvLoadErr = loadDotEnvFile(path)
	})
	return dotEnvLoadErr
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNumber)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty key", path, lineNumber)
		}

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: set %s: %w", path, lineNumber, key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func detectLocalHTTPProxyURL() string {
	for _, address := range []string{
		"127.0.0.1:7897",
		"127.0.0.1:7890",
		"127.0.0.1:8080",
		"127.0.0.1:8888",
	} {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return "http://" + address
		}
	}
	return ""
}
