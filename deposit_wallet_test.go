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
			// 真实主网向量：该 owner 的 deposit wallet 已由工厂 0x0000…Cc07 实际部署，
			// 地址链上可查。这是唯一非自洽的 ground truth，公式回归必须先过这条。
			name:    "polygon mainnet real deployment",
			owner:   "0xDb4A281dF766581B4A5735E8BEa9148e13f9FCFb",
			chainID: 137,
			want:    "0xB2d2eCb0E15dEb9Cd232Ab83019aF9345E2eE377",
		},
		{
			// beacon proxy 公式（Polygon 主网当前工厂行为）派生的自洽 fixture。
			name:    "polygon mainnet hardhat-0",
			owner:   "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			chainID: 137,
			want:    "0xBc0fF067b7740Eff76C1ca93c875Ba6B890d6B50",
		},
		{
			name:    "polygon mainnet hardhat-1",
			owner:   "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
			chainID: 137,
			want:    "0x4Fe2CC4925607a473264FA89e7138075695A5F8e",
		},
		{
			// Amoy 工厂未升级，仍按普通 ERC-1967 clone 公式派生。
			name:    "amoy testnet hardhat-0",
			owner:   "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			chainID: 80002,
			want:    "0x12d77BcbDC3E67F5Da4Dc8e798623482D4a711db",
		},
		{
			name:    "lowercase owner is checksummed",
			owner:   "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
			chainID: 137,
			want:    "0xBc0fF067b7740Eff76C1ca93c875Ba6B890d6B50",
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
	if polygon.Beacon != PolygonDepositWalletBeacon {
		t.Fatalf("Polygon beacon = %q, want %q", polygon.Beacon, PolygonDepositWalletBeacon)
	}
	if polygon.Implementation != PolygonDepositWalletImplementation {
		t.Fatalf("Polygon implementation = %q, want %q", polygon.Implementation, PolygonDepositWalletImplementation)
	}

	amoy, err := DepositWalletConfigForChain(80002)
	if err != nil {
		t.Fatalf("DepositWalletConfigForChain(80002) error = %v", err)
	}
	if amoy.Beacon != "" {
		t.Fatalf("Amoy beacon = %q, want empty (legacy clone factory)", amoy.Beacon)
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

	// 用 Polygon 默认配置显式调用，结果应等价于 chainID=137（beacon proxy 模式）。
	got, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{
			Factory:        PolygonDepositWalletFactory,
			Beacon:         PolygonDepositWalletBeacon,
			Implementation: PolygonDepositWalletImplementation,
		},
	)
	if err != nil {
		t.Fatalf("DeriveDepositWalletAddressWithConfig() error = %v", err)
	}
	want := "0xBc0fF067b7740Eff76C1ca93c875Ba6B890d6B50"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Beacon 留空时退回普通 ERC-1967 clone 公式（Amoy 工厂当前行为）。
	got, err = DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{
			Factory:        AmoyDepositWalletFactory,
			Implementation: AmoyDepositWalletImplementation,
		},
	)
	if err != nil {
		t.Fatalf("DeriveDepositWalletAddressWithConfig() legacy mode error = %v", err)
	}
	if want := "0x12d77BcbDC3E67F5Da4Dc8e798623482D4a711db"; got != want {
		t.Fatalf("legacy mode got %q, want %q", got, want)
	}

	if _, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{Factory: "not-an-address", Implementation: PolygonDepositWalletImplementation},
	); err == nil {
		t.Fatal("expected error for invalid factory address")
	}

	if _, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{Factory: PolygonDepositWalletFactory, Beacon: "not-an-address"},
	); err == nil {
		t.Fatal("expected error for invalid beacon address")
	}

	if _, err := DeriveDepositWalletAddressWithConfig(
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		DepositWalletConfig{Factory: PolygonDepositWalletFactory, Implementation: "not-an-address"},
	); err == nil {
		t.Fatal("expected error for invalid implementation address in legacy mode")
	}
}
