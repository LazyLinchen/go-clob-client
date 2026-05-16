package clobclient

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

	// orderTypeString 是 CTF Exchange V2 Order 结构的 EIP-712 类型字符串，POLY_1271 包裹签名的尾部需要原文附上。
	// 必须与 clob-client-v2/src/order-utils/exchangeOrderBuilderV2.ts 中的 ORDER_TYPE_STRING 完全一致。
	orderTypeString = "Order(uint256 salt,address maker,address signer,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType,uint256 timestamp,bytes32 metadata,bytes32 builder)"
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
//
// 对 SignatureType 0/1/2（EOA、Proxy、Gnosis Safe），输出标准 65 字节 ECDSA 签名。
// 对 SignatureType 3（POLY_1271 / Deposit Wallet），输出 ERC-7739 包裹签名，
// 其布局与 @polymarket/clob-client-v2/src/order-utils/exchangeOrderBuilderV2.ts 中的
// ExchangeOrderBuilderV2.buildOrderSignature 一致：
//
//	innerSig(65) || appDomainSep(32) || contentsHash(32) || orderTypeString || uint16BE(len(orderTypeString))
//
// 其中 innerSig 是对 ERC-7739 TypedDataSign 结构的 ECDSA 签名，
// 外层 EIP712Domain 仍是 CTF Exchange V2，内层字段描述 order.Signer 处的 DepositWallet 域。
func (c *Client) SignOrderV2(ctx context.Context, order OrderV2, options SignOrderV2Options) (*SignedOrderV2, error) {
	if c.signer == nil {
		return nil, errors.New("l1 signer is not configured")
	}

	if err := validateOrderV2(order); err != nil {
		return nil, err
	}

	if order.SignatureType == SignatureTypePoly1271 {
		return c.signOrderV2Poly1271(ctx, order, options)
	}

	signerAddress := strings.TrimSpace(c.signer.Address())
	if strings.TrimSpace(order.Signer) == "" {
		order.Signer = signerAddress
	}
	if !strings.EqualFold(order.Signer, signerAddress) {
		return nil, errors.New("order signer does not match configured signer")
	}

	exchangeAddress, err := c.resolveExchangeV2Address(options)
	if err != nil {
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

func (c *Client) signOrderV2Poly1271(ctx context.Context, order OrderV2, options SignOrderV2Options) (*SignedOrderV2, error) {
	if strings.TrimSpace(order.Maker) == "" {
		return nil, errors.New("POLY_1271 orders require maker to be the deposit wallet address")
	}
	if strings.TrimSpace(order.Signer) == "" {
		return nil, errors.New("POLY_1271 orders require signer to be the deposit wallet address")
	}
	if !strings.EqualFold(order.Maker, order.Signer) {
		return nil, errors.New("POLY_1271 orders require maker and signer to match the deposit wallet address")
	}

	// 防御 SignOrderV2 直接调用绕过 resolveOrderMaker 的场景：
	// EOA 必须真的拥有 order.Maker 这个 deposit wallet，否则链上 isValidSignature 会拒签。
	expected, err := DeriveDepositWalletAddress(c.signer.Address(), c.chainID)
	if err != nil {
		return nil, fmt.Errorf("derive expected deposit wallet: %w", err)
	}
	if !strings.EqualFold(order.Maker, expected) {
		return nil, fmt.Errorf(
			"POLY_1271 order.maker %s does not match deposit wallet derived from signer %s (expected %s)",
			order.Maker, c.signer.Address(), expected,
		)
	}

	exchangeAddress, err := c.resolveExchangeV2Address(options)
	if err != nil {
		return nil, err
	}

	typedData, err := buildPoly1271TypedData(order, c.chainID, exchangeAddress)
	if err != nil {
		return nil, err
	}

	innerSigHex, err := c.signer.SignTypedData(ctx, typedData)
	if err != nil {
		return nil, err
	}
	innerSig, err := hexutil.Decode(innerSigHex)
	if err != nil {
		return nil, fmt.Errorf("decode inner signature: %w", err)
	}
	if len(innerSig) != 65 {
		return nil, fmt.Errorf("inner signature must be 65 bytes, got %d", len(innerSig))
	}

	appDomainSep, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("compute app domain separator: %w", err)
	}

	orderMessage, ok := typedData.Message["contents"].(apitypes.TypedDataMessage)
	if !ok {
		return nil, errors.New("typed data contents missing")
	}
	contentsHash, err := typedData.HashStruct("Order", orderMessage)
	if err != nil {
		return nil, fmt.Errorf("compute contents hash: %w", err)
	}

	typeBytes := []byte(orderTypeString)
	if len(typeBytes) > 0xFFFF {
		return nil, fmt.Errorf("order type string exceeds uint16 length: %d", len(typeBytes))
	}

	wrapped := make([]byte, 0, len(innerSig)+len(appDomainSep)+len(contentsHash)+len(typeBytes)+2)
	wrapped = append(wrapped, innerSig...)
	wrapped = append(wrapped, appDomainSep...)
	wrapped = append(wrapped, contentsHash...)
	wrapped = append(wrapped, typeBytes...)

	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(typeBytes)))
	wrapped = append(wrapped, lenBuf...)

	return &SignedOrderV2{
		OrderV2:   order,
		Signature: "0x" + hex.EncodeToString(wrapped),
	}, nil
}

