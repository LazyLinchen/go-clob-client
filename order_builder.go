package clobclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	// ZeroBytes32 是 V2 订单 metadata / builder 字段的默认空值。
	ZeroBytes32 = "0x0000000000000000000000000000000000000000000000000000000000000000"
	// ZeroAddress 是 V1 订单 taker 字段的默认空地址。
	ZeroAddress = "0x0000000000000000000000000000000000000000"

	ctfExchangeDomainName      = "Polymarket CTF Exchange"
	ctfExchangeV2DomainName    = ctfExchangeDomainName
	ctfExchangeV1DomainVersion = "1"
	ctfExchangeV2DomainVersion = "2"

	polygonExchangeV1        = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	polygonNegRiskExchangeV1 = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
	polygonExchangeV2        = "0xE111180000d2663C0091e4f400237545B87B996B"
	polygonNegRiskExchangeV2 = "0xe2222d279d744050d28e00520010520000310F59"
	amoyExchangeV2           = "0xE111180000d2663C0091e4f400237545B87B996B"
	amoyNegRiskExchangeV2    = "0xe2222d279d744050d28e00520010520000310F59"

	collateralTokenDecimals = 6
)

// TickSize 表示 Polymarket CLOB 支持的价格最小跳动单位。
type TickSize string

const (
	TickSizeTenth       TickSize = "0.1"
	TickSizeCent        TickSize = "0.01"
	TickSizeMilli       TickSize = "0.001"
	TickSizeTenThousand TickSize = "0.0001"
)

// OrderType 表示订单提交时的执行类型。
type OrderType string

const (
	OrderTypeGTC OrderType = "GTC"
	OrderTypeFOK OrderType = "FOK"
	OrderTypeGTD OrderType = "GTD"
	OrderTypeFAK OrderType = "FAK"
)

type orderRoundConfig struct {
	price  int
	size   int
	amount int
}

// CreateOrderParams 描述本地构造并签名 V2 限价单所需的参数。
type CreateOrderParams struct {
	// Version 控制订单签名和 payload schema，留空时使用客户端默认版本。
	Version CLOBVersion
	// Maker 是实际出资地址，留空时优先使用客户端 FunderAddress，再回退到 signer 地址。
	Maker string
	// TokenID 是要交易的条件 token ID。
	TokenID string
	// Price 是订单价格，按十进制字符串传入以避免 float 精度误差。
	Price string
	// Size 是条件 token 数量，按十进制字符串传入。
	Size string
	// Side 是买卖方向。
	Side Side
	// TickSize 是该市场的最小价格跳动单位，必须由调用方显式提供。
	TickSize TickSize
	// NegRisk 控制是否使用 neg-risk V2 exchange 合约地址签名。
	NegRisk bool
	// Metadata 是 V2 订单 metadata bytes32，留空时使用 ZeroBytes32。
	Metadata string
	// Builder 是 V2 订单 builder bytes32，留空时使用 ZeroBytes32。
	Builder string
	// Expiration 是订单过期时间，Unix seconds；0 表示不过期。
	Expiration int64
	// TimestampMS 是订单时间戳，Unix milliseconds；0 表示使用当前时间。
	TimestampMS int64
	// Salt 允许测试或高级调用方显式指定 salt；留空时自动生成。
	Salt string
	// Taker 是 V1 订单 taker 地址，留空时使用 ZeroAddress；V2 不参与签名和 payload。
	Taker string
	// Nonce 是 V1 订单 nonce，必须非负；V2 不参与签名和 payload。
	Nonce int64
	// FeeRateBps 是 V1 订单 feeRateBps，必须非负；V2 不参与签名和 payload。
	FeeRateBps int64
}

