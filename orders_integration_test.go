package clobclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	liveGammaMarketsHost         = "https://gamma-api.polymarket.com"
	livePreferredPostMarketLabel = "btc-updown-15m"
	liveDefaultPostAmount        = "1"
)

var livePreferredPostMarketSlugPrefixes = []string{
	"btc-updown-15m",
}

type livePostOrderConfig struct {
	Question        string
	Slug            string
	TokenID         string
	Side            Side
	TickSize        TickSize
	NegRisk         bool
	MarketAmount    string
	MarketPrice     string
	MarketOrderType OrderType
	LimitSize       string
	LimitPrice      string
}

type liveGammaMarket struct {
	Question        string           `json:"question"`
	Slug            string           `json:"slug"`
	EventSlug       string           `json:"eventSlug"`
	EnableOrderBook bool             `json:"enableOrderBook"`
	Active          bool             `json:"active"`
	Closed          bool             `json:"closed"`
	ClobTokenIDs    json.RawMessage  `json:"clobTokenIds"`
	Events          []liveGammaEvent `json:"events"`
}

type liveGammaEvent struct {
	Slug string `json:"slug"`
}

func TestIntegrationCreatePostAndCancelOrder(t *testing.T) {
	client := newLiveAuthedClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	config := loadLiveRestingLimitOrderConfig(t, ctx, client)
	client = prepareLivePostOrderClient(t, ctx, client, config)

	price := safeLiveRestingBuyPrice(t, config)
	params := CreatePostOrderRequestParams{
		CreateOrderParams: CreateOrderParams{
			TokenID:  requireLivePostOrderTokenID(t, config, "TestIntegrationCreatePostAndCancelOrder"),
			Price:    price,
			Size:     defaultLiveLimitSize(config),
			Side:     SideBuy,
			TickSize: requireLivePostOrderTickSize(t, config, "TestIntegrationCreatePostAndCancelOrder"),
		},
		OrderType: OrderTypeGTC,
		PostOnly:  true,
	}
	params.NegRisk = config.NegRisk

	var postedOrderID string
	canceled := false
	defer func() {
		if postedOrderID == "" || canceled {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := client.CancelOrder(cleanupCtx, postedOrderID); err != nil {
			t.Logf("cleanup CancelOrder(%s) error: %v", postedOrderID, err)
		}
	}()

	t.Logf("creating and posting order with params: %+v", params)

	response, err := client.CreateAndPostOrder(ctx, params)
	if err != nil {
		t.Fatalf("CreateAndPostOrder() error = %v", err)
	}
	if response == nil {
		t.Fatal("CreateAndPostOrder() returned nil response")
	}
	t.Logf("CreateAndPostOrder() response: %+v", response)
	if !response.Success {
		t.Fatalf("CreateAndPostOrder() failed: %+v", response)
	}
	postedOrderID = strings.TrimSpace(response.OrderID)
	if postedOrderID == "" {
		t.Fatalf("CreateAndPostOrder() response missing orderID: %+v", response)
	}

	cancelResponse, err := client.CancelOrder(ctx, postedOrderID)
	if err != nil {
		t.Fatalf("CancelOrder(%s) error = %v", postedOrderID, err)
	}
	if cancelResponse == nil {
		t.Fatal("CancelOrder() returned nil response")
	}
	if !containsString(cancelResponse.Canceled, postedOrderID) {
		t.Fatalf("canceled orders = %#v, want %q to be present", cancelResponse.Canceled, postedOrderID)
	}
	canceled = true

	fmt.Printf(
		"TestIntegrationCreatePostAndCancelOrder: order_id=%s price=%s order_type=%s post_only=%t canceled=%v not_canceled=%s\n",
		postedOrderID,
		price,
		OrderTypeGTC,
		params.PostOnly,
		cancelResponse.Canceled,
		strings.TrimSpace(string(cancelResponse.NotCanceled)),
	)
}

func TestIntegrationCreatePostMarketBuyThenSellOrder(t *testing.T) {
	client := newLiveAuthedClient(t)

	setupCtx, setupCancel := context.WithTimeout(context.Background(), 60*time.Second)
	buyConfig := loadLiveMarketRoundTripConfig(t, setupCtx, client)
	client = prepareLivePostOrderClient(t, setupCtx, client, buyConfig)
	setupCancel()

	buyCtx, buyCancel := context.WithTimeout(context.Background(), 45*time.Second)
	buyResponse := postLiveMarketOrder(t, buyCtx, client, "market BUY", buyConfig, SideBuy, buyConfig.MarketAmount, buyConfig.MarketPrice)
	buyCancel()
	sellAmount, err := filledSharesFromBuyResponse(buyResponse)
	if err != nil {
		t.Fatalf("resolve bought shares from BUY response: %v", err)
	}
	if sellAmount == "0" {
		t.Fatalf("BUY response produced zero sell amount: %+v", buyResponse)
	}

	// Give live balances a short window to reflect the just-filled market BUY.
	time.Sleep(1 * time.Second)

	sellConfig := buyConfig
	sellConfig.Side = SideSell
	sellConfig.MarketAmount = sellAmount
	sellConfig.MarketPrice = liveMarketEnvFirst(SideSell, "PRICE")
	sellConfig.MarketOrderType = liveMarketOrderTypeForSide(SideSell)

	sellCtx, sellCancel := context.WithTimeout(context.Background(), 45*time.Second)
	sellResponse := postLiveMarketOrder(t, sellCtx, client, "market SELL", sellConfig, SideSell, sellAmount, sellConfig.MarketPrice)
	sellCancel()

	fmt.Printf(
		"TestIntegrationCreatePostMarketBuyThenSellOrder: token=%s buy_order_id=%s buy_usdc=%s bought_shares=%s sell_order_id=%s sell_amount=%s sell_status=%s\n",
		buyConfig.TokenID,
		buyResponse.OrderID,
		buyConfig.MarketAmount,
		sellAmount,
		sellResponse.OrderID,
		sellAmount,
		sellResponse.Status,
	)
}

func postLiveMarketOrder(
	t *testing.T,
	ctx context.Context,
	client *Client,
	label string,
	config livePostOrderConfig,
	side Side,
	amount string,
	price string,
) *PostOrderResponse {
	t.Helper()

	params := CreatePostMarketOrderRequestParams{
		CreateMarketOrderParams: CreateMarketOrderParams{
			TokenID:   requireLivePostOrderTokenID(t, config, label),
			Amount:    strings.TrimSpace(amount),
			Price:     strings.TrimSpace(price),
			Side:      side,
			TickSize:  requireLivePostOrderTickSize(t, config, label),
			OrderType: config.MarketOrderType,
		},
	}
	params.NegRisk = config.NegRisk

	response, err := client.CreateAndPostMarketOrder(ctx, params)
	if err != nil {
		t.Fatalf("%s CreateAndPostMarketOrder() error = %v", label, err)
	}
	if response == nil {
		t.Fatalf("%s CreateAndPostMarketOrder() returned nil response", label)
	}
	if !response.Success {
		t.Fatalf("%s CreateAndPostMarketOrder() failed: %+v", label, response)
	}
	if strings.TrimSpace(response.OrderID) == "" {
		t.Fatalf("%s response missing orderID: %+v", label, response)
	}
	return response
}

func loadLiveRestingLimitOrderConfig(t *testing.T, ctx context.Context, client *Client) livePostOrderConfig {
	t.Helper()

	config, err := discoverLiveRestingLimitOrderConfig(ctx, client)
	if err != nil {
		t.Fatalf("discover live resting limit order config: %v", err)
	}
	t.Logf(
		"using live resting limit-order market: slug=%q question=%q token=%s tick=%s neg_risk=%t limit_price=%s size=%s",
		config.Slug,
		config.Question,
		config.TokenID,
		config.TickSize,
		config.NegRisk,
		config.LimitPrice,
		config.LimitSize,
	)
	return config
}

func loadLiveMarketRoundTripConfig(t *testing.T, ctx context.Context, client *Client) livePostOrderConfig {
	t.Helper()

	config, err := discoverLiveMarketRoundTripConfig(ctx, client)
	if err != nil {
		t.Fatalf("discover live market roundtrip config: %v", err)
	}
	t.Logf(
		"using live market roundtrip market: slug=%q question=%q token=%s buy_amount=%s tick=%s neg_risk=%t buy_protection_price=%q buy_order_type=%s",
		config.Slug,
		config.Question,
		config.TokenID,
		config.MarketAmount,
		config.TickSize,
		config.NegRisk,
		config.MarketPrice,
		config.MarketOrderType,
	)
	return config
}

func discoverLiveRestingLimitOrderConfig(ctx context.Context, client *Client) (livePostOrderConfig, error) {
	if config, ok, err := liveLimitOrderConfigFromEnv(ctx, client); ok || err != nil {
		return config, err
	}

	return discoverLiveConfigFromGamma(ctx, client, func(market liveGammaMarket, tokenID string, book *OrderBook) (livePostOrderConfig, bool) {
		config, ok := liveLimitOrderConfigFromBook(book)
		if !ok {
			return livePostOrderConfig{}, false
		}
		config.Question = strings.TrimSpace(market.Question)
		config.Slug = strings.TrimSpace(market.Slug)
		config.TokenID = strings.TrimSpace(tokenID)
		return config, true
	})
}

func discoverLiveMarketRoundTripConfig(ctx context.Context, client *Client) (livePostOrderConfig, error) {
	if config, ok, err := liveMarketRoundTripConfigFromEnv(ctx, client); ok || err != nil {
		return config, err
	}

	return discoverLiveConfigFromGamma(ctx, client, func(market liveGammaMarket, tokenID string, book *OrderBook) (livePostOrderConfig, bool) {
		config, ok := liveMarketRoundTripConfigFromBook(book)
		if !ok {
			return livePostOrderConfig{}, false
		}
		config.Question = strings.TrimSpace(market.Question)
		config.Slug = strings.TrimSpace(market.Slug)
		config.TokenID = strings.TrimSpace(tokenID)
		return config, true
	})
}

func discoverLiveConfigFromGamma(
	ctx context.Context,
	client *Client,
	build func(market liveGammaMarket, tokenID string, book *OrderBook) (livePostOrderConfig, bool),
) (livePostOrderConfig, error) {
	if client == nil {
		return livePostOrderConfig{}, fmt.Errorf("client is nil")
	}

	baseURL, err := url.Parse(liveGammaMarketsHost)
	if err != nil {
		return livePostOrderConfig{}, fmt.Errorf("parse gamma host: %w", err)
	}

	var markets []liveGammaMarket
	query := url.Values{
		"active":    []string{"true"},
		"closed":    []string{"false"},
		"limit":     []string{"200"},
		"order":     []string{"volume"},
		"ascending": []string{"false"},
	}
	if err := client.doJSONBase(ctx, baseURL, http.MethodGet, "/markets", query, nil, nil, &markets); err != nil {
		return livePostOrderConfig{}, fmt.Errorf("load gamma markets: %w", err)
	}

	var fallback *livePostOrderConfig
	for _, market := range markets {
		if !market.EnableOrderBook || !market.Active || market.Closed {
			continue
		}
		matchesTarget := livePostOrderMarketMatchesTarget(market)

		tokenIDs, err := market.tokenIDs()
		if err != nil {
			continue
		}
		for _, tokenID := range tokenIDs {
			book, err := client.GetOrderBook(ctx, tokenID)
			if err != nil || book == nil {
				continue
			}

			config, ok := build(market, tokenID, book)
			if !ok {
				continue
			}
			if matchesTarget {
				return config, nil
			}
			if fallback == nil {
				copy := config
				fallback = &copy
			}
		}
	}
	if fallback != nil {
		return *fallback, nil
	}

	return livePostOrderConfig{}, fmt.Errorf(
		"target market family %q not found and no fallback market had usable order book liquidity (accepted prefixes: %s)",
		livePreferredPostMarketLabel,
		strings.Join(livePreferredPostMarketSlugPrefixes, ", "),
	)
}

func liveLimitOrderConfigFromEnv(ctx context.Context, client *Client) (livePostOrderConfig, bool, error) {
	tokenID := liveEnvFirst("POLYMARKET_LIMIT_POST_TOKEN_ID", "POLYMARKET_POST_TOKEN_ID")
	if tokenID == "" {
		return livePostOrderConfig{}, false, nil
	}

	book, err := client.GetOrderBook(ctx, tokenID)
	if err != nil {
		return livePostOrderConfig{}, true, fmt.Errorf("load order book for configured post token %s: %w", tokenID, err)
	}
	config, ok := liveLimitOrderConfigFromBook(book)
	if !ok {
		return livePostOrderConfig{}, true, fmt.Errorf("configured post token %s has unsupported order book", tokenID)
	}
	config.Question = "configured post token"
	config.Slug = "configured-post-token"
	config.TokenID = strings.TrimSpace(tokenID)
	return config, true, nil
}

func liveLimitOrderConfigFromBook(book *OrderBook) (livePostOrderConfig, bool) {
	if book == nil {
		return livePostOrderConfig{}, false
	}

	tickSize := TickSize(strings.TrimSpace(liveEnvFirst("POLYMARKET_LIMIT_POST_TICK_SIZE", "POLYMARKET_POST_TICK_SIZE")))
	if tickSize == "" {
		tickSize = TickSize(strings.TrimSpace(string(book.TickSize)))
	}
	if _, err := roundingConfigForTick(tickSize); err != nil {
		return livePostOrderConfig{}, false
	}

	size := liveEnvFirst("POLYMARKET_LIMIT_POST_SIZE", "POLYMARKET_POST_SIZE")
	if size == "" {
		size = strings.TrimSpace(string(book.MinOrderSize))
	}
	if size == "" {
		size = liveDefaultPostAmount
	}

	limitPrice := liveEnvFirst("POLYMARKET_LIMIT_POST_PRICE", "POLYMARKET_POST_PRICE")
	if limitPrice == "" {
		limitPrice = strings.TrimSpace(string(tickSize))
	}

	negRisk, ok := liveNegRiskFromEnv("POLYMARKET_LIMIT_POST_NEG_RISK", "POLYMARKET_POST_NEG_RISK")
	if !ok {
		negRisk = book.NegRisk
	}

	return livePostOrderConfig{
		Side:       SideBuy,
		TickSize:   tickSize,
		NegRisk:    negRisk,
		LimitSize:  size,
		LimitPrice: limitPrice,
	}, true
}

func liveMarketRoundTripConfigFromEnv(ctx context.Context, client *Client) (livePostOrderConfig, bool, error) {
	tokenID := liveMarketEnvFirst(SideBuy, "TOKEN_ID")
	if tokenID == "" {
		return livePostOrderConfig{}, false, nil
	}

	book, err := client.GetOrderBook(ctx, tokenID)
	if err != nil {
		return livePostOrderConfig{}, true, fmt.Errorf("load order book for configured market token %s: %w", tokenID, err)
	}
	config, ok := liveMarketRoundTripConfigFromBook(book)
	if !ok {
		return livePostOrderConfig{}, true, fmt.Errorf("configured market token %s cannot support BUY then SELL roundtrip", tokenID)
	}
	config.Question = "configured market token"
	config.Slug = "configured-market-token"
	config.TokenID = strings.TrimSpace(tokenID)
	return config, true, nil
}

func liveMarketRoundTripConfigFromBook(book *OrderBook) (livePostOrderConfig, bool) {
	if book == nil {
		return livePostOrderConfig{}, false
	}

	tickSize := TickSize(strings.TrimSpace(liveMarketEnvFirst(SideBuy, "TICK_SIZE")))
	if tickSize == "" {
		tickSize = TickSize(strings.TrimSpace(string(book.TickSize)))
	}
	if _, err := roundingConfigForTick(tickSize); err != nil {
		return livePostOrderConfig{}, false
	}

	buyAmount := liveMarketAmountForBook(book)
	buyOrderType := liveMarketOrderTypeForSide(SideBuy)
	buyPriceOverride := liveMarketEnvFirst(SideBuy, "PRICE")
	if len(book.Asks) == 0 || len(book.Bids) == 0 {
		return livePostOrderConfig{}, false
	}

	effectiveBuyPrice := strings.TrimSpace(buyPriceOverride)
	if effectiveBuyPrice == "" {
		calculated, err := calculateMarketPriceFromLevels(book.Asks, SideBuy, buyAmount, buyOrderType)
		if err != nil {
			return livePostOrderConfig{}, false
		}
		effectiveBuyPrice = calculated.String()
	}

	estimatedShares, err := estimateBuySharesForAmount(buyAmount, effectiveBuyPrice)
	if err != nil {
		return livePostOrderConfig{}, false
	}
	if _, err := calculateMarketPriceFromLevels(book.Bids, SideSell, estimatedShares, OrderTypeFOK); err != nil {
		return livePostOrderConfig{}, false
	}

	negRisk, ok := liveNegRiskFromEnv("POLYMARKET_MARKET_BUY_NEG_RISK", "POLYMARKET_MARKET_NEG_RISK")
	if !ok {
		negRisk = book.NegRisk
	}

	return livePostOrderConfig{
		Side:            SideBuy,
		TickSize:        tickSize,
		NegRisk:         negRisk,
		MarketAmount:    strings.TrimSpace(buyAmount),
		MarketPrice:     strings.TrimSpace(buyPriceOverride),
		MarketOrderType: buyOrderType,
	}, true
}

func liveMarketAmountForBook(book *OrderBook) string {
	if amount := liveMarketEnvFirst(SideBuy, "AMOUNT"); amount != "" {
		return amount
	}
	if book != nil {
		if amount := strings.TrimSpace(string(book.MinOrderSize)); amount != "" {
			return amount
		}
	}
	return liveDefaultPostAmount
}

func liveMarketOrderTypeForSide(side Side) OrderType {
	raw := strings.ToUpper(liveMarketEnvFirst(side, "ORDER_TYPE"))
	if raw == "" {
		return OrderTypeFOK
	}
	switch OrderType(raw) {
	case OrderTypeFOK, OrderTypeFAK:
		return OrderType(raw)
	default:
		return OrderTypeFOK
	}
}

func liveMarketEnvFirst(side Side, suffix string) string {
	sideName := strings.ToUpper(strings.TrimSpace(string(side)))
	keys := []string{
		"POLYMARKET_MARKET_" + sideName + "_" + suffix,
		"POLYMARKET_MARKET_" + suffix,
	}
	if suffix == "TOKEN_ID" {
		keys = append(keys, "POLYMARKET_POST_TOKEN_ID")
	}
	return liveEnvFirst(keys...)
}

func liveNegRiskFromEnv(keys ...string) (bool, bool) {
	raw := liveEnvFirst(keys...)
	if raw == "" {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

func estimateBuySharesForAmount(amount string, price string) (string, error) {
	amountRat, err := parsePositiveDecimal(amount, "buy amount")
	if err != nil {
		return "", err
	}
	priceRat, err := parsePositiveDecimal(price, "buy price")
	if err != nil {
		return "", err
	}
	return ratToDecimalString(new(big.Rat).Quo(amountRat, priceRat)), nil
}

func filledSharesFromBuyResponse(response *PostOrderResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("buy response is nil")
	}
	raw := strings.TrimSpace(response.TakingAmount.String())
	if raw == "" {
		return "", fmt.Errorf("buy response missing TakingAmount: %+v", response)
	}

	shares, err := parseNonNegativeDecimal(raw, "TakingAmount")
	if err != nil {
		return "", err
	}
	if shares.Sign() == 0 {
		return "0", nil
	}
	if strings.Contains(raw, ".") {
		return ratToDecimalString(shares), nil
	}

	// Older/alternate response shapes may return token units as an integer.
	return ratToDecimalString(new(big.Rat).Quo(shares, new(big.Rat).SetInt(tenPow(collateralTokenDecimals)))), nil
}

func livePostOrderMarketMatchesTarget(market liveGammaMarket) bool {
	if len(livePreferredPostMarketSlugPrefixes) == 0 {
		return true
	}

	for _, rawPrefix := range livePreferredPostMarketSlugPrefixes {
		prefix := strings.ToLower(strings.TrimSpace(rawPrefix))
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(market.Slug)), prefix) {
			return true
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(market.EventSlug)), prefix) {
			return true
		}
		for _, event := range market.Events {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.Slug)), prefix) {
				return true
			}
		}
	}
	return false
}

