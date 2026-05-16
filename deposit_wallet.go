package clobclient

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Polymarket Deposit Wallet 工厂与 implementation 地址，常量直接对齐
// builder-relayer-client/src/config/index.ts 中的 DepositWalletContracts。
const (
	// PolygonDepositWalletFactory 是 Polygon 主网（chain 137）的 deposit wallet 工厂地址。
	PolygonDepositWalletFactory = "0x00000000000Fb5C9ADea0298D729A0CB3823Cc07"
	// PolygonDepositWalletImplementation 是 Polygon 主网（chain 137）的 deposit wallet implementation 地址。
	PolygonDepositWalletImplementation = "0x58CA52ebe0DadfdF531Cde7062e76746de4Db1eB"

	// AmoyDepositWalletFactory 是 Amoy 测试网（chain 80002）的 deposit wallet 工厂地址。
	AmoyDepositWalletFactory = "0x00000000000Fb5C9ADea0298D729A0CB3823Cc07"
	// AmoyDepositWalletImplementation 是 Amoy 测试网（chain 80002）的 deposit wallet implementation 地址。
	AmoyDepositWalletImplementation = "0x50a88fE9a441cB4c9c2aD6A2207CE2795C7D7Fbd"
)

const (
	// depositWalletDomainName 是 DepositWallet 合约 EIP-712 域名，用于 POLY_1271 包裹签名的内层。
	depositWalletDomainName = "DepositWallet"
	// depositWalletDomainVersion 是 DepositWallet 合约 EIP-712 域版本号。
	depositWalletDomainVersion = "1"
)

// DepositWalletConfig 描述某条链上 deposit wallet 的工厂与 implementation 地址。
type DepositWalletConfig struct {
	// Factory 是 DepositWalletFactory 合约地址。
	Factory string
	// Implementation 是 DepositWallet implementation 合约地址。
	Implementation string
}

// DepositWalletConfigForChain 返回内置的 deposit wallet 部署配置；仅 Polygon 主网与 Amoy 测试网已知。
func DepositWalletConfigForChain(chainID int64) (DepositWalletConfig, error) {
	switch chainID {
	case 137:
		return DepositWalletConfig{
			Factory:        PolygonDepositWalletFactory,
			Implementation: PolygonDepositWalletImplementation,
		}, nil
	case 80002:
		return DepositWalletConfig{
			Factory:        AmoyDepositWalletFactory,
			Implementation: AmoyDepositWalletImplementation,
		}, nil
	default:
		return DepositWalletConfig{}, fmt.Errorf("deposit wallet not supported on chain %d", chainID)
	}
}

// DeriveDepositWalletAddress 返回 owner EOA 对应的 deposit wallet 确定性地址。
// 算法与 @polymarket/builder-relayer-client/src/builder/derive.ts 中的 deriveDepositWallet 一致。
func DeriveDepositWalletAddress(owner string, chainID int64) (string, error) {
	config, err := DepositWalletConfigForChain(chainID)
	if err != nil {
		return "", err
	}
	return DeriveDepositWalletAddressWithConfig(owner, config)
}

// DeriveDepositWalletAddressWithConfig 允许显式传入工厂 + implementation 地址完成派生，
// 便于在测试网新部署或本地分叉环境覆盖默认配置。
func DeriveDepositWalletAddressWithConfig(owner string, config DepositWalletConfig) (string, error) {
	if !common.IsHexAddress(owner) {
		return "", fmt.Errorf("invalid owner address %q", owner)
	}
	if !common.IsHexAddress(config.Factory) {
		return "", fmt.Errorf("invalid factory address %q", config.Factory)
	}
	if !common.IsHexAddress(config.Implementation) {
		return "", fmt.Errorf("invalid implementation address %q", config.Implementation)
	}

	ownerAddr := common.HexToAddress(owner)
	factoryAddr := common.HexToAddress(config.Factory)
	implAddr := common.HexToAddress(config.Implementation)

	// abi.encode(address factory, bytes32 walletId)
	// walletId = bytes32(uint160(owner))，即把 20 字节 owner 左侧 0 填充到 32 字节。
	args := make([]byte, 64)
	copy(args[12:32], factoryAddr.Bytes())
	copy(args[44:64], ownerAddr.Bytes())

	salt := crypto.Keccak256(args)
	bytecodeHash := initCodeHashERC1967(implAddr, args)

	// CREATE2 标准公式：keccak256(0xff || from || salt || bytecodeHash)[12:]
	buf := make([]byte, 1+20+32+32)
	buf[0] = 0xff
	copy(buf[1:21], factoryAddr.Bytes())
	copy(buf[21:53], salt)
	copy(buf[53:85], bytecodeHash)

	hash := crypto.Keccak256(buf)
	return common.BytesToAddress(hash[12:]).Hex(), nil
}

// Solady v0.1.26 LibClone.initCodeHashERC1967 使用的固定字节。
// 这两段常量分别是 ERC-1967 minimal proxy 的 runtime + 收尾字节，参考
// https://github.com/Vectorized/solady/blob/v0.1.26/src/utils/LibClone.sol。
var (
	erc1967Const1     = mustDecodeHexLiteral("cc3735a920a3ca505d382bbc545af43d6000803e6038573d6000fd5b3d6000f3")
	erc1967Const2     = mustDecodeHexLiteral("5155f3363d3d373d3d363d7f360894a13ba1a3210667c828492db98dca3e2076")
	erc1967PrefixBase = mustParseBigHexLiteral("61003d3d8160233d3973")
)

// initCodeHashERC1967 复刻 Solady LibClone.initCodeHashERC1967(implementation, args)。
// 布局：prefix(10) || implementation(20) || 0x6009 || const2(32) || const1(32) || args。
// prefix 的高位 byte[2] 编码的是 runtime 大小的低字节，加上 args 长度后形成最终的 PUSH2 操作数。
func initCodeHashERC1967(implementation common.Address, args []byte) []byte {
	// runtimeSizeAdj = ERC1967_PREFIX + (len(args) << 56)
	combined := new(big.Int).Lsh(big.NewInt(int64(len(args))), 56)
	combined.Add(combined, erc1967PrefixBase)

	prefix := make([]byte, 10)
	combined.FillBytes(prefix)

	buf := make([]byte, 0, 10+20+2+32+32+len(args))
	buf = append(buf, prefix...)
	buf = append(buf, implementation.Bytes()...)
	buf = append(buf, 0x60, 0x09)
	buf = append(buf, erc1967Const2...)
	buf = append(buf, erc1967Const1...)
	buf = append(buf, args...)

	return crypto.Keccak256(buf)
}

func mustDecodeHexLiteral(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("deposit_wallet: invalid hex literal %q: %v", s, err))
	}
	return b
}

func mustParseBigHexLiteral(s string) *big.Int {
	value, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic(fmt.Sprintf("deposit_wallet: invalid big.Int hex literal %q", s))
	}
	return value
}
