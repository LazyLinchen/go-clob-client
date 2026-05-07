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
)

const (
	// ZeroBytes32 是 V2 订单 metadata / builder 字段的默认空值。
	ZeroBytes32 = "0x0000000000000000000000000000000000000000000000000000000000000000"

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
	// BuilderCode 是 Builder 字段的明确别名；留空时使用客户端默认 builder code。
	BuilderCode string
	// Expiration 是订单过期时间，Unix seconds；0 表示不过期。
	Expiration int64
	// TimestampMS 是订单时间戳，Unix milliseconds；0 表示使用当前时间。
	TimestampMS int64
	// Salt 允许测试或高级调用方显式指定 salt；留空时自动生成。
	Salt string
}

// CreateMarketOrderParams 描述本地构造并签名 V2 market order 所需的参数。
type CreateMarketOrderParams struct {
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
	// BuilderCode 是 Builder 字段的明确别名；留空时使用客户端默认 builder code。
	BuilderCode string
	// UserUSDCBalance 启用 BUY market order 的 fee-aware amount 调整，避免 amount+fees 超出可用 USDC。
	UserUSDCBalance string
	// FeeInfo 允许调用方显式传入 V2 market fee 参数；留空且 UserUSDCBalance 生效时会自动查询。
	FeeInfo *MarketFeeInfo
	// BuilderTakerFeeBps 允许调用方显式传入 builder taker fee bps；留空且 builder code 非零时会自动查询。
	BuilderTakerFeeBps *int64
	// Expiration 是订单过期时间，Unix seconds；0 表示不过期。
	Expiration int64
	// TimestampMS 是订单时间戳，Unix milliseconds；0 表示使用当前时间。
	TimestampMS int64
	// Salt 允许测试或高级调用方显式指定 salt；留空时自动生成。
	Salt string
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

// CreateOrder 按官方 SDK V2 规则构造并签名一笔限价单。
func (c *Client) CreateOrder(ctx context.Context, params CreateOrderParams) (*SignedOrderV2, error) {
	order, err := c.buildOrderV2(ctx, params)
	if err != nil {
		return nil, err
	}
	return c.SignOrderV2(ctx, order, SignOrderV2Options{NegRisk: params.NegRisk})
}

// CreateMarketOrder 按官方 SDK V2 规则构造并签名一笔 market order。
func (c *Client) CreateMarketOrder(ctx context.Context, params CreateMarketOrderParams) (*SignedOrderV2, error) {
	order, err := c.buildMarketOrderV2(ctx, params)
	if err != nil {
		return nil, err
	}
	return c.SignOrderV2(ctx, order, SignOrderV2Options{NegRisk: params.NegRisk})
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

func (c *Client) buildOrderV2(ctx context.Context, params CreateOrderParams) (OrderV2, error) {
	roundConfig, err := roundingConfigForTick(params.TickSize)
	if err != nil {
		return OrderV2{}, err
	}

	side, makerAmount, takerAmount, err := buildOrderAmounts(params.Side, params.Size, params.Price, roundConfig)
	if err != nil {
		return OrderV2{}, err
	}

	return c.buildOrderV2FromAmounts(ctx, orderV2CommonParams{
		Maker:       params.Maker,
		TokenID:     params.TokenID,
		Side:        side,
		Metadata:    params.Metadata,
		Builder:     params.Builder,
		BuilderCode: params.BuilderCode,
		Expiration:  params.Expiration,
		TimestampMS: params.TimestampMS,
		Salt:        params.Salt,
	}, makerAmount, takerAmount)
}

func (c *Client) buildMarketOrderV2(ctx context.Context, params CreateMarketOrderParams) (OrderV2, error) {
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

	amount, err := c.adjustMarketBuyAmountForFees(ctx, params, price)
	if err != nil {
		return OrderV2{}, err
	}

	side, makerAmount, takerAmount, err := buildMarketOrderAmounts(params.Side, amount, price, roundConfig, roundModeDown)
	if err != nil {
		return OrderV2{}, err
	}

	return c.buildOrderV2FromAmounts(ctx, orderV2CommonParams{
		Maker:       params.Maker,
		TokenID:     params.TokenID,
		Side:        side,
		Metadata:    params.Metadata,
		Builder:     params.Builder,
		BuilderCode: params.BuilderCode,
		Expiration:  params.Expiration,
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
	Maker       string
	TokenID     string
	Side        Side
	Metadata    string
	Builder     string
	BuilderCode string
	Expiration  int64
	TimestampMS int64
	Salt        string
}

func (c *Client) buildOrderV2FromAmounts(ctx context.Context, params orderV2CommonParams, makerAmount string, takerAmount string) (OrderV2, error) {
	if c.signer == nil {
		return OrderV2{}, errors.New("l1 signer is not configured")
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

	maker, err := c.resolveOrderMaker(ctx, params.Maker, params.TokenID)
	if err != nil {
		return OrderV2{}, err
	}

	metadata, err := normalizeBytes32(params.Metadata, "metadata")
	if err != nil {
		return OrderV2{}, err
	}
	builder, err := c.resolveOrderBuilder(params.Builder, params.BuilderCode)
	if err != nil {
		return OrderV2{}, err
	}

	if params.Expiration < 0 {
		return OrderV2{}, errors.New("expiration must be non-negative")
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
		Salt:          salt,
		Maker:         common.HexToAddress(maker).Hex(),
		Signer:        common.HexToAddress(c.signer.Address()).Hex(),
		TokenID:       strings.TrimSpace(params.TokenID),
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          params.Side,
		SignatureType: c.signatureType,
		Timestamp:     strconv.FormatInt(timestamp, 10),
		Expiration:    strconv.FormatInt(params.Expiration, 10),
		Metadata:      metadata,
		Builder:       builder,
	}, nil
}

func (c *Client) resolveOrderMaker(ctx context.Context, requestedMaker string, tokenID string) (string, error) {
	maker := strings.TrimSpace(requestedMaker)
	if maker == "" {
		maker = c.funderAddress
	}
	if maker == "" && c.signatureType != SignatureTypeEOA && c.autoDiscoverFunder {
		discovery, err := c.DiscoverFunder(ctx, FunderDiscoveryParams{
			AssetID: strings.TrimSpace(tokenID),
		})
		if err != nil {
			return "", fmt.Errorf("auto-discover funder: %w", err)
		}
		if discovery.Preferred != nil {
			maker = discovery.Preferred.Address
		} else {
			subject := funderDiscoverySubject(discovery)
			if len(discovery.Candidates) == 0 {
				return "", fmt.Errorf("funder address is required when signature type is not EOA; auto-discovery found no candidates for %s", subject)
			}
			return "", fmt.Errorf("funder address is required when signature type is not EOA; auto-discovery found %d candidates for %s", len(discovery.Candidates), subject)
		}
	}
	if maker == "" && c.signatureType != SignatureTypeEOA {
		return "", errors.New("funder address is required when signature type is not EOA")
	}
	if maker == "" {
		maker = c.signer.Address()
	}
	if !common.IsHexAddress(maker) {
		return "", fmt.Errorf("invalid maker address %q", maker)
	}
	return common.HexToAddress(maker).Hex(), nil
}

func (c *Client) resolveOrderBuilder(builder string, builderCode string) (string, error) {
	builder = strings.TrimSpace(builder)
	builderCode = strings.TrimSpace(builderCode)

	var normalizedBuilder string
	if builder != "" {
		var err error
		normalizedBuilder, err = normalizeBytes32(builder, "builder")
		if err != nil {
			return "", err
		}
	}

	var normalizedCode string
	if builderCode != "" {
		var err error
		normalizedCode, err = normalizeBuilderCode(builderCode)
		if err != nil {
			return "", err
		}
	}

	if normalizedBuilder != "" && normalizedCode != "" && !strings.EqualFold(normalizedBuilder, normalizedCode) {
		return "", errors.New("builder and builderCode must match when both are provided")
	}
	if normalizedCode != "" {
		return normalizedCode, nil
	}
	if normalizedBuilder != "" {
		return normalizedBuilder, nil
	}
	if c != nil && c.builderCode != "" {
		return c.builderCode, nil
	}
	return ZeroBytes32, nil
}

func (c *Client) adjustMarketBuyAmountForFees(ctx context.Context, params CreateMarketOrderParams, price string) (string, error) {
	amount := strings.TrimSpace(params.Amount)
	if strings.TrimSpace(params.UserUSDCBalance) == "" {
		return amount, nil
	}

	side, err := normalizeSide(params.Side)
	if err != nil {
		return "", err
	}
	if side != SideBuy {
		return amount, nil
	}

	builder, err := c.resolveOrderBuilder(params.Builder, params.BuilderCode)
	if err != nil {
		return "", err
	}

	feeInfo, err := c.resolveMarketFeeInfo(ctx, params.TokenID, params.FeeInfo)
	if err != nil {
		return "", err
	}

	builderTakerFeeBps, err := c.resolveBuilderTakerFeeBps(ctx, builder, params.BuilderTakerFeeBps)
	if err != nil {
		return "", err
	}

	return adjustBuyAmountForFees(amount, price, params.UserUSDCBalance, feeInfo, builderTakerFeeBps)
}

func (c *Client) resolveMarketFeeInfo(ctx context.Context, tokenID string, explicit *MarketFeeInfo) (MarketFeeInfo, error) {
	if explicit != nil {
		return explicit.normalized()
	}

	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return MarketFeeInfo{}, errors.New("tokenID is required")
	}

	book, err := c.GetOrderBook(ctx, tokenID)
	if err != nil {
		return MarketFeeInfo{}, fmt.Errorf("load order book for fee info: %w", err)
	}
	conditionID := strings.TrimSpace(book.Market)
	if conditionID == "" {
		market, err := c.GetMarketByToken(ctx, tokenID)
		if err != nil {
			return MarketFeeInfo{}, fmt.Errorf("resolve market for fee info: %w", err)
		}
		conditionID = strings.TrimSpace(market.ConditionID)
	}
	if conditionID == "" {
		return MarketFeeInfo{}, errors.New("market conditionID is required to resolve fee info")
	}

	marketInfo, err := c.GetCLOBMarketInfo(ctx, conditionID)
	if err != nil {
		return MarketFeeInfo{}, fmt.Errorf("load market fee info: %w", err)
	}
	return marketInfo.FeeDetails.marketFeeInfo().normalized()
}

func (c *Client) resolveBuilderTakerFeeBps(ctx context.Context, builder string, explicit *int64) (int64, error) {
	if explicit != nil {
		if *explicit < 0 {
			return 0, errors.New("builder taker fee bps must be non-negative")
		}
		return *explicit, nil
	}
	if !isBuilderOrder(builder) {
		return 0, nil
	}

	rates, err := c.GetBuilderFeeRates(ctx, builder)
	if err != nil {
		return 0, fmt.Errorf("load builder fee rates: %w", err)
	}
	return rates.TakerFeeRateBps, nil
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

func adjustBuyAmountForFees(amount string, price string, userUSDCBalance string, feeInfo MarketFeeInfo, builderTakerFeeBps int64) (string, error) {
	amountRat, err := parsePositiveDecimal(amount, "amount")
	if err != nil {
		return "", err
	}
	priceRat, err := parsePositiveDecimal(price, "price")
	if err != nil {
		return "", err
	}
	balanceRat, err := parsePositiveDecimal(userUSDCBalance, "userUSDCBalance")
	if err != nil {
		return "", err
	}

	feeInfo, err = feeInfo.normalized()
	if err != nil {
		return "", err
	}
	feeRate, err := parseNonNegativeDecimal(feeInfo.Rate, "fee rate")
	if err != nil {
		return "", err
	}
	builderRate, err := builderFeeRateFromBps(builderTakerFeeBps)
	if err != nil {
		return "", err
	}

	oneMinusPrice := new(big.Rat).Sub(big.NewRat(1, 1), priceRat)
	if oneMinusPrice.Sign() < 0 {
		return "", errors.New("price must be less than or equal to 1 when adjusting buy amount for fees")
	}

	platformFeeBase := new(big.Rat).Mul(priceRat, oneMinusPrice)
	platformFeeRate := new(big.Rat).Mul(feeRate, powRat(platformFeeBase, feeInfo.Exponent))
	platformFee := new(big.Rat).Mul(new(big.Rat).Quo(amountRat, priceRat), platformFeeRate)
	builderFee := new(big.Rat).Mul(amountRat, builderRate)
	totalCost := new(big.Rat).Add(amountRat, platformFee)
	totalCost.Add(totalCost, builderFee)

	if balanceRat.Cmp(totalCost) > 0 {
		return strings.TrimSpace(amount), nil
	}

	denominator := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(platformFeeRate, priceRat))
	denominator.Add(denominator, builderRate)
	adjusted := new(big.Rat).Quo(balanceRat, denominator)
	if adjusted.Sign() <= 0 {
		return "", errors.New("adjusted buy amount is not positive")
	}
	return ratToDecimalString(adjusted), nil
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

func parseNonNegativeDecimal(value string, fieldName string) (*big.Rat, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s is required", fieldName)
	}

	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", fieldName, value)
	}
	if parsed.Sign() < 0 {
		return nil, fmt.Errorf("%s must be non-negative", fieldName)
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

func powRat(value *big.Rat, exponent int64) *big.Rat {
	if exponent <= 0 {
		return big.NewRat(1, 1)
	}

	result := big.NewRat(1, 1)
	for i := int64(0); i < exponent; i++ {
		result.Mul(result, value)
	}
	return result
}

func tenPow(decimals int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
}
