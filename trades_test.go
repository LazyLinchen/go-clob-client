package clobclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSignerPrivateKey = "4c0883a69102937d6231471b5dbb6204fe5129617082794e6a6f2d14de3cd59c"

// newSignedTestClient 构造一个带 signer 与 L2 凭证、指向测试服务器的客户端。
func newSignedTestClient(t *testing.T, host string) *Client {
	t.Helper()

	signer, err := NewPrivateKeySigner(testSignerPrivateKey)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}

	client, err := NewClient(ClientConfig{
		Host:   host,
		Signer: signer,
		Credentials: &APICredentials{
			Key:        "test-key",
			Secret:     "c2VjcmV0",
			Passphrase: "test-pass",
		},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestGetTradesBuildsRequestAndParsesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointTrades {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointTrades)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if got := r.URL.Query().Get("market"); got != "condition-1" {
			t.Fatalf("market = %q, want condition-1", got)
		}
		if got := r.URL.Query().Get("asset_id"); got != "token-1" {
			t.Fatalf("asset_id = %q, want token-1", got)
		}
		if got := r.URL.Query().Get("next_cursor"); got != "MA==" {
			t.Fatalf("next_cursor = %q, want MA==", got)
		}
		if got := r.Header.Get(HeaderPolyAPIKey); got != "test-key" {
			t.Fatalf("missing L2 API key header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"limit": 100,
			"count": 1,
			"next_cursor": "LTE=",
			"data": [{
				"id": "trade-1",
				"taker_order_id": "taker-1",
				"market": "condition-1",
				"asset_id": "token-1",
				"side": "BUY",
				"size": "10",
				"price": "0.5",
				"status": "MATCHED",
				"trader_side": "TAKER",
				"bucket_index": 3,
				"created_at": 1700000000,
				"maker_orders": [{
					"order_id": "maker-1",
					"matched_amount": "4",
					"price": "0.5",
					"fee_rate_bps": "0",
					"asset_id": "token-1",
					"outcome": "Yes",
					"side": "SELL"
				}]
			}]
		}`))
	}))
	defer server.Close()

	client := newSignedTestClient(t, server.URL)

	got, err := client.GetTrades(context.Background(), TradesParams{
		Market:     "condition-1",
		AssetID:    "token-1",
		NextCursor: "MA==",
	})
	if err != nil {
		t.Fatalf("GetTrades() error = %v", err)
	}
	if got.Count != 1 || got.NextCursor != "LTE=" {
		t.Fatalf("unexpected pagination: count=%d next=%q", got.Count, got.NextCursor)
	}
	if len(got.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(got.Data))
	}
	trade := got.Data[0]
	if trade.ID != "trade-1" || trade.TraderSide != "TAKER" {
		t.Fatalf("unexpected trade = %+v", trade)
	}
	if trade.BucketIndex != "3" || trade.CreatedAt != "1700000000" {
		t.Fatalf("numeric tolerance failed: bucket=%q created=%q", trade.BucketIndex, trade.CreatedAt)
	}
	if len(trade.MakerOrders) != 1 || trade.MakerOrders[0].Side != "SELL" {
		t.Fatalf("unexpected maker orders = %+v", trade.MakerOrders)
	}
}

func TestGetTradesAcceptsBareArray(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"trade-1","price":"0.5"},{"id":"trade-2","price":"0.6"}]`))
	}))
	defer server.Close()

	client := newSignedTestClient(t, server.URL)

	got, err := client.GetTrades(context.Background(), TradesParams{})
	if err != nil {
		t.Fatalf("GetTrades() error = %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("len(data) = %d, want 2", len(got.Data))
	}
	if got.Data[1].ID != "trade-2" {
		t.Fatalf("data[1].ID = %q, want trade-2", got.Data[1].ID)
	}
}

func TestGetTradesRequiresCredentials(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.GetTrades(context.Background(), TradesParams{}); err == nil {
		t.Fatal("expected error when L2 credentials are not configured")
	}
}