// CreateMarketOrderParams 描述本地构造并签名 V2 market order 所需的参数。
type CreateMarketOrderParams struct {
	// Version 控制订单签名和 payload schema，留空时使用客户端默认版本。
	Version CLOBVersion
	// Maker 是实际出资地址，留空时优先使用客户端 FunderAddress，再回退到 signer 地址。
	Maker string
	// TokenID 是要交易的条件 token ID。
	TokenID string
	// Amount 表示 market order 的输入数量：BUY 为抵押资产金额，SELL 为条件 token 数量。
	Amount string
	// Price 是保护价格；留空或小于等于 0 时会根据当前订单簿计算。
	Price string
	// Side 是买卖方向。
	Side Side
	// TickSize 是该市场的最小价格跳动单位，必须由调用方显式提供。
	TickSize TickSize
	// OrderType 控制盘口价格不足时的行为；留空时按 FOK 处理。
	OrderType OrderType
	// NegRisk 控制是否使用 neg-risk V2 exchange 合约地址签名。
	NegRisk bool
	// Metadata 是 V2 订单 metadata bytes32，留空时使用 ZeroBytes32。
	Metadata string
	// Builder 是 V2 订单 builder bytes32，留空时使用 ZeroBytes32。
	Builder string
	// Expiration 是订单过期时间，Unix seconds；0 表示不过期。
	Expiration int64
	// TimestampMS 是订单时间戳，Unix milliseconds；0 表示使用当前时间。
	TimestampMS int64
	// Salt 允许测试或高级调用方显式指定 salt；留空时自动生成。
	Salt string
	// Taker 是 V1 订单 taker 地址，留空时使用 ZeroAddress；V2 不参与签名和 payload。
	Taker string
	// Nonce 是 V1 订单 nonce，必须非负；V2 不参与签名和 payload。
	Nonce int64
	// FeeRateBps 是 V1 订单 feeRateBps，必须非负；V2 不参与签名和 payload。
	FeeRateBps int64
}

// CreatePostOrderRequestParams 描述构造可直接提交给 PostOrder 的 V2 限价单参数。
type CreatePostOrderRequestParams struct {
	CreateOrderParams
	// Owner 是 CLOB API 下单 payload 中的 owner 字段；留空时使用客户端 API key。
	Owner string
	// OrderType 留空时默认为 GTC。
	OrderType OrderType
	// PostOnly 对应官方 SDK orderToJsonV2 的 postOnly 字段。
	PostOnly bool
	// DeferExec 对应官方 SDK orderToJsonV2 的 deferExec 字段。
	DeferExec bool
}

// CreatePostMarketOrderRequestParams 描述构造可直接提交给 PostOrder 的 V2 market order 参数。
type CreatePostMarketOrderRequestParams struct {
	CreateMarketOrderParams
	// Owner 是 CLOB API 下单 payload 中的 owner 字段；留空时使用客户端 API key。
	Owner string
	// PostOnly 对应官方 SDK orderToJsonV2 的 postOnly 字段。
	PostOnly bool
	// DeferExec 对应官方 SDK orderToJsonV2 的 deferExec 字段。
	DeferExec bool
}

// MarketOrderPriceParams 描述从当前订单簿计算 market order 保护价格的参数。
type MarketOrderPriceParams struct {
	TokenID string
	// Amount 表示 market order 的输入数量：BUY 为抵押资产金额，SELL 为条件 token 数量。
	Amount string
	Side   Side
	// OrderType 控制盘口价格不足时的行为；留空时按 FOK 处理。
	OrderType OrderType
}

// SignOrderV2Options 描述签名已有 V2 订单时的合约选择。
type SignOrderV2Options struct {
	// Version 控制签名 schema，留空时按 order.Version 或客户端默认版本选择。
	Version CLOBVersion
	// ExchangeAddress 允许显式覆盖 verifyingContract；留空时按 ChainID 和 NegRisk 选择默认 V2 合约。
	ExchangeAddress string
	// NegRisk 控制默认合约选择。
	NegRisk bool
}

// OrderV2 表示 Polymarket V2 订单的签名前结构。
type OrderV2 struct {
	Version       CLOBVersion   `json:"-"`
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	Taker         string        `json:"taker,omitempty"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp"`
	Expiration    string        `json:"expiration"`
	Nonce         string        `json:"nonce,omitempty"`
	FeeRateBps    string        `json:"feeRateBps,omitempty"`
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
	Taker         string        `json:"taker,omitempty"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp,omitempty"`
	Expiration    string        `json:"expiration"`
	Nonce         string        `json:"nonce,omitempty"`
	FeeRateBps    string        `json:"feeRateBps,omitempty"`
	Metadata      string        `json:"metadata,omitempty"`
	Builder       string        `json:"builder,omitempty"`
	Signature     string        `json:"signature"`
}

// CreateOrder 按官方 SDK V2 规则构造并签名一笔限价单。
func (c *Client) CreateOrder(ctx context.Context, params CreateOrderParams) (*SignedOrderV2, error) {
	order, err := c.buildOrderV2(params)
	if err != nil {
		return nil, err
	}
	return c.SignOrderV2(ctx, order, SignOrderV2Options{Version: order.Version, NegRisk: params.NegRisk})
}

