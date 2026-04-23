package clobclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const liveAuthTestEnv = "POLYMARKET_RUN_LIVE_AUTH_TEST"
const liveAuthTestValue = "run-live"

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

	derivedCreds, err := client.DeriveAPIKey(ctx, params)
	if err != nil {
		t.Fatalf("DeriveAPIKey() error = %v", err)
	}
	assertNonEmptyCredentials(t, derivedCreds)
}

func newLiveAuthTestClient(t *testing.T) (*Client, L1AuthParams) {
	t.Helper()

	if err := loadDotEnv(".env"); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	if strings.TrimSpace(os.Getenv(liveAuthTestEnv)) != liveAuthTestValue {
		t.Skip("set POLYMARKET_RUN_LIVE_AUTH_TEST=run-live to run the live Polymarket L1 auth integration test")
	}

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

	host := strings.TrimSpace(os.Getenv("POLYMARKET_HOST"))
	if host == "" {
		host = DefaultHost
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
		Host:          host,
		ChainID:       chainID,
		Signer:        signer,
		Timeout:       20 * time.Second,
		ProxyURL:      strings.TrimSpace(os.Getenv("POLYMARKET_PROXY_URL")),
		SignatureType: liveSignatureType(t),
		FunderAddress: strings.TrimSpace(os.Getenv("POLYMARKET_FUNDER_ADDRESS")),
		CLOBVersion:   liveCLOBVersion(t),
		UseServerTime: liveUseServerTime(),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return client, L1AuthParams{Nonce: nonce}
}

func liveCLOBVersion(t *testing.T) CLOBVersion {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_VERSION"))
	if raw == "" {
		return CLOBVersionV2
	}
	version, err := normalizeCLOBVersion(CLOBVersion(raw))
	if err != nil {
		t.Fatalf("parse POLYMARKET_CLOB_VERSION: %v", err)
	}
	return version
}

func liveUseServerTime() bool {
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