func (m liveGammaMarket) tokenIDs() ([]string, error) {
	var values []string
	if err := json.Unmarshal(m.ClobTokenIDs, &values); err == nil {
		return trimNonEmptyStrings(values), nil
	}

	var encoded string
	if err := json.Unmarshal(m.ClobTokenIDs, &encoded); err == nil {
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			return nil, nil
		}
		if err := json.Unmarshal([]byte(encoded), &values); err == nil {
			return trimNonEmptyStrings(values), nil
		}
		return []string{encoded}, nil
	}

	return nil, fmt.Errorf("unsupported clobTokenIds payload")
}

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func defaultLiveLimitSize(config livePostOrderConfig) string {
	if value := strings.TrimSpace(config.LimitSize); value != "" {
		return value
	}
	return liveDefaultPostAmount
}

func requireLivePostOrderTokenID(t *testing.T, config livePostOrderConfig, label string) string {
	t.Helper()

	if tokenID := strings.TrimSpace(config.TokenID); tokenID != "" {
		return tokenID
	}
	t.Fatalf("%s: live post-order auto-discovery did not provide tokenID", label)
	return ""
}

func requireLivePostOrderTickSize(t *testing.T, config livePostOrderConfig, label string) TickSize {
	t.Helper()

	if tickSize := TickSize(strings.TrimSpace(string(config.TickSize))); tickSize != "" {
		return tickSize
	}
	t.Fatalf("%s: live post-order auto-discovery did not provide tick size", label)
	return ""
}