// CreateMarketOrder 按官方 SDK V2 规则构造并签名一笔 market order。
func (c *Client) CreateMarketOrder(ctx context.Context, params CreateMarketOrderParams) (*SignedOrderV2, error) {
	order, err := c.buildMarketOrderV2(ctx, params)
	if err != nil {
		return nil, err
	}
	return c.SignOrderV2(ctx, order, SignOrderV2Options{Version: order.Version, NegRisk: params.NegRisk})
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

	version, err := c.resolveOrderVersion(options.Version)
	if err != nil {
		return nil, err
	}
	if order.Version != "" {
		version, err = normalizeCLOBVersion(order.Version)
		if err != nil {
			return nil, err
		}
	}
	order.Version = version

	exchangeAddress := strings.TrimSpace(options.ExchangeAddress)
	if exchangeAddress == "" {
		exchangeAddress, err = exchangeAddressForVersion(version, c.chainID, options.NegRisk)
		if err != nil {
			return nil, err
		}
	}
	if !common.IsHexAddress(exchangeAddress) {
		return nil, fmt.Errorf("invalid exchange address %q", exchangeAddress)
	}

	if err := validateOrderForVersion(order, version); err != nil {
		return nil, err
	}

	var typedData apitypes.TypedData
	switch version {
	case CLOBVersionV1:
		typedData, err = BuildOrderV1TypedData(order, c.chainID, exchangeAddress)
	case CLOBVersionV2:
		typedData, err = BuildOrderV2TypedData(order, c.chainID, exchangeAddress)
	default:
		err = fmt.Errorf("unsupported CLOB version %q", version)
	}
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

// CreatePostOrderRequest 构造可直接传给 PostOrder 的 V2 下单请求。
func (c *Client) CreatePostOrderRequest(ctx context.Context, params CreatePostOrderRequestParams) (*PostOrderRequest, error) {
	owner := strings.TrimSpace(params.Owner)
	if owner == "" {
		owner = c.defaultOwner()
	}
	if owner == "" {
		return nil, errors.New("owner is required")
	}

	signedOrder, err := c.CreateOrder(ctx, params.CreateOrderParams)
	if err != nil {
		return nil, err
	}

	payload, err := signedOrder.Payload()
	if err != nil {
		return nil, err
	}

	orderType := params.OrderType
	if orderType == "" {
		orderType = OrderTypeGTC
	}
	orderType, err = normalizeOrderType(orderType, OrderTypeGTC)
	if err != nil {
		return nil, err
	}

	postOnly := params.PostOnly
	deferExec := params.DeferExec
	return &PostOrderRequest{
		Order:     payload,
		Owner:     owner,
		OrderType: string(orderType),
		PostOnly:  &postOnly,
		DeferExec: &deferExec,
	}, nil
}

// CreatePostMarketOrderRequest 构造可直接传给 PostOrder 的 V2 market order 请求。
func (c *Client) CreatePostMarketOrderRequest(ctx context.Context, params CreatePostMarketOrderRequestParams) (*PostOrderRequest, error) {
	owner := strings.TrimSpace(params.Owner)
	if owner == "" {
		owner = c.defaultOwner()
	}
	if owner == "" {
		return nil, errors.New("owner is required")
	}

	signedOrder, err := c.CreateMarketOrder(ctx, params.CreateMarketOrderParams)
	if err != nil {
		return nil, err
	}

	payload, err := signedOrder.Payload()
	if err != nil {
		return nil, err
	}

	orderType := params.OrderType
	if orderType == "" {
		orderType = OrderTypeFOK
	}
	orderType, err = normalizeOrderType(orderType, OrderTypeFOK)
	if err != nil {
		return nil, err
	}

	postOnly := params.PostOnly
	deferExec := params.DeferExec
	return &PostOrderRequest{
		Order:     payload,
		Owner:     owner,
		OrderType: string(orderType),
		PostOnly:  &postOnly,
		DeferExec: &deferExec,
	}, nil
}

// CreateAndPostOrder 构造、签名并提交一笔 V2 限价单。
func (c *Client) CreateAndPostOrder(ctx context.Context, params CreatePostOrderRequestParams) (*PostOrderResponse, error) {
	request, err := c.CreatePostOrderRequest(ctx, params)
	if err != nil {
		return nil, err
	}
	return c.PostOrder(ctx, *request)
}

// CreateAndPostMarketOrder 构造、签名并提交一笔 V2 market order。
func (c *Client) CreateAndPostMarketOrder(ctx context.Context, params CreatePostMarketOrderRequestParams) (*PostOrderResponse, error) {
	request, err := c.CreatePostMarketOrderRequest(ctx, params)
	if err != nil {
		return nil, err
	}
	return c.PostOrder(ctx, *request)
}

// Payload 返回和官方 SDK orderToJsonV2 一致的 order payload。
func (o SignedOrderV2) Payload() (*SignedOrderV2Payload, error) {
	version, err := normalizeCLOBVersion(o.Version)
	if err != nil {
		return nil, err
	}

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
	if version == CLOBVersionV1 {
		payload.Taker = o.Taker
		payload.Timestamp = ""
		payload.Metadata = ""
		payload.Builder = ""
		payload.Nonce = o.Nonce
		payload.FeeRateBps = o.FeeRateBps
	}
	return payload, nil
}

// BuildOrderV1TypedData 构造 Polymarket V1 订单签名所需的 EIP-712 typed data。
func BuildOrderV1TypedData(order OrderV2, chainID int64, exchangeAddress string) (apitypes.TypedData, error) {
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
	expiration, err := parseUint256(order.Expiration, "expiration")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	nonce, err := parseUint256(order.Nonce, "nonce")
	if err != nil {
		return apitypes.TypedData{}, err
	}
	feeRateBps, err := parseUint256(order.FeeRateBps, "feeRateBps")
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
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              ctfExchangeDomainName,
			Version:           ctfExchangeV1DomainVersion,
			ChainId:           gethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchangeAddress,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          salt,
			"maker":         order.Maker,
			"signer":        order.Signer,
			"taker":         order.Taker,
			"tokenId":       tokenID,
			"makerAmount":   makerAmount,
			"takerAmount":   takerAmount,
			"expiration":    expiration,
			"nonce":         nonce,
			"feeRateBps":    feeRateBps,
			"side":          big.NewInt(side),
			"signatureType": big.NewInt(int64(order.SignatureType)),
		},
	}, nil
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

