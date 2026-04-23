package clobclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientDefaultsShareTransport(t *testing.T) {
	t.Parallel()

	first, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	second, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if first.httpClient.Transport == nil {
		t.Fatal("expected default transport to be configured")
	}
	if first.httpClient.Transport != second.httpClient.Transport {
		t.Fatal("expected default clients to share the same transport for connection reuse")
	}
	if first.ChainID() != DefaultChainID {
		t.Fatalf("ChainID() = %d, want %d", first.ChainID(), DefaultChainID)
	}
	if first.BaseURL() != DefaultHost {
		t.Fatalf("BaseURL() = %q, want %q", first.BaseURL(), DefaultHost)
	}
	if first.CLOBVersion() != CLOBVersionV2 {
		t.Fatalf("CLOBVersion() = %q, want %q", first.CLOBVersion(), CLOBVersionV2)
	}
}

func TestNewClientFunderAddress(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{
		FunderAddress: "0x1111111111111111111111111111111111111111",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.FunderAddress() != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("FunderAddress() = %q", client.FunderAddress())
	}

	cloned, err := client.WithFunderAddress("0x2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatalf("WithFunderAddress() error = %v", err)
	}
	if cloned.FunderAddress() != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("cloned.FunderAddress() = %q", cloned.FunderAddress())
	}

	if _, err := NewClient(ClientConfig{FunderAddress: "not-an-address"}); err == nil {
		t.Fatal("expected invalid funder address error")
	}
	if _, err := client.WithFunderAddress("not-an-address"); err == nil {
		t.Fatal("expected invalid funder address error")
	}
}

func TestNewClientCLOBVersionSelectsDefaultHost(t *testing.T) {
	t.Parallel()

	v1Client, err := NewClient(ClientConfig{CLOBVersion: CLOBVersionV1})
	if err != nil {
		t.Fatalf("NewClient(V1) error = %v", err)
	}
	if v1Client.CLOBVersion() != CLOBVersionV1 {
		t.Fatalf("V1 CLOBVersion() = %q", v1Client.CLOBVersion())
	}
	if v1Client.BaseURL() != ProductionHost {
		t.Fatalf("V1 BaseURL() = %q, want %q", v1Client.BaseURL(), ProductionHost)
	}

	if _, err := NewClient(ClientConfig{CLOBVersion: "v3"}); err == nil {
		t.Fatal("expected unsupported CLOB version error")
	}
}

func TestNewClientWithProxySharesProxyTransport(t *testing.T) {
	t.Parallel()

	first, err := NewClient(ClientConfig{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	second, err := NewClient(ClientConfig{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	firstTransport, ok := first.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("first transport type = %T, want *http.Transport", first.httpClient.Transport)
	}
	secondTransport, ok := second.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("second transport type = %T, want *http.Transport", second.httpClient.Transport)
	}
	if firstTransport == sharedTransport {
		t.Fatal("expected proxy client to use a dedicated transport, not the direct shared transport")
	}
	if firstTransport != secondTransport {
		t.Fatal("expected identical proxy URLs to reuse the same transport")
	}

	req := httptest.NewRequest(http.MethodGet, "https://clob.polymarket.com/time", nil)
	proxyURL, err := firstTransport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy URL = %v, want http://127.0.0.1:7890", proxyURL)
	}
}

func TestNewClientWithProxyClonesCustomTransport(t *testing.T) {
	t.Parallel()

	originalTransport := http.DefaultTransport.(*http.Transport).Clone()
	originalClient := &http.Client{
		Timeout:   3 * time.Second,
		Transport: originalTransport,
	}

	client, err := NewClient(ClientConfig{
		HTTPClient: originalClient,
		ProxyURL:   "http://127.0.0.1:7890",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.httpClient == originalClient {
		t.Fatal("expected NewClient to clone the provided http.Client")
	}

	clonedTransport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.httpClient.Transport)
	}
	if clonedTransport == originalTransport {
		t.Fatal("expected proxy client to clone the provided transport")
	}

	req := httptest.NewRequest(http.MethodGet, "https://clob.polymarket.com/time", nil)
	proxyURL, err := clonedTransport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy URL = %v, want http://127.0.0.1:7890", proxyURL)
	}

	originalProxyURL, err := originalTransport.Proxy(req)
	if err != nil {
		t.Fatalf("original Proxy() error = %v", err)
	}
	if originalProxyURL != nil {
		t.Fatalf("original transport proxy = %v, want nil", originalProxyURL)
	}
}

func TestNewClientRejectsInvalidProxyURL(t *testing.T) {
	t.Parallel()

	_, err := NewClient(ClientConfig{ProxyURL: "127.0.0.1:7890"})
	if err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}