func requireLiveLimitPrice(t *testing.T, config livePostOrderConfig, label string) string {
	t.Helper()

	if price := strings.TrimSpace(config.LimitPrice); price != "" {
		return price
	}
	t.Fatalf("%s: live post-order auto-discovery did not provide limit price", label)
	return ""
}

func safeLiveRestingBuyPrice(t *testing.T, config livePostOrderConfig) string {
	t.Helper()

	price := requireLiveLimitPrice(t, config, "safeLiveRestingBuyPrice")
	limitPrice, err := parsePositiveDecimal(price, "limit price")
	if err != nil {
		t.Fatalf("parse limit price: %v", err)
	}

	tickSize := string(requireLivePostOrderTickSize(t, config, "safeLiveRestingBuyPrice"))
	tick, err := parsePositiveDecimal(tickSize, "tickSize")
	if err != nil {
		t.Fatalf("parse tick size: %v", err)
	}
	if limitPrice.Cmp(tick) < 0 {
		t.Fatalf("limit price %s is below tick size %s", price, tickSize)
	}

	return price
}

func prepareLivePostOrderClient(t *testing.T, ctx context.Context, client *Client, config livePostOrderConfig) *Client {
	t.Helper()

	if client == nil {
		t.Fatal("live post-order client is nil")
	}
	if client.SignatureType() == SignatureTypeEOA || strings.TrimSpace(client.FunderAddress()) != "" {
		return client
	}

	cloned, discovery, err := client.WithDiscoveredFunder(ctx, FunderDiscoveryParams{
		AssetID: strings.TrimSpace(config.TokenID),
	})
	if err == nil {
		if discovery != nil && discovery.Preferred != nil {
			t.Logf("using discovered funder=%s source=%s", discovery.Preferred.Address, discovery.Preferred.Source)
		}
		return cloned
	}

	if discovery != nil && discovery.Preferred == nil {
		t.Skipf(
			"skip post-order smoke: signature_type=%d requires funder address, but auto-discovery found %d candidates; set POLYMARKET_FUNDER_ADDRESS to force a specific funder",
			client.SignatureType(),
			len(discovery.Candidates),
		)
	}

	t.Fatalf("prepare live post-order client: %v", err)
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