func (c *Client) buildOrderV2(params CreateOrderParams) (OrderV2, error) {
	version, err := c.resolveOrderVersion(params.Version)
	if err != nil {
		return OrderV2{}, err
	}

	roundConfig, err := roundingConfigForTick(params.TickSize)
	if err != nil {
		return OrderV2{}, err
	}

	side, makerAmount, takerAmount, err := buildOrderAmounts(params.Side, params.Size, params.Price, roundConfig)
	if err != nil {
		return OrderV2{}, err
	}

	return c.buildOrderV2FromAmounts(orderV2CommonParams{
		Version:     version,
		Maker:       params.Maker,
		Taker:       params.Taker,
		TokenID:     params.TokenID,
		Side:        side,
		Metadata:    params.Metadata,
		Builder:     params.Builder,
		Expiration:  params.Expiration,
		Nonce:       params.Nonce,
		FeeRateBps:  params.FeeRateBps,
		TimestampMS: params.TimestampMS,
		Salt:        params.Salt,
	}, makerAmount, takerAmount)
}

func (c *Client) buildMarketOrderV2(ctx context.Context, params CreateMarketOrderParams) (OrderV2, error) {
	version, err := c.resolveOrderVersion(params.Version)
	if err != nil {
		return OrderV2{}, err
	}

	roundConfig, err := roundingConfigForTick(params.TickSize)
	if err != nil {
		return OrderV2{}, err
	}

	price := strings.TrimSpace(params.Price)
	if price == "" || decimalIsNonPositive(price) {
		calculatedPrice, err := c.CalculateMarketOrderPrice(ctx, MarketOrderPriceParams{
			TokenID:   params.TokenID,
			Amount:    params.Amount,
			Side:      params.Side,
			OrderType: params.OrderType,
		})
		if err != nil {
			return OrderV2{}, err
		}
		price = string(calculatedPrice)
	}

	priceRoundMode := roundModeNormal
	if version == CLOBVersionV2 {
		priceRoundMode = roundModeDown
	}
	side, makerAmount, takerAmount, err := buildMarketOrderAmounts(params.Side, params.Amount, price, roundConfig, priceRoundMode)
	if err != nil {
		return OrderV2{}, err
	}

	return c.buildOrderV2FromAmounts(orderV2CommonParams{
		Version:     version,
		Maker:       params.Maker,
		Taker:       params.Taker,
		TokenID:     params.TokenID,
		Side:        side,
		Metadata:    params.Metadata,
		Builder:     params.Builder,
		Expiration:  params.Expiration,
		Nonce:       params.Nonce,
		FeeRateBps:  params.FeeRateBps,
		TimestampMS: params.TimestampMS,
		Salt:        params.Salt,
	}, makerAmount, takerAmount)
}

