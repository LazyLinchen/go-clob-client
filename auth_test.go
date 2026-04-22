package clobclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func TestBuildClobAuthTypedData(t *testing.T) {
	t.Parallel()

	typedData := BuildClobAuthTypedData("0x1111111111111111111111111111111111111111", 137, 1712345678, 9)

	if typedData.PrimaryType != "ClobAuth" {
		t.Fatalf("PrimaryType = %q, want ClobAuth", typedData.PrimaryType)
	}
	if typedData.Domain.Name != clobAuthDomainName {
		t.Fatalf("Domain.Name = %q, want %q", typedData.Domain.Name, clobAuthDomainName)
	}
	if typedData.Domain.Version != clobAuthDomainVersion {
		t.Fatalf("Domain.Version = %q, want %q", typedData.Domain.Version, clobAuthDomainVersion)
	}
	if got := typedData.Message["timestamp"]; got != "1712345678" {
		t.Fatalf("timestamp = %#v, want 1712345678", got)
	}
	if got := typedData.Message["message"]; got != ClobAuthMessage {
		t.Fatalf("message = %#v, want %q", got, ClobAuthMessage)
	}
}

func TestPrivateKeySignerSignTypedDataRecoversAddress(t *testing.T) {
	t.Parallel()

	signer, err := NewPrivateKeySigner("4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c")
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	typedData := BuildClobAuthTypedData(signer.Address(), 137, 1712345678, 11)
	signatureHex, err := signer.SignTypedData(context.Background(), typedData)
	if err != nil {
		t.Fatalf("SignTypedData() error = %v", err)
	}

	signature, err := hexutil.Decode(signatureHex)
	if err != nil {
		t.Fatalf("Decode(signature) error = %v", err)
	}
	if len(signature) != 65 {
		t.Fatalf("signature length = %d, want 65", len(signature))
	}
	if signature[64] != 27 && signature[64] != 28 {
		t.Fatalf("signature V = %d, want 27 or 28", signature[64])
	}

	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatalf("TypedDataAndHash() error = %v", err)
	}

	recoverySig := append([]byte(nil), signature...)
	recoverySig[64] -= 27

	pubKey, err := crypto.SigToPub(digest, recoverySig)
	if err != nil {
		t.Fatalf("SigToPub() error = %v", err)
	}

	recovered := crypto.PubkeyToAddress(*pubKey).Hex()
	if !strings.EqualFold(recovered, signer.Address()) {
		t.Fatalf("recovered address = %q, want %q", recovered, signer.Address())
	}
}

func TestBuildL1HeadersUsesLocalTimeByDefault(t *testing.T) {
	t.Parallel()

	signer, err := NewPrivateKeySigner("4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c")
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:   server.URL,
		Signer: signer,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	before := time.Now().Unix()
	headers, err := client.BuildL1Headers(context.Background(), L1AuthParams{Nonce: 7})
	if err != nil {
		t.Fatalf("BuildL1Headers() error = %v", err)
	}
	after := time.Now().Unix()

	if got := headers.Get(HeaderPolyAddress); !strings.EqualFold(got, signer.Address()) {
		t.Fatalf("POLY_ADDRESS = %q, want %q", got, signer.Address())
	}
	gotTimestamp, err := strconv.ParseInt(headers.Get(HeaderPolyTimestamp), 10, 64)
	if err != nil {
		t.Fatalf("parse POLY_TIMESTAMP: %v", err)
	}
	if gotTimestamp < before || gotTimestamp > after {
		t.Fatalf("POLY_TIMESTAMP = %d, want between %d and %d", gotTimestamp, before, after)
	}
	if got := headers.Get(HeaderPolyNonce); got != "7" {
		t.Fatalf("POLY_NONCE = %q, want 7", got)
	}
	if got := headers.Get(HeaderPolySignature); got == "" {
		t.Fatal("POLY_SIGNATURE is empty")
	}
}

func TestBuildL1HeadersUsesServerTimeWhenConfigured(t *testing.T) {
	t.Parallel()

	signer, err := NewPrivateKeySigner("4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c")
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time" {
			t.Fatalf("path = %q, want /time", r.URL.Path)
		}
		_, _ = w.Write([]byte("1712345678"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:          server.URL,
		Signer:        signer,
		UseServerTime: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	headers, err := client.BuildL1Headers(context.Background(), L1AuthParams{Nonce: 7})
	if err != nil {
		t.Fatalf("BuildL1Headers() error = %v", err)
	}

	if got := headers.Get(HeaderPolyTimestamp); got != "1712345678" {
		t.Fatalf("POLY_TIMESTAMP = %q, want 1712345678", got)
	}
}

func TestCreateAndDeriveAPIKey(t *testing.T) {
	t.Parallel()

	signer, err := NewPrivateKeySigner("4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c")
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/time":
			t.Fatalf("unexpected request to /time")
		case "/auth/api-key":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %q, want POST", r.Method)
			}
			assertL1HeadersPresent(t, r)
			_, _ = w.Write([]byte(`{"apiKey":"key-1","secret":"secret-1","passphrase":"pass-1"}`))
		case "/auth/derive-api-key":
			if r.Method != http.MethodGet {
				t.Fatalf("method = %q, want GET", r.Method)
			}
			assertL1HeadersPresent(t, r)
			_, _ = w.Write([]byte(`{"api_key":"key-2","secret":"secret-2","passphrase":"pass-2"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:   server.URL,
		Signer: signer,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	createCreds, err := client.CreateAPIKey(context.Background(), L1AuthParams{Nonce: 1})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if createCreds.APIKey != "key-1" || createCreds.Secret != "secret-1" || createCreds.Passphrase != "pass-1" {
		t.Fatalf("unexpected create credentials = %+v", createCreds)
	}

	deriveCreds, err := client.DeriveAPIKey(context.Background(), L1AuthParams{Nonce: 1})
	if err != nil {
		t.Fatalf("DeriveAPIKey() error = %v", err)
	}
	if deriveCreds.APIKey != "key-2" || deriveCreds.Secret != "secret-2" || deriveCreds.Passphrase != "pass-2" {
		t.Fatalf("unexpected derive credentials = %+v", deriveCreds)
	}
}

func TestCreateOrDeriveAPIKeyFallsBackToDerive(t *testing.T) {
	t.Parallel()

	signer, err := NewPrivateKeySigner("4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c")
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	var createCalls int
	var deriveCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/time":
			t.Fatalf("unexpected request to /time")
		case "/auth/api-key":
			createCalls++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Could not create api key"}`))
		case "/auth/derive-api-key":
			deriveCalls++
			_, _ = w.Write([]byte(`{"apiKey":"key-derived","secret":"secret-derived","passphrase":"pass-derived"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:   server.URL,
		Signer: signer,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	creds, err := client.CreateOrDeriveAPIKey(context.Background(), L1AuthParams{Nonce: 9})
	if err != nil {
		t.Fatalf("CreateOrDeriveAPIKey() error = %v", err)
	}

	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
	if deriveCalls != 1 {
		t.Fatalf("deriveCalls = %d, want 1", deriveCalls)
	}
	if creds.APIKey != "key-derived" {
		t.Fatalf("APIKey = %q, want key-derived", creds.APIKey)
	}
}

func assertL1HeadersPresent(t *testing.T, r *http.Request) {
	t.Helper()

	for _, key := range []string{HeaderPolyAddress, HeaderPolySignature, HeaderPolyTimestamp, HeaderPolyNonce} {
		if got := r.Header.Get(key); got == "" {
			t.Fatalf("missing %s header", key)
		}
	}
}
