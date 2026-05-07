package clobclient

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	ctfExchangeDomainName      = "Polymarket CTF Exchange"
	ctfExchangeV2DomainName    = ctfExchangeDomainName
	ctfExchangeV2DomainVersion = "2"

	polygonExchangeV2        = "0xE111180000d2663C0091e4f400237545B87B996B"
	polygonNegRiskExchangeV2 = "0xe2222d279d744050d28e00520010520000310F59"
	amoyExchangeV2           = "0xE111180000d2663C0091e4f400237545B87B996B"
	amoyNegRiskExchangeV2    = "0xe2222d279d744050d28e00520010520000310F59"
)

// SignOrderV2Options 描述签名已有 V2 订单时的合约选择。
type SignOrderV2Options struct {
	// ExchangeAddress 允许显式覆盖 verifyingContract；留空时按 ChainID 和 NegRisk 选择默认 V2 合约。
	ExchangeAddress string
	// NegRisk 控制默认合约选择。
	NegRisk bool
}

// OrderV2 表示 Polymarket V2 订单的签名前结构。
type OrderV2 struct {
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp"`
	Expiration    string        `json:"expiration"`
	Metadata      string        `json:"metadata"`
	Builder       string        `json:"builder"`
}

// SignedOrderV2 表示带 EIP-712 签名的 V2 订单。
type SignedOrderV2 struct {
	OrderV2
	Signature string `json:"signature"`
}

// SignedOrderV2Payload 是官方 SDK orderToJsonV2 输出中的 order 子对象形状。
type SignedOrderV2Payload struct {
	Salt          int64         `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp,omitempty"`
	Expiration    string        `json:"expiration"`
	Metadata      string        `json:"metadata,omitempty"`
	Builder       string        `json:"builder,omitempty"`
	Signature     string        `json:"signature"`
}

// SignOrderV2 对已有 V2 订单生成 EIP-712 签名。
func (c *Client) SignOrderV2(ctx context.Context, order OrderV2, options SignOrderV2Options) (*SignedOrderV2, error) {
	if c.signer == nil {
		return nil, errors.New("l1 signer is not configured")
	}

	signerAddress := strings.TrimSpace(c.signer.Address())
	if strings.TrimSpace(order.Signer) == "" {
		order.Signer = signerAddress
	}
	if !strings.EqualFold(order.Signer, signerAddress) {
		return nil, errors.New("order signer does not match configured signer")
	}

	exchangeAddress := strings.TrimSpace(options.ExchangeAddress)
	if exchangeAddress == "" {
		var err error
		exchangeAddress, err = exchangeV2Address(c.chainID, options.NegRisk)
		if err != nil {
			return nil, err
		}
	}
	if !common.IsHexAddress(exchangeAddress) {
		return nil, fmt.Errorf("invalid exchange address %q", exchangeAddress)
	}

	if err := validateOrderV2(order); err != nil {
		return nil, err
	}

	typedData, err := BuildOrderV2TypedData(order, c.chainID, exchangeAddress)
	if err != nil {
		return nil, err
	}

	signature, err := c.signer.SignTypedData(ctx, typedData)
	if err != nil {
		return nil, err
	}

	return &SignedOrderV2{
		OrderV2:   order,
		Signature: signature,
	}, nil
}

// Payload 返回和官方 SDK orderToJsonV2 一致的 order payload。
func (o SignedOrderV2) Payload() (*SignedOrderV2Payload, error) {
	salt, err := strconv.ParseInt(strings.TrimSpace(o.Salt), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}
	if salt < 0 {
		return nil, errors.New("order salt must be non-negative")
	}

	payload := &SignedOrderV2Payload{
		Salt:          salt,
		Maker:         o.Maker,
		Signer:        o.Signer,
		TokenID:       o.TokenID,
		MakerAmount:   o.MakerAmount,
		TakerAmount:   o.TakerAmount,
		Side:          o.Side,
		SignatureType: o.SignatureType,
		Timestamp:     o.Timestamp,
		Expiration:    o.Expiration,
		Metadata:      o.Metadata,
		Builder:       o.Builder,
		Signature:     o.Signature,
	}
	return payload, nil
}

// BuildOrderV2TypedData 构造 Polymarket V2 订单签名所需的 EIP-712 typed data。
func BuildOrderV2TypedData(order OrderV2, chainID int64, exchangeAddress string) (apitypes.TypedData, error) {
	salt, err := parseUint256(order.Salt, "salt")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	tokenID, err := parseUint256(order.TokenID, "tokenID")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	makerAmount, err := parseUint256(order.MakerAmount, "makerAmount")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	takerAmount, err := parseUint256(order.TakerAmount, "takerAmount")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	timestamp, err := parseUint256(order.Timestamp, "timestamp")
	if err != nil {
		return apitypes.TypedData{}, err
	}

	side, err := sideUint8(order.Side)
	if err != nil {
		return apitypes.TypedData{}, err
	}

	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
				{Name: "timestamp", Type: "uint256"},
				{Name: "metadata", Type: "bytes32"},
				{Name: "builder", Type: "bytes32"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              ctfExchangeDomainName,
			Version:           ctfExchangeV2DomainVersion,
			ChainId:           gethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchangeAddress,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          salt,
			"maker":         order.Maker,
			"signer":        order.Signer,
			"tokenId":       tokenID,
			"makerAmount":   makerAmount,
			"takerAmount":   takerAmount,
			"side":          big.NewInt(side),
			"signatureType": big.NewInt(int64(order.SignatureType)),
			"timestamp":     timestamp,
			"metadata":      order.Metadata,
			"builder":       order.Builder,
		},
	}, nil
}

func validateOrderV2(order OrderV2) error {
	if _, err := parseUint256(order.Salt, "salt"); err != nil {
		return err
	}
	if !common.IsHexAddress(order.Maker) {
		return fmt.Errorf("invalid maker address %q", order.Maker)
	}
	if !common.IsHexAddress(order.Signer) {
		return fmt.Errorf("invalid signer address %q", order.Signer)
	}
	if _, err := parseUint256(order.TokenID, "tokenID"); err != nil {
		return err
	}
	if _, err := parseUint256(order.MakerAmount, "makerAmount"); err != nil {
		return err
	}
	if _, err := parseUint256(order.TakerAmount, "takerAmount"); err != nil {
		return err
	}
	if _, err := sideUint8(order.Side); err != nil {
		return err
	}
	if order.SignatureType < SignatureTypeEOA || order.SignatureType > SignatureTypePoly1271 {
		return fmt.Errorf("invalid signature type %d", order.SignatureType)
	}
	if _, err := parseUint256(order.Timestamp, "timestamp"); err != nil {
		return err
	}
	if _, err := parseUint256(order.Expiration, "expiration"); err != nil {
		return err
	}
	if _, err := normalizeBytes32(order.Metadata, "metadata"); err != nil {
		return err
	}
	if _, err := normalizeBytes32(order.Builder, "builder"); err != nil {
		return err
	}
	return nil
}

func exchangeV2Address(chainID int64, negRisk bool) (string, error) {
	switch chainID {
	case 137:
		if negRisk {
			return polygonNegRiskExchangeV2, nil
		}
		return polygonExchangeV2, nil
	case 80002:
		if negRisk {
			return amoyNegRiskExchangeV2, nil
		}
		return amoyExchangeV2, nil
	default:
		return "", fmt.Errorf("unsupported chain ID %d", chainID)
	}
}
