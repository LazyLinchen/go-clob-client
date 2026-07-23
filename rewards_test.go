package clobclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUserRewardPercentagesParsesMap(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointUserRewardPercentages {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointUserRewardPercentages)
		}
		if got := r.Header.Get(HeaderPolyAPIKey); got != "test-key" {
			t.Fatalf("missing L2 API key header, got %q", got)
		}
		if got := r.URL.Query().Get("signature_type"); got != "0" {
			t.Fatalf("signature_type = %q, want 0", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"condition-1":"0.25","condition-2":"0.1"}`))
	}))
	defer server.Close()

	client := newSignedTestClient(t, server.URL)

	got, err := client.GetUserRewardPercentages(context.Background())
	if err != nil {
		t.Fatalf("GetUserRewardPercentages() error = %v", err)
	}
	if got["condition-1"] != "0.25" {
		t.Fatalf("condition-1 = %q, want 0.25", got["condition-1"])
	}
}

func TestGetUserEarningsSetsDateAndParsesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointUserEarnings {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointUserEarnings)
		}
		if got := r.URL.Query().Get("date"); got != "2026-06-15" {
			t.Fatalf("date = %q, want 2026-06-15", got)
		}
		if got := r.URL.Query().Get("signature_type"); got != "0" {
			t.Fatalf("signature_type = %q, want 0", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"limit": 100,
			"count": 1,
			"next_cursor": "LTE=",
			"data": [{
				"condition_id": "condition-1",
				"market_id": 123,
				"event_id": "evt-1",
				"question": "Will it rain?",
				"rewards_min_size": "10",
				"market_competitiveness": "0.9",
				"tokens": [{"token_id":"token-1","outcome":"Yes","price":"0.5"}],
				"maker_address": "0xmaker",
				"earning_percentage": 12.5,
				"earnings": [{"asset_address":"0xusdc","earnings":1.75,"asset_rate":0.5}]
			}]
		}`))
	}))
	defer server.Close()

	client := newSignedTestClient(t, server.URL)

	got, err := client.GetUserEarnings(context.Background(), "2026-06-15", "")
	if err != nil {
		t.Fatalf("GetUserEarnings() error = %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(got.Data))
	}
	entry := got.Data[0]
	if entry.ConditionID != "condition-1" || entry.MarketID != "123" || entry.EventID != "evt-1" {
		t.Fatalf("unexpected earnings entry = %+v", entry)
	}
	if entry.MakerAddress != "0xmaker" || entry.EarningPercentage != "12.5" {
		t.Fatalf("unexpected maker earnings = %+v", entry)
	}
	if len(entry.Earnings) != 1 || entry.Earnings[0].Earnings != "1.75" {
		t.Fatalf("unexpected earnings breakdown = %+v", entry.Earnings)
	}
}

func TestGetCurrentRewardsSetsSponsoredAndParsesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointCurrentRewards {
			t.Fatalf("path = %q, want %q", r.URL.Path, endpointCurrentRewards)
		}
		if got := r.URL.Query().Get("sponsored"); got != "true" {
			t.Fatalf("sponsored = %q, want true", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"limit": 500,
			"count": 1,
			"next_cursor": "LTE=",
			"data": [{
				"condition_id": "condition-1",
				"rewards_max_spread": 99,
				"rewards_min_size": 10,
				"rewards_config": [{
					"id": 0,
					"asset_address": "0xabc",
					"start_date": "2024-03-01",
					"end_date": "2500-12-31",
					"rate_per_day": 2,
					"total_rewards": 92
				}],
				"sponsored_daily_rate": 0.5,
				"sponsors_count": 2,
				"native_daily_rate": 2.5,
				"total_daily_rate": 3
			}]
		}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.GetCurrentRewards(context.Background(), true, "")
	if err != nil {
		t.Fatalf("GetCurrentRewards() error = %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(got.Data))
	}
	market := got.Data[0]
	if market.ConditionID != "condition-1" || market.SponsorsCount != 2 {
		t.Fatalf("unexpected rewards market = %+v", market)
	}
	if market.RewardsMaxSpread != "99" || market.TotalDailyRate != "3" {
		t.Fatalf("numeric tolerance failed: maxSpread=%q totalDaily=%q", market.RewardsMaxSpread, market.TotalDailyRate)
	}
	if len(market.RewardsConfig) != 1 || market.RewardsConfig[0].RatePerDay != "2" {
		t.Fatalf("unexpected rewards config = %+v", market.RewardsConfig)
	}
}

func TestGetUserRewardPercentagesRequiresCredentials(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.GetUserRewardPercentages(context.Background()); err == nil {
		t.Fatal("expected error when L2 credentials are not configured")
	}
}
