package clobclient

import (
	"testing"
)

func TestDeriveDepositWalletAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		owner   string
		chainID int64
		want    string
		wantErr bool
	}{
		{
			name:    "polygon mainnet hardhat-0",
			owner:   "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			chainID: 137,
			want:    "0xdf8b9E8f9AB23f261F6e1B171B7454ae6E46Ba76",
		},
		{
			name:    "polygon mainnet hardhat-1",
			owner:   "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
			chainID: 137,
			want:    "0x5F4f45aBd6e86C60B3Df2a4aF103f85256A2Ce6d",
		},
		{
			name:    "amoy testnet hardhat-0",
			owner:   "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			chainID: 80002,
			want:    "0x12d77BcbDC3E67F5Da4Dc8e798623482D4a711db",
		},
		{
			name:    "lowercase owner is checksummed",
			owner:   "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
			chainID: 137,
			want:    "0xdf8b9E8f9AB23f261F6e1B171B7454ae6E46Ba76",
		},
		{
			name:    "invalid owner",
			owner:   "not-an-address",
			chainID: 137,
			wantErr: true,
		},
		{
			name:    "unsupported chain",
			owner:   "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			chainID: 1,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DeriveDepositWalletAddress(tc.owner, tc.chainID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DeriveDepositWalletAddress(%q, %d) = %q, want error", tc.owner, tc.chainID, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeriveDepositWalletAddress(%q, %d) error = %v", tc.owner, tc.chainID, err)
			}
			if got != tc.want {
				t.Fatalf("DeriveDepositWalletAddress(%q, %d) = %q, want %q", tc.owner, tc.chainID, got, tc.want)
			}
		})
	}
}

func TestDepositWalletConfigForChain(t *testing.T) {
	t.Parallel()

	polygon, err := DepositWalletConfigForChain(137)
	if err != nil {
		t.Fatalf("DepositWalletConfigForChain(137) error = %v", err)
	}
	if polygon.Factory != PolygonDepositWalletFactory {
		t.Fatalf("Polygon factory = %q, want %q", polygon.Factory, PolygonDepositWalletFactory)
	}
	if polygon.Implementation != PolygonDepositWalletImplementation {
		t.Fatalf("Polygon implementation = %q, want %q", polygon.Implementation, PolygonDepositWalletImplementation)
	}

	amoy, err := DepositWalletConfigForChain(80002)
	if err != nil {
		t.Fatalf("DepositWalletConfigForChain(80002) error = %v", err)
	}
	if amoy.Implementation != AmoyDepositWalletImplementation {
		t.Fatalf("Amoy implementation = %q, want %q", amoy.Implementation, AmoyDepositWalletImplementation)
	}

	if _, err := DepositWalletConfigForChain(1); err == nil {
		t.Fatal("expected error for unsupported chain")
	}
}

func TestDeriveDepositWalletAddressWithConfig(t *testing.T) {
	t.Parallel()

	// 用 Polygon 默认配置显式调用，结果应等价于 chainID=137。
	got, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{
			Factory:        PolygonDepositWalletFactory,
			Implementation: PolygonDepositWalletImplementation,
		},
	)
	if err != nil {
		t.Fatalf("DeriveDepositWalletAddressWithConfig() error = %v", err)
	}
	want := "0xdf8b9E8f9AB23f261F6e1B171B7454ae6E46Ba76"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	if _, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{Factory: "not-an-address", Implementation: PolygonDepositWalletImplementation},
	); err == nil {
		t.Fatal("expected error for invalid factory address")
	}

	if _, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{Factory: PolygonDepositWalletFactory, Implementation: "not-an-address"},
	); err == nil {
		t.Fatal("expected error for invalid implementation address")
	}
}
