package clobclient

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// SignatureTypeUnknown 是识别失败时返回的值，不能用于下单。
const SignatureTypeUnknown SignatureType = -1

// ErrUnknownWallet 表示资金钱包不匹配该 signer 的已支持钱包派生地址。
// 可以通过 errors.Is 检查；不会将未知钱包猜测为 EOA 或 session signer。
var ErrUnknownWallet = errors.New("wallet does not match a supported wallet derived from signer")

// Polygon deployment constants and proxy init code follow Polymarket/ts-sdk
// 1462212874971ce60d1ab56a6df13eb6eda89daa, environments.ts and wallet.ts.
const (
	polygonProxyFactory         = "0xaB45c5A4B0c941a2F231C04C3f49182e1A254052"
	polygonProxyImplementation  = "0x44e999d5c2F66Ef0861317f9A4805AC2e90aEB4f"
	polygonSafeFactory          = "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b"
	polygonSafeInitCodeHash     = "0x2bce2127ff07fb632d16c8347c4ebf501f4841168bed00d9e6ef715ddb6fcecf"
	proxyWalletBytecodeTemplate = "3d3d606380380380913d393d73%s5af4602a57600080fd5b602d8060366000396000f3363d3d373d3d3d363d73%s5af43d82803e903d91602b57fd5bf352e831dd00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000"
)

// DetectSignatureType 通过 signer EOA 与实际资金钱包地址识别签名类型。
// 纯本地地址匹配，不需要私钥、API 凭证或 RPC，不证明钱包已部署或已获授权。
// Polygon (137) 支持 EOA、Proxy、Safe 和当前 beacon Deposit Wallet；
// Amoy (80002) 支持 EOA 和本库内置配置的 Deposit Wallet。其他链返回错误。
// walletAddress 必须由调用方提供，不能仅凭 signer 判断实际使用的钱包。
func DetectSignatureType(signerAddress, walletAddress string, chainID int64) (SignatureType, error) {
	signer, err := detectionAddress(signerAddress, "signer")
	if err != nil {
		return SignatureTypeUnknown, err
	}
	wallet, err := detectionAddress(walletAddress, "wallet")
	if err != nil {
		return SignatureTypeUnknown, err
	}
	config, err := DepositWalletConfigForChain(chainID)
	if err != nil {
		return SignatureTypeUnknown, err
	}
	if wallet == signer {
		return SignatureTypeEOA, nil
	}
	deposit, err := DeriveDepositWalletAddressWithConfig(signer.Hex(), config)
	if err != nil {
		return SignatureTypeUnknown, err
	}
	if wallet == common.HexToAddress(deposit) {
		return SignatureTypePoly1271, nil
	}
	if chainID == 137 {
		// Safe salt = keccak256(abi.encode(address)); Proxy uses packed address.
		safeSalt := crypto.Keccak256Hash(common.LeftPadBytes(signer.Bytes(), 32))
		safe := crypto.CreateAddress2(common.HexToAddress(polygonSafeFactory), safeSalt, common.FromHex(polygonSafeInitCodeHash))
		if wallet == safe {
			return SignatureTypePolyGnosisSafe, nil
		}
		code, err := hex.DecodeString(fmt.Sprintf(proxyWalletBytecodeTemplate,
			strings.ToLower(polygonProxyFactory[2:]), strings.ToLower(polygonProxyImplementation[2:])))
		if err != nil {
			return SignatureTypeUnknown, fmt.Errorf("decode proxy init code: %w", err)
		}
		proxySalt := crypto.Keccak256Hash(signer.Bytes())
		proxy := crypto.CreateAddress2(common.HexToAddress(polygonProxyFactory), proxySalt, crypto.Keccak256(code))
		if wallet == proxy {
			return SignatureTypePolyProxy, nil
		}
	}
	return SignatureTypeUnknown, fmt.Errorf("%w: signer %s, wallet %s, chain %d", ErrUnknownWallet, signer.Hex(), wallet.Hex(), chainID)
}

func detectionAddress(value, field string) (common.Address, error) {
	value = strings.TrimSpace(value)
	if !common.IsHexAddress(value) {
		return common.Address{}, fmt.Errorf("invalid %s address %q", field, value)
	}
	address := common.HexToAddress(value)
	if address == (common.Address{}) {
		return common.Address{}, fmt.Errorf("%s address must not be zero", field)
	}
	return address, nil
}

// WithDetectedSignatureType 返回已识别 SignatureType 并设置 FunderAddress 的客户端副本。
// 必须配置 Signer，并提供实际资金钱包地址。失败时不修改原客户端。
func (c *Client) WithDetectedSignatureType(walletAddress string) (*Client, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}
	if c.signer == nil {
		return nil, errors.New("signer is required to detect signature type")
	}
	signatureType, err := DetectSignatureType(c.signer.Address(), walletAddress, c.chainID)
	if err != nil {
		return nil, err
	}
	cloned := *c
	cloned.signatureType = signatureType
	cloned.funderAddress = common.HexToAddress(strings.TrimSpace(walletAddress)).Hex()
	return &cloned, nil
}