// CalculateMarketOrderPrice 根据当前订单簿计算 market order 能覆盖给定 amount 的保护价格。
func (c *Client) CalculateMarketOrderPrice(ctx context.Context, params MarketOrderPriceParams) (NumberString, error) {
	tokenID := strings.TrimSpace(params.TokenID)
	if tokenID == "" {
		return "", errors.New("tokenID is required")
	}

	side, err := normalizeSide(params.Side)
	if err != nil {
		return "", err
	}
	if _, err := parsePositiveDecimal(params.Amount, "amount"); err != nil {
		return "", err
	}

	orderBook, err := c.GetOrderBook(ctx, tokenID)
	if err != nil {
		return "", err
	}

	levels := orderBook.Bids
	if side == SideBuy {
		levels = orderBook.Asks
	}
	return calculateMarketPriceFromLevels(levels, side, params.Amount, params.OrderType)
}

type orderV2CommonParams struct {
	Version     CLOBVersion
	Maker       string
	Taker       string
	TokenID     string
	Side        Side
	Metadata    string
	Builder     string
	Expiration  int64
	Nonce       int64
	FeeRateBps  int64
	TimestampMS int64
	Salt        string
}

func (c *Client) buildOrderV2FromAmounts(params orderV2CommonParams, makerAmount string, takerAmount string) (OrderV2, error) {
	if c.signer == nil {
		return OrderV2{}, errors.New("l1 signer is not configured")
	}

	version, err := c.resolveOrderVersion(params.Version)
	if err != nil {
		return OrderV2{}, err
	}

	if _, err := parseUint256(params.TokenID, "tokenID"); err != nil {
		return OrderV2{}, err
	}
	if _, err := parseUint256(makerAmount, "makerAmount"); err != nil {
		return OrderV2{}, err
	}
	if _, err := parseUint256(takerAmount, "takerAmount"); err != nil {
		return OrderV2{}, err
	}

	maker := strings.TrimSpace(params.Maker)
	if maker == "" {
		maker = c.funderAddress
	}
	if maker == "" && c.signatureType != SignatureTypeEOA {
		return OrderV2{}, errors.New("funder address is required when signature type is not EOA")
	}
	if maker == "" {
		maker = c.signer.Address()
	}
	if !common.IsHexAddress(maker) {
		return OrderV2{}, fmt.Errorf("invalid maker address %q", maker)
	}

	taker := strings.TrimSpace(params.Taker)
	if taker == "" {
		taker = ZeroAddress
	}
	if !common.IsHexAddress(taker) {
		return OrderV2{}, fmt.Errorf("invalid taker address %q", taker)
	}

	metadata, err := normalizeBytes32(params.Metadata, "metadata")
	if err != nil {
		return OrderV2{}, err
	}
	builder, err := normalizeBytes32(params.Builder, "builder")
	if err != nil {
		return OrderV2{}, err
	}

	if params.Expiration < 0 {
		return OrderV2{}, errors.New("expiration must be non-negative")
	}
	if params.Nonce < 0 {
		return OrderV2{}, errors.New("nonce must be non-negative")
	}
	if params.FeeRateBps < 0 {
		return OrderV2{}, errors.New("feeRateBps must be non-negative")
	}
	if params.TimestampMS < 0 {
		return OrderV2{}, errors.New("timestampMS must be non-negative")
	}

	timestamp := params.TimestampMS
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	salt := strings.TrimSpace(params.Salt)
	if salt == "" {
		generated, err := generateOrderSalt()
		if err != nil {
			return OrderV2{}, err
		}
		salt = generated
	}
	if _, err := parseUint256(salt, "salt"); err != nil {
		return OrderV2{}, err
	}

	return OrderV2{
		Version:       version,
		Salt:          salt,
		Maker:         common.HexToAddress(maker).Hex(),
		Signer:        common.HexToAddress(c.signer.Address()).Hex(),
		Taker:         common.HexToAddress(taker).Hex(),
		TokenID:       strings.TrimSpace(params.TokenID),
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          params.Side,
		SignatureType: c.signatureType,
		Timestamp:     strconv.FormatInt(timestamp, 10),
		Expiration:    strconv.FormatInt(params.Expiration, 10),
		Nonce:         strconv.FormatInt(params.Nonce, 10),
		FeeRateBps:    strconv.FormatInt(params.FeeRateBps, 10),
		Metadata:      metadata,
		Builder:       builder,
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

func validateOrderV1(order OrderV2) error {
	if _, err := parseUint256(order.Salt, "salt"); err != nil {
		return err
	}
	if !common.IsHexAddress(order.Maker) {
		return fmt.Errorf("invalid maker address %q", order.Maker)
	}
	if !common.IsHexAddress(order.Signer) {
		return fmt.Errorf("invalid signer address %q", order.Signer)
	}
	if !common.IsHexAddress(order.Taker) {
		return fmt.Errorf("invalid taker address %q", order.Taker)
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
	if _, err := parseUint256(order.Expiration, "expiration"); err != nil {
		return err
	}
	if _, err := parseUint256(order.Nonce, "nonce"); err != nil {
		return err
	}
	if _, err := parseUint256(order.FeeRateBps, "feeRateBps"); err != nil {
		return err
	}
	return nil
}

func validateOrderForVersion(order OrderV2, version CLOBVersion) error {
	switch version {
	case CLOBVersionV1:
		return validateOrderV1(order)
	case CLOBVersionV2:
		return validateOrderV2(order)
	default:
		return fmt.Errorf("unsupported CLOB version %q", version)
	}
}

func (c *Client) resolveOrderVersion(version CLOBVersion) (CLOBVersion, error) {
	if version != "" {
		return normalizeCLOBVersion(version)
	}
	if c == nil || c.clobVersion == "" {
		return CLOBVersionV2, nil
	}
	return normalizeCLOBVersion(c.clobVersion)
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

func exchangeV1Address(chainID int64, negRisk bool) (string, error) {
	switch chainID {
	case 137:
		if negRisk {
			return polygonNegRiskExchangeV1, nil
		}
		return polygonExchangeV1, nil
	default:
		return "", fmt.Errorf("unsupported V1 chain ID %d", chainID)
	}
}

func exchangeAddressForVersion(version CLOBVersion, chainID int64, negRisk bool) (string, error) {
	switch version {
	case CLOBVersionV1:
		return exchangeV1Address(chainID, negRisk)
	case CLOBVersionV2:
		return exchangeV2Address(chainID, negRisk)
	default:
		return "", fmt.Errorf("unsupported CLOB version %q", version)
	}
}

func roundingConfigForTick(tickSize TickSize) (orderRoundConfig, error) {
	switch tickSize {
	case TickSizeTenth:
		return orderRoundConfig{price: 1, size: 2, amount: 3}, nil
	case TickSizeCent:
		return orderRoundConfig{price: 2, size: 2, amount: 4}, nil
	case TickSizeMilli:
		return orderRoundConfig{price: 3, size: 2, amount: 5}, nil
	case TickSizeTenThousand:
		return orderRoundConfig{price: 4, size: 2, amount: 6}, nil
	default:
		return orderRoundConfig{}, fmt.Errorf("unsupported tickSize %q", tickSize)
	}
}

func buildOrderAmounts(side Side, size string, price string, roundConfig orderRoundConfig) (Side, string, string, error) {
	normalizedSide, err := normalizeSide(side)
	if err != nil {
		return "", "", "", err
	}

	sizeRat, err := parsePositiveDecimal(size, "size")
	if err != nil {
		return "", "", "", err
	}
	priceRat, err := parsePositiveDecimal(price, "price")
	if err != nil {
		return "", "", "", err
	}

	roundedPrice := roundRat(priceRat, roundConfig.price, roundModeNormal)

	var rawMakerAmount *big.Rat
	var rawTakerAmount *big.Rat
	if normalizedSide == SideBuy {
		rawTakerAmount = roundRat(sizeRat, roundConfig.size, roundModeDown)
		rawMakerAmount = new(big.Rat).Mul(rawTakerAmount, roundedPrice)
		rawMakerAmount = normalizeRawAmount(rawMakerAmount, roundConfig.amount)
	} else {
		rawMakerAmount = roundRat(sizeRat, roundConfig.size, roundModeDown)
		rawTakerAmount = new(big.Rat).Mul(rawMakerAmount, roundedPrice)
		rawTakerAmount = normalizeRawAmount(rawTakerAmount, roundConfig.amount)
	}

	makerAmount, err := decimalToUnits(rawMakerAmount, collateralTokenDecimals, "makerAmount")
	if err != nil {
		return "", "", "", err
	}
	takerAmount, err := decimalToUnits(rawTakerAmount, collateralTokenDecimals, "takerAmount")
	if err != nil {
		return "", "", "", err
	}

	return normalizedSide, makerAmount, takerAmount, nil
}

func buildMarketOrderAmounts(side Side, amount string, price string, roundConfig orderRoundConfig, priceRoundMode roundMode) (Side, string, string, error) {
	normalizedSide, err := normalizeSide(side)
	if err != nil {
		return "", "", "", err
	}

	amountRat, err := parsePositiveDecimal(amount, "amount")
	if err != nil {
		return "", "", "", err
	}
	priceRat, err := parsePositiveDecimal(price, "price")
	if err != nil {
		return "", "", "", err
	}

	roundedAmount := roundRat(amountRat, roundConfig.size, roundModeDown)
	if roundedAmount.Sign() <= 0 {
		return "", "", "", errors.New("amount rounds to zero")
	}
	roundedPrice := roundRat(priceRat, roundConfig.price, priceRoundMode)
	if roundedPrice.Sign() <= 0 {
		return "", "", "", errors.New("price rounds to zero")
	}

	var rawMakerAmount *big.Rat
	var rawTakerAmount *big.Rat
	if normalizedSide == SideBuy {
		rawMakerAmount = roundedAmount
		rawTakerAmount = new(big.Rat).Quo(roundedAmount, roundedPrice)
		rawTakerAmount = normalizeRawAmount(rawTakerAmount, roundConfig.amount)
	} else {
		rawMakerAmount = roundedAmount
		rawTakerAmount = new(big.Rat).Mul(roundedAmount, roundedPrice)
		rawTakerAmount = normalizeRawAmount(rawTakerAmount, roundConfig.amount)
	}

	makerAmount, err := decimalToUnits(rawMakerAmount, collateralTokenDecimals, "makerAmount")
	if err != nil {
		return "", "", "", err
	}
	takerAmount, err := decimalToUnits(rawTakerAmount, collateralTokenDecimals, "takerAmount")
	if err != nil {
		return "", "", "", err
	}

	return normalizedSide, makerAmount, takerAmount, nil
}

func calculateMarketPriceFromLevels(levels []BookLevel, side Side, amount string, orderType OrderType) (NumberString, error) {
	if len(levels) == 0 {
		return "", errors.New("order book side has no liquidity")
	}

	amountRat, err := parsePositiveDecimal(amount, "amount")
	if err != nil {
		return "", err
	}

	normalizedSide, err := normalizeSide(side)
	if err != nil {
		return "", err
	}

	total := new(big.Rat)
	for i := len(levels) - 1; i >= 0; i-- {
		level := levels[i]
		price, err := parsePositiveDecimal(string(level.Price), "price")
		if err != nil {
			return "", fmt.Errorf("order book level %d: %w", i, err)
		}
		size, err := parsePositiveDecimal(string(level.Size), "size")
		if err != nil {
			return "", fmt.Errorf("order book level %d: %w", i, err)
		}

		if normalizedSide == SideBuy {
			total.Add(total, new(big.Rat).Mul(size, price))
		} else {
			total.Add(total, size)
		}
		if total.Cmp(amountRat) >= 0 {
			return NumberString(strings.TrimSpace(string(level.Price))), nil
		}
	}

	orderType, err = normalizeOrderType(orderType, OrderTypeFOK)
	if err != nil {
		return "", err
	}
	if orderType == OrderTypeFOK {
		return "", errors.New("not enough liquidity for FOK market order")
	}

	return NumberString(strings.TrimSpace(string(levels[0].Price))), nil
}

func decimalIsNonPositive(value string) bool {
	parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return ok && parsed.Sign() <= 0
}

func normalizeOrderType(orderType OrderType, defaultValue OrderType) (OrderType, error) {
	normalized := OrderType(strings.ToUpper(strings.TrimSpace(string(orderType))))
	if normalized == "" {
		normalized = defaultValue
	}
	switch normalized {
	case OrderTypeGTC, OrderTypeFOK, OrderTypeGTD, OrderTypeFAK:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid order type %q", orderType)
	}
}

func normalizeRawAmount(value *big.Rat, amountDecimals int) *big.Rat {
	if decimalPlacesRat(value) <= amountDecimals {
		return value
	}

	value = roundRat(value, amountDecimals+4, roundModeUp)
	if decimalPlacesRat(value) <= amountDecimals {
		return value
	}
	return roundRat(value, amountDecimals, roundModeDown)
}

type roundMode int

const (
	roundModeNormal roundMode = iota
	roundModeDown
	roundModeUp
)

func roundRat(value *big.Rat, decimals int, mode roundMode) *big.Rat {
	if decimalPlacesRat(value) <= decimals {
		return new(big.Rat).Set(value)
	}

	scale := tenPow(decimals)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled.Num(), scaled.Denom(), remainder)

	switch mode {
	case roundModeNormal:
		doubled := new(big.Int).Mul(remainder, big.NewInt(2))
		if doubled.Cmp(scaled.Denom()) >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		}
	case roundModeUp:
		if remainder.Sign() > 0 {
			quotient.Add(quotient, big.NewInt(1))
		}
	}

	return new(big.Rat).SetFrac(quotient, scale)
}

func decimalPlacesRat(value *big.Rat) int {
	decimal := ratToDecimalString(value)
	if dot := strings.IndexByte(decimal, '.'); dot >= 0 {
		return len(decimal) - dot - 1
	}
	return 0
}

func ratToDecimalString(value *big.Rat) string {
	num := new(big.Int).Set(value.Num())
	den := new(big.Int).Set(value.Denom())

	twos := 0
	for new(big.Int).Mod(den, big.NewInt(2)).Sign() == 0 {
		den.Div(den, big.NewInt(2))
		twos++
	}
	fives := 0
	for new(big.Int).Mod(den, big.NewInt(5)).Sign() == 0 {
		den.Div(den, big.NewInt(5))
		fives++
	}
	if den.Cmp(big.NewInt(1)) != 0 {
		return value.FloatString(18)
	}

	scaleDecimals := twos
	if fives > scaleDecimals {
		scaleDecimals = fives
	}

	scaled := new(big.Int).Mul(num, tenPow(scaleDecimals))
	scaled.Quo(scaled, value.Denom())
	negative := scaled.Sign() < 0
	if negative {
		scaled.Abs(scaled)
	}

	digits := scaled.Text(10)
	if scaleDecimals == 0 {
		if negative {
			return "-" + digits
		}
		return digits
	}
	if len(digits) <= scaleDecimals {
		digits = strings.Repeat("0", scaleDecimals-len(digits)+1) + digits
	}

	intPart := digits[:len(digits)-scaleDecimals]
	fracPart := strings.TrimRight(digits[len(digits)-scaleDecimals:], "0")
	if fracPart == "" {
		if negative {
			return "-" + intPart
		}
		return intPart
	}
	if negative {
		return "-" + intPart + "." + fracPart
	}
	return intPart + "." + fracPart
}

func decimalToUnits(value *big.Rat, decimals int, fieldName string) (string, error) {
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(tenPow(decimals)))
	if scaled.Denom().Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("%s has more than %d decimal places", fieldName, decimals)
	}
	if scaled.Num().Sign() < 0 {
		return "", fmt.Errorf("%s must be non-negative", fieldName)
	}
	return scaled.Num().String(), nil
}