func TestGetServerTime(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time" {
			t.Fatalf("path = %q, want /time", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("1234567890"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:    server.URL,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetServerTime(context.Background())
	if err != nil {
		t.Fatalf("GetServerTime() error = %v", err)
	}
	if got != 1234567890 {
		t.Fatalf("GetServerTime() = %d, want 1234567890", got)
	}
}

func TestGetOrderBookBuildsRequestAndParsesResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book" {
			t.Fatalf("path = %q, want /book", r.URL.Path)
		}
		if got := r.URL.Query().Get("token_id"); got != "asset-123" {
			t.Fatalf("token_id = %q, want asset-123", got)
		}
		if got := r.Header.Get("User-Agent"); got != defaultUserAgent {
			t.Fatalf("User-Agent = %q, want %q", got, defaultUserAgent)
		}

		response := OrderBook{
			Market:         "condition-1",
			AssetID:        "asset-123",
			Timestamp:      "1234567890",
			Hash:           "hash-1",
			Bids:           []BookLevel{{Price: "0.45", Size: "100"}},
			Asks:           []BookLevel{{Price: "0.46", Size: "90"}},
			MinOrderSize:   "1",
			TickSize:       "0.01",
			NegRisk:        false,
			LastTradePrice: "0.45",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetOrderBook(context.Background(), "asset-123")
	if err != nil {
		t.Fatalf("GetOrderBook() error = %v", err)
	}

	if got.AssetID != "asset-123" {
		t.Fatalf("AssetID = %q, want asset-123", got.AssetID)
	}
	if got.Bids[0].Price != "0.45" {
		t.Fatalf("best bid = %q, want 0.45", got.Bids[0].Price)
	}
	if got.TickSize != "0.01" {
		t.Fatalf("TickSize = %q, want 0.01", got.TickSize)
	}
}

func TestPublicPriceEndpointsHandleMixedNumericShapes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/price":
			if got := r.URL.Query().Get("side"); got != "BUY" {
				t.Fatalf("side = %q, want BUY", got)
			}
			_, _ = w.Write([]byte(`{"price":0.45}`))
		case "/midpoint":
			_, _ = w.Write([]byte(`{"mid_price":"0.455"}`))
		case "/spread":
			_, _ = w.Write([]byte(`{"spread":"0.01"}`))
		case "/tick-size":
			_, _ = w.Write([]byte(`{"minimum_tick_size":0.01}`))
		case "/clob-markets/condition-1":
			_, _ = w.Write([]byte(`{
				"gst":"2023-11-07T05:31:56Z",
				"r":{},
				"t":[{"t":"token-1","o":"Yes"}],
				"mos":5,
				"mts":0.01,
				"mbf":0,
				"tbf":4,
				"rfqe":true,
				"itode":false,
				"ibce":true,
				"fd":{"r":0.02,"e":2,"to":true},
				"oas":123
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	price, err := client.GetPrice(context.Background(), "asset-123", "buy")
	if err != nil {
		t.Fatalf("GetPrice() error = %v", err)
	}
	if price.Price != "0.45" {
		t.Fatalf("price = %q, want 0.45", price.Price)
	}

	mid, err := client.GetMidpoint(context.Background(), "asset-123")
	if err != nil {
		t.Fatalf("GetMidpoint() error = %v", err)
	}
	if mid.Midpoint != "0.455" {
		t.Fatalf("midpoint = %q, want 0.455", mid.Midpoint)
	}

	spread, err := client.GetSpread(context.Background(), "asset-123")
	if err != nil {
		t.Fatalf("GetSpread() error = %v", err)
	}
	if spread.Spread != "0.01" {
		t.Fatalf("spread = %q, want 0.01", spread.Spread)
	}

	tickSize, err := client.GetTickSize(context.Background(), "asset-123")
	if err != nil {
		t.Fatalf("GetTickSize() error = %v", err)
	}
	if tickSize.MinimumTickSize != "0.01" {
		t.Fatalf("MinimumTickSize = %q, want 0.01", tickSize.MinimumTickSize)
	}

	marketInfo, err := client.GetCLOBMarketInfo(context.Background(), "condition-1")
	if err != nil {
		t.Fatalf("GetCLOBMarketInfo() error = %v", err)
	}
	if marketInfo.MinimumOrderSize != "5" {
		t.Fatalf("MinimumOrderSize = %q, want 5", marketInfo.MinimumOrderSize)
	}
	if marketInfo.FeeDetails.Rate != "0.02" {
		t.Fatalf("FeeDetails.Rate = %q, want 0.02", marketInfo.FeeDetails.Rate)
	}
	if len(marketInfo.Tokens) != 1 || marketInfo.Tokens[0].Outcome != "Yes" {
		t.Fatalf("unexpected tokens = %+v", marketInfo.Tokens)
	}
}

func TestAPIErrorIncludesMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.GetSpread(context.Background(), "asset-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusTooManyRequests)
	}
	if apiErr.Message != "rate limited" {
		t.Fatalf("Message = %q, want rate limited", apiErr.Message)
	}
}