func (c *Client) resolveExchangeV2Address(options SignOrderV2Options) (string, error) {
	exchangeAddress := strings.TrimSpace(options.ExchangeAddress)
	if exchangeAddress == "" {
		var err error
		exchangeAddress, err = exchangeV2Address(c.chainID, options.NegRisk)
		if err != nil {
			return "", err
		}
	}
	if !common.IsHexAddress(exchangeAddress) {
		return "", fmt.Errorf("invalid exchange address %q", exchangeAddress)
	}
	return exchangeAddress, nil
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
	orderMessage, err := orderV2Message(order)
	if err != nil {
		return apitypes.TypedData{}, err
	}

	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": ctfExchangeV2EIP712DomainType(),
			"Order":        ctfExchangeV2OrderType(),
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              ctfExchangeDomainName,
			Version:           ctfExchangeV2DomainVersion,
			ChainId:           gethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchangeAddress,
		},
		Message: orderMessage,
	}, nil
}

// buildPoly1271TypedData 构造 POLY_1271 内层 ERC-7739 TypedDataSign 的 EIP-712 typed data。
// 外层 EIP712Domain 仍是 CTF Exchange V2，内层字段引用 order.Signer 处的 DepositWallet 域。
func buildPoly1271TypedData(order OrderV2, chainID int64, exchangeAddress string) (apitypes.TypedData, error) {
	orderMessage, err := orderV2Message(order)
	if err != nil {
		return apitypes.TypedData{}, err
	}

	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": ctfExchangeV2EIP712DomainType(),
			"TypedDataSign": {
				{Name: "contents", Type: "Order"},
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
				{Name: "salt", Type: "bytes32"},
			},
			"Order": ctfExchangeV2OrderType(),
		},
		PrimaryType: "TypedDataSign",
		Domain: apitypes.TypedDataDomain{
			Name:              ctfExchangeDomainName,
			Version:           ctfExchangeV2DomainVersion,
			ChainId:           gethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchangeAddress,
		},
		Message: apitypes.TypedDataMessage{
			"contents":          orderMessage,
			"name":              depositWalletDomainName,
			"version":           depositWalletDomainVersion,
			"chainId":           gethmath.NewHexOrDecimal256(chainID),
			"verifyingContract": order.Signer,
			"salt":              ZeroBytes32,
		},
	}, nil
}

func ctfExchangeV2EIP712DomainType() []apitypes.Type {
	return []apitypes.Type{
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	}
}

func ctfExchangeV2OrderType() []apitypes.Type {
	return []apitypes.Type{
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
	}
}

func orderV2Message(order OrderV2) (apitypes.TypedDataMessage, error) {
	salt, err := parseUint256(order.Salt, "salt")
	if err != nil {
		return nil, err
	}
	tokenID, err := parseUint256(order.TokenID, "tokenID")
	if err != nil {
		return nil, err
	}
	makerAmount, err := parseUint256(order.MakerAmount, "makerAmount")
	if err != nil {
		return nil, err
	}
	takerAmount, err := parseUint256(order.TakerAmount, "takerAmount")
	if err != nil {
		return nil, err
	}
	timestamp, err := parseUint256(order.Timestamp, "timestamp")
	if err != nil {
		return nil, err
	}
	side, err := sideUint8(order.Side)
	if err != nil {
		return nil, err
	}

	return apitypes.TypedDataMessage{
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