func parsePositiveDecimal(value string, fieldName string) (*big.Rat, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s is required", fieldName)
	}

	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", fieldName, value)
	}
	if parsed.Sign() <= 0 {
		return nil, fmt.Errorf("%s must be positive", fieldName)
	}
	return parsed, nil
}

func parseUint256(value string, fieldName string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s is required", fieldName)
	}

	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", fieldName, value)
	}
	if parsed.Sign() < 0 {
		return nil, fmt.Errorf("%s must be non-negative", fieldName)
	}
	if parsed.BitLen() > 256 {
		return nil, fmt.Errorf("%s exceeds uint256", fieldName)
	}
	return parsed, nil
}

func normalizeBytes32(value string, fieldName string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ZeroBytes32, nil
	}
	if !strings.HasPrefix(value, "0x") && !strings.HasPrefix(value, "0X") {
		return "", fmt.Errorf("%s must be a 0x-prefixed bytes32 value", fieldName)
	}
	payload := value[2:]
	if len(payload) != 64 {
		return "", fmt.Errorf("%s must be 32 bytes", fieldName)
	}
	if _, err := hex.DecodeString(payload); err != nil {
		return "", fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	return "0x" + strings.ToLower(payload), nil
}

func sideUint8(side Side) (int64, error) {
	normalized, err := normalizeSide(side)
	if err != nil {
		return 0, err
	}
	if normalized == SideBuy {
		return 0, nil
	}
	return 1, nil
}

func generateOrderSalt() (string, error) {
	max := big.NewInt(time.Now().UnixMilli() + 1)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate order salt: %w", err)
	}
	return value.String(), nil
}

func tenPow(decimals int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
}
