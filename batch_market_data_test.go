package clobclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOrderBooksPostsTokenIDs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointBooks {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointBooks)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}

		var params []BookParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(params) != 2 || params[0].TokenID != "token-1" || params[1].TokenID != "token-2" {
			t.Fatalf("unexpected params = %+v", params)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"market":"c-1","asset_id":"token-1","bids":[{"price":"0.4","size":"5"}]},
			{"market":"c-1","asset_id":"token-2","asks":[{"price":"0.6","size":"7"}]}
		]`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	books, err := client.GetOrderBooks(context.Background(), []string{"token-1", "token-1", "token-2"})
	if err != nil {
		t.Fatalf("GetOrderBooks() error = %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("len(books) = %d, want 2", len(books))
	}
	if books[0].AssetID != "token-1" || books[0].Bids[0].Price != "0.4" {
		t.Fatalf("unexpected book[0] = %+v", books[0])
	}
}

func TestGetPricesRequiresSideAndParsesMap(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointPrices {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointPrices)
		}

		var params []BookParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(params) != 1 || params[0].Side != SideBuy {
			t.Fatalf("unexpected params = %+v", params)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token-1":{"BUY":"0.45","SELL":"0.46"}}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	prices, err := client.GetPrices(context.Background(), []BookParams{{TokenID: "token-1", Side: "buy"}})
	if err != nil {
		t.Fatalf("GetPrices() error = %v", err)
	}
	if prices["token-1"]["BUY"] != "0.45" {
		t.Fatalf("BUY price = %q, want 0.45", prices["token-1"]["BUY"])
	}

	if _, err := client.GetPrices(context.Background(), []BookParams{{TokenID: "token-1"}}); err == nil {
		t.Fatal("expected error when side is missing for GetPrices")
	}
}

func TestGetMidpointsAndSpreadsParseMaps(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case endpointMidpoints:
			_, _ = w.Write([]byte(`{"token-1":"0.455","token-2":"0.5"}`))
		case endpointSpreads:
			_, _ = w.Write([]byte(`{"token-1":"0.01"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	midpoints, err := client.GetMidpoints(context.Background(), []string{"token-1", "token-2"})
	if err != nil {
		t.Fatalf("GetMidpoints() error = %v", err)
	}
	if midpoints["token-2"] != "0.5" {
		t.Fatalf("midpoint token-2 = %q, want 0.5", midpoints["token-2"])
	}

	spreads, err := client.GetSpreads(context.Background(), []string{"token-1"})
	if err != nil {
		t.Fatalf("GetSpreads() error = %v", err)
	}
	if spreads["token-1"] != "0.01" {
		t.Fatalf("spread token-1 = %q, want 0.01", spreads["token-1"])
	}
}

func TestBatchMarketDataRejectsEmptyInput(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.GetOrderBooks(context.Background(), nil); err == nil {
		t.Fatal("expected error for empty token list")
	}
	if _, err := client.GetMidpoints(context.Background(), []string{"  "}); err == nil {
		t.Fatal("expected error for blank token list")
	}
	if _, err := client.GetPrices(context.Background(), nil); err == nil {
		t.Fatal("expected error for empty book params")
	}
}
