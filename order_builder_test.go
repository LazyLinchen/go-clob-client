package clobclient

import (
	"context"
	"testing"
)

func newOrderBuilderTestClient(t *testing.T) *Client {
	t.Helper()

	signer, err := NewPrivateKeySigner(testSignerPrivateKey)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}
	client, err := NewClient(ClientConfig{Signer: signer})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestCreateOrderSupportsQuarterCentTick(t *testing.T) {
	t.Parallel()

	client := newOrderBuilderTestClient(t)
	order, err := client.CreateOrder(context.Background(), CreateOrderParams{
		TokenID:     "123",
		Price:       "0.5013",
		Size:        "10",
		Side:        SideBuy,
		TickSize:    TickSize("0.0025"),
		Salt:        "1",
		TimestampMS: 1,
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if order.MakerAmount != "5025000" || order.TakerAmount != "10000000" {
		t.Fatalf("amounts = maker %s taker %s, want 5025000/10000000", order.MakerAmount, order.TakerAmount)
	}
}

func TestCreateMarketOrderCapsDerivedAmountPrecision(t *testing.T) {
	t.Parallel()

	client := newOrderBuilderTestClient(t)
	order, err := client.CreateMarketOrder(context.Background(), CreateMarketOrderParams{
		TokenID:     "123",
		Amount:      "9.98",
		Price:       "0.985",
		Side:        SideBuy,
		TickSize:    TickSizeMilli,
		OrderType:   OrderTypeFOK,
		Salt:        "1",
		TimestampMS: 1,
	})
	if err != nil {
		t.Fatalf("CreateMarketOrder() error = %v", err)
	}
	if order.MakerAmount != "9980000" {
		t.Fatalf("MakerAmount = %s, want 9980000", order.MakerAmount)
	}
	if order.TakerAmount != "10131900" {
		t.Fatalf("TakerAmount = %s, want 10131900 (four decimal places)", order.TakerAmount)
	}
}
