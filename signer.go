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
	ClobAuthMessage       = "This message attests that I control the given wallet"
)

type L1Signer interface {
	Address() string
	SignTypedData(ctx context.Context, typedData apitypes.TypedData) (string, error)
}

type PrivateKeySigner struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

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

func NewPrivateKeySignerFromECDSA(privateKey *ecdsa.PrivateKey) *PrivateKeySigner {
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &PrivateKeySigner{
		privateKey: privateKey,
		address:    address,
	}
}

func (s *PrivateKeySigner) Address() string {
	return s.address.Hex()
}

func (s *PrivateKeySigner) SignTypedData(_ context.Context, typedData apitypes.TypedData) (string, error) {
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return "", fmt.Errorf("hash typed data: %w", err)
	}

	signature, err := crypto.Sign(digest, s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign typed data: %w", err)
	}

	// Match wallet-style serialized signatures where V is 27/28.
	signature[64] += 27
	return hexutil.Encode(signature), nil
}

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
