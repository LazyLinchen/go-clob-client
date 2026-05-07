package clobclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIntegrationGetBalanceAllowance(t *testing.T) {
	l2Client := newLiveAuthedClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := l2Client.GetBalanceAllowance(ctx, liveBalanceAllowanceParams(t))
	if err != nil {
		t.Fatalf("GetBalanceAllowance() error = %v", err)
	}
	if response == nil {
		t.Fatal("balance response is nil")
	}
	if strings.TrimSpace(response.Balance) == "" {
		t.Fatal("balance is empty")
	}
	if strings.TrimSpace(response.Allowance) == "" && len(response.Allowances) == 0 {
		t.Fatal("allowance is empty")
	}
	fmt.Printf("Balance: %s, Allowance: %s, Allowances: %+v\n", response.Balance, response.Allowance, response.Allowances)
}

func liveBalanceAllowanceParams(t *testing.T) BalanceAllowanceParams {
	t.Helper()

	rawAssetType := strings.ToUpper(strings.TrimSpace(os.Getenv("POLYMARKET_BALANCE_ASSET_TYPE")))
	if rawAssetType == "" {
		rawAssetType = string(AssetTypeCollateral)
	}

	params := BalanceAllowanceParams{
		AssetType: AssetType(rawAssetType),
		TokenID:   strings.TrimSpace(os.Getenv("POLYMARKET_BALANCE_TOKEN_ID")),
	}

	switch params.AssetType {
	case AssetTypeCollateral:
	case AssetTypeConditional:
		if params.TokenID == "" {
			t.Fatal("POLYMARKET_BALANCE_TOKEN_ID is required when POLYMARKET_BALANCE_ASSET_TYPE=CONDITIONAL")
		}
	default:
		t.Fatalf("unsupported POLYMARKET_BALANCE_ASSET_TYPE: %q", rawAssetType)
	}

	return params
}
