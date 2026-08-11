package clobclient

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Polymarket Deposit Wallet 工厂与代理相关地址。工厂地址对齐
// builder-relayer-client/src/config/index.ts 中的 DepositWalletContracts；
// beacon 地址来自链上取证（工厂 implementation 内嵌的 immutable，
// 并与已部署 wallet 的 ERC-1967 beacon slot 实测值一致）。
const (
	// PolygonDepositWalletFactory 是 Polygon 主网（chain 137）的 deposit wallet 工厂地址。
	PolygonDepositWalletFactory = "0x00000000000Fb5C9ADea0298D729A0CB3823Cc07"
	// PolygonDepositWalletBeacon 是 Polygon 主网 deposit wallet beacon proxy 指向的 beacon 合约。
	// 当前主网工厂部署的是 Solady ERC-1967 beacon proxy，该地址参与 CREATE2 init code hash。
	PolygonDepositWalletBeacon = "0x7A18EDfe055488A3128f01F563e5B479D92ffc3a"
	// PolygonDepositWalletImplementation 是 Polygon 主网 deposit wallet 的逻辑合约地址，
	// 由 beacon 动态解析，不参与地址派生。
	PolygonDepositWalletImplementation = "0x58CA52ebe0DadfdF531Cde7062e76746de4Db1eB"

	// AmoyDepositWalletFactory 是 Amoy 测试网（chain 80002）的 deposit wallet 工厂地址。
	AmoyDepositWalletFactory = "0x00000000000Fb5C9ADea0298D729A0CB3823Cc07"
	// AmoyDepositWalletImplementation 是 Amoy 测试网的 deposit wallet implementation 地址。
	// Amoy 工厂尚未升级到 beacon 部署，仍以普通 ERC-1967 clone 方式部署，该地址参与派生。
	AmoyDepositWalletImplementation = "0x50a88fE9a441cB4c9c2aD6A2207CE2795C7D7Fbd"
)

const (
	// depositWalletDomainName 是 DepositWallet 合约 EIP-712 域名，用于 POLY_1271 包裹签名的内层。
	depositWalletDomainName = "DepositWallet"
	// depositWalletDomainVersion 是 DepositWallet 合约 EIP-712 域版本号。
	depositWalletDomainVersion = "1"
)

// DepositWalletConfig 描述某条链上 deposit wallet 的部署参数。
//
// 两种派生模式（由 Beacon 是否为空决定）：
//   - Beacon 非空：工厂以 Solady ERC-1967 beacon proxy 部署（当前 Polygon 主网），
//     init code 内嵌 beacon 地址，Implementation 不参与派生。
//   - Beacon 为空：工厂以普通 Solady ERC-1967 clone 部署（当前 Amoy），
//     init code 内嵌 Implementation 地址。
type DepositWalletConfig struct {
	// Factory 是 DepositWalletFactory 合约地址，即 CREATE2 deployer。
	Factory string
	// Beacon 是 ERC-1967 beacon 合约地址；非空时启用 beacon proxy 派生模式。
	Beacon string
	// Implementation 是 DepositWallet implementation 合约地址；
	// 仅在 Beacon 为空（普通 clone 模式）时参与派生。
	Implementation string
}

