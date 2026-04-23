package clobclient

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	clobAuthDomainName    = "ClobAuthDomain"
	clobAuthDomainVersion = "1"
	// ClobAuthMessage 是 Polymarket L1 EIP-712 签名中的固定文案。
	ClobAuthMessage = "This message attests that I control the given wallet"
)

// L1Signer 定义生成 Polymarket L1 鉴权签名所需的最小能力。
type L1Signer interface {
	Address() string
	SignTypedData(ctx context.Context, typedData apitypes.TypedData) (string, error)
}

// PrivateKeySigner 使用本地 ECDSA 私钥生成 L1 签名。
type PrivateKeySigner struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

// NewPrivateKeySigner 从十六进制私钥字符串创建签名器。
func NewPrivateKeySigner(privateKeyHex string) (*PrivateKeySigner, error) {
	privateKeyHex = strings.TrimSpace(privateKeyHex)
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("private key is required")
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return NewPrivateKeySignerFromECDSA(privateKey), nil
}

// NewPrivateKeySignerFromECDSA 从 go-ethereum 的私钥对象创建签名器。
func NewPrivateKeySignerFromECDSA(privateKey *ecdsa.PrivateKey) *PrivateKeySigner {
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &PrivateKeySigner{
		privateKey: privateKey,
		address:    address,
	}
}

// Address 返回签名器对应的钱包地址。
func (s *PrivateKeySigner) Address() string {
	return s.address.Hex()
}

// SignTypedData 对 EIP-712 typed data 进行签名并返回十六进制编码结果。
func (s *PrivateKeySigner) SignTypedData(_ context.Context, typedData apitypes.TypedData) (string, error) {
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return "", fmt.Errorf("hash typed data: %w", err)
	}

	signature, err := crypto.Sign(digest, s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign typed data: %w", err)
	}

	// 对齐常见钱包的序列化格式，让 V 落在 27/28。
	signature[64] += 27
	return hexutil.Encode(signature), nil
}

// BuildClobAuthTypedData 构造 Polymarket L1 鉴权所需的 EIP-712 结构体。
func BuildClobAuthTypedData(address string, chainID int64, timestamp int64, nonce uint64) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"ClobAuth": {
				{Name: "address", Type: "address"},
				{Name: "timestamp", Type: "string"},
				{Name: "nonce", Type: "uint256"},
				{Name: "message", Type: "string"},
			},
		},
		PrimaryType: "ClobAuth",
		Domain: apitypes.TypedDataDomain{
			Name:    clobAuthDomainName,
			Version: clobAuthDomainVersion,
			ChainId: gethmath.NewHexOrDecimal256(chainID),
		},
		Message: apitypes.TypedDataMessage{
			"address":   address,
			"timestamp": fmt.Sprintf("%d", timestamp),
			"nonce":     new(big.Int).SetUint64(nonce),
			"message":   ClobAuthMessage,
		},
	}
}
