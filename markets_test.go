package clobclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetMarketsParsesCursorPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointMarkets {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointMarkets)
		}
		if got := r.URL.Query().Get("next_cursor"); got != "MA==" {
			t.Fatalf("next_cursor = %q, want MA==", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"limit": 100,
			"count": 1,
			"next_cursor": "LTE=",
			"data": [{
				"condition_id": "condition-1",
				"question": "Will it rain?",
				"market_slug": "will-it-rain",
				"minimum_tick_size": "0.01",
				"maker_base_fee": 0,
				"taker_base_fee": 4,
				"neg_risk": false,
				"tokens": [{"token_id":"token-1","outcome":"Yes","price":"0.5","winner":false}],
				"rewards": {"rates":[{"asset_address":"0xabc","rewards_daily_rate":"3"}],"min_size":"10","max_spread":"0.03"}
			}]
		}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetMarkets(context.Background(), "MA==")
	if err != nil {
		t.Fatalf("GetMarkets() error = %v", err)
	}
	if got.NextCursor != "LTE=" || len(got.Data) != 1 {
		t.Fatalf("unexpected response: next=%q count=%d", got.NextCursor, len(got.Data))
	}
	market := got.Data[0]
	if market.ConditionID != "condition-1" || market.TakerBaseFee != 4 {
		t.Fatalf("unexpected market = %+v", market)
	}
	if len(market.Tokens) != 1 || market.Tokens[0].Outcome != "Yes" {
		t.Fatalf("unexpected tokens = %+v", market.Tokens)
	}
	if market.Rewards.MinSize != "10" || len(market.Rewards.Rates) != 1 {
		t.Fatalf("unexpected rewards = %+v", market.Rewards)
	}
}

func TestGetSimplifiedMarketsParsesResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointSimplifiedMarkets {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointSimplifiedMarkets)
		}
		if r.URL.RawQuery != "" {
			t.Fatalf("expected no query when cursor empty, got %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"limit": 100,
			"count": 1,
			"next_cursor": "LTE=",
			"data": [{
				"condition_id": "condition-1",
				"active": true,
				"closed": false,
				"accepting_orders": true,
				"tokens": [{"token_id":"token-1","outcome":"Yes","price":"0.5"}],
				"rewards": {"min_size":"10","max_spread":"0.03"}
			}]
		}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetSimplifiedMarkets(context.Background(), "")
	if err != nil {
		t.Fatalf("GetSimplifiedMarkets() error = %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(got.Data))
	}
	market := got.Data[0]
	if !market.Active || !market.AcceptingOrders || market.ConditionID != "condition-1" {
		t.Fatalf("unexpected simplified market = %+v", market)
	}
}

func TestGetMarketByConditionID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointMarket+"condition-1" {
			t.Fatalf("path = %q, want %scondition-1", r.URL.Path, endpointMarket)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"condition_id":"condition-1","question":"Will it rain?","neg_risk":true}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetMarket(context.Background(), "condition-1")
	if err != nil {
		t.Fatalf("GetMarket() error = %v", err)
	}
	if got.ConditionID != "condition-1" || !got.NegRisk {
		t.Fatalf("unexpected market = %+v", got)
	}

	if _, err := client.GetMarket(context.Background(), "  "); err == nil {
		t.Fatal("expected error for blank conditionID")
	}
}