// DepositWalletConfigForChain 返回内置的 deposit wallet 部署配置；仅 Polygon 主网与 Amoy 测试网已知。
func DepositWalletConfigForChain(chainID int64) (DepositWalletConfig, error) {
	switch chainID {
	case 137:
		return DepositWalletConfig{
			Factory:        PolygonDepositWalletFactory,
			Beacon:         PolygonDepositWalletBeacon,
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
// 算法对齐链上工厂的真实部署方式（见 DepositWalletConfig 的两种模式），
// salt = keccak256(abi.encode(factory, bytes32(uint160(owner))))，两种模式一致。
//
// 注意：@polymarket/builder-relayer-client@0.0.9 的 deriveDepositWallet 只实现了
// 普通 ERC-1967 clone 公式，对已升级为 beacon 部署的 Polygon 主网工厂会算出
// 与链上实际部署不符的地址，勿以其为 Polygon 主网的参考。
func DeriveDepositWalletAddress(owner string, chainID int64) (string, error) {
	config, err := DepositWalletConfigForChain(chainID)
	if err != nil {
		return "", err
	}
	return DeriveDepositWalletAddressWithConfig(owner, config)
}

// DeriveDepositWalletAddressWithConfig 允许显式传入部署配置完成派生，
// 便于在测试网新部署或本地分叉环境覆盖默认配置。
func DeriveDepositWalletAddressWithConfig(owner string, config DepositWalletConfig) (string, error) {
	if !common.IsHexAddress(owner) {
		return "", fmt.Errorf("invalid owner address %q", owner)
	}
	if !common.IsHexAddress(config.Factory) {
		return "", fmt.Errorf("invalid factory address %q", config.Factory)
	}
	isBeaconMode := config.Beacon != ""
	if isBeaconMode && !common.IsHexAddress(config.Beacon) {
		return "", fmt.Errorf("invalid beacon address %q", config.Beacon)
	}
	if !isBeaconMode && !common.IsHexAddress(config.Implementation) {
		return "", fmt.Errorf("invalid implementation address %q", config.Implementation)
	}

	ownerAddr := common.HexToAddress(owner)
	factoryAddr := common.HexToAddress(config.Factory)

	// abi.encode(address factory, bytes32 walletId)
	// walletId = bytes32(uint160(owner))，即把 20 字节 owner 左侧 0 填充到 32 字节。
	args := make([]byte, 64)
	copy(args[12:32], factoryAddr.Bytes())
	copy(args[44:64], ownerAddr.Bytes())

	salt := crypto.Keccak256(args)
	var bytecodeHash []byte
	if isBeaconMode {
		bytecodeHash = initCodeHashERC1967BeaconProxy(common.HexToAddress(config.Beacon), args)
	} else {
		bytecodeHash = initCodeHashERC1967(common.HexToAddress(config.Implementation), args)
	}

	// CREATE2 标准公式：keccak256(0xff || from || salt || bytecodeHash)[12:]
	buf := make([]byte, 1+20+32+32)
	buf[0] = 0xff
	copy(buf[1:21], factoryAddr.Bytes())
	copy(buf[21:53], salt)
	copy(buf[53:85], bytecodeHash)

	hash := crypto.Keccak256(buf)
	return common.BytesToAddress(hash[12:]).Hex(), nil
}

// Solady v0.1.26 LibClone.initCodeHashERC1967BeaconProxy(beacon, args) 使用的固定字节，参考
// https://github.com/Vectorized/solady/blob/v0.1.26/src/utils/LibClone.sol。
// runtime 中内嵌 ERC-1967 beacon slot
// 0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50，
// 跨 const2 尾部与 const3 头部拼接而成。
var (
	beaconProxyConst1     = mustDecodeHexLiteral("60195155f3363d3d373d3d363d602036600436635c60da")
	beaconProxyConst2     = mustDecodeHexLiteral("1b60e01b36527fa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6c")
	beaconProxyConst3     = mustDecodeHexLiteral("b3582b35133d50545afa5036515af43d6000803e604d573d6000fd5b3d6000f3")
	beaconProxyPrefixBase = mustParseBigHexLiteral("6100523d8160233d3973")
)

// initCodeHashERC1967BeaconProxy 复刻 Solady LibClone.initCodeHashERC1967BeaconProxy(beacon, args)。
// 布局：prefix(10) || beacon(20) || const1(23) || const2(32) || const3(32) || args，
// 共 0x75 + len(args) 字节。prefix 中 PUSH2 操作数 0x0052 需加上 args 长度得到最终 runtime 大小，
// 对应 Solady 汇编里的 add(0x6100523d8160233d3973, shl(56, n))。
func initCodeHashERC1967BeaconProxy(beacon common.Address, args []byte) []byte {
	combined := new(big.Int).Lsh(big.NewInt(int64(len(args))), 56)
	combined.Add(combined, beaconProxyPrefixBase)

	prefix := make([]byte, 10)
	combined.FillBytes(prefix)

	buf := make([]byte, 0, 10+20+23+32+32+len(args))
	buf = append(buf, prefix...)
	buf = append(buf, beacon.Bytes()...)
	buf = append(buf, beaconProxyConst1...)
	buf = append(buf, beaconProxyConst2...)
	buf = append(buf, beaconProxyConst3...)
	buf = append(buf, args...)

	return crypto.Keccak256(buf)
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
