package clobclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	endpointMarkets                   = "/markets"
	endpointSamplingMarkets           = "/sampling-markets"
	endpointSimplifiedMarkets         = "/simplified-markets"
	endpointSamplingSimplifiedMarkets = "/sampling-simplified-markets"
	endpointMarket                    = "/markets/"

	// MarketsEndCursor 是分页游标到达末页时服务端返回的哨兵值。
	MarketsEndCursor = "LTE="
)

// MarketToken 表示市场列表里的单个 outcome token。
type MarketToken struct {
	TokenID string       `json:"token_id"`
	Outcome string       `json:"outcome"`
	Price   NumberString `json:"price"`
	Winner  bool         `json:"winner"`
}

// RewardsRate 表示某个资产的每日激励发放速率。
type RewardsRate struct {
	AssetAddress     string       `json:"asset_address"`
	RewardsDailyRate NumberString `json:"rewards_daily_rate"`
}

// MarketRewards 表示市场对象内嵌的激励配置。
type MarketRewards struct {
	Rates     []RewardsRate `json:"rates"`
	MinSize   NumberString  `json:"min_size"`
	MaxSpread NumberString  `json:"max_spread"`
}

// SimplifiedMarket 表示精简市场表示，用于 simplified / sampling-simplified 列表。
type SimplifiedMarket struct {
	ConditionID     string        `json:"condition_id"`
	Tokens          []MarketToken `json:"tokens"`
	Rewards         MarketRewards `json:"rewards"`
	Active          bool          `json:"active"`
	Closed          bool          `json:"closed"`
	Archived        bool          `json:"archived"`
	AcceptingOrders bool          `json:"accepting_orders"`
}

// Market 表示完整市场对象，用于 markets / sampling-markets 列表及单市场查询。
type Market struct {
	ConditionID             string        `json:"condition_id"`
	QuestionID              string        `json:"question_id"`
	Question                string        `json:"question"`
	Description             string        `json:"description"`
	MarketSlug              string        `json:"market_slug"`
	Tokens                  []MarketToken `json:"tokens"`
	Rewards                 MarketRewards `json:"rewards"`
	MinimumOrderSize        NumberString  `json:"minimum_order_size"`
	MinimumTickSize         NumberString  `json:"minimum_tick_size"`
	EndDateISO              string        `json:"end_date_iso"`
	GameStartTime           string        `json:"game_start_time"`
	SecondsDelay            int64         `json:"seconds_delay"`
	FPMM                    string        `json:"fpmm"`
	MakerBaseFee            int64         `json:"maker_base_fee"`
	TakerBaseFee            int64         `json:"taker_base_fee"`
	NotificationsEnabled    bool          `json:"notifications_enabled"`
	NegRisk                 bool          `json:"neg_risk"`
	NegRiskMarketID         string        `json:"neg_risk_market_id"`
	NegRiskRequestID        string        `json:"neg_risk_request_id"`
	Icon                    string        `json:"icon"`
	Image                   string        `json:"image"`
	EnableOrderBook         bool          `json:"enable_order_book"`
	Active                  bool          `json:"active"`
	Closed                  bool          `json:"closed"`
	Archived                bool          `json:"archived"`
	AcceptingOrders         bool          `json:"accepting_orders"`
	AcceptingOrderTimestamp string        `json:"accepting_order_timestamp"`
	Is5050Outcome           bool          `json:"is_50_50_outcome"`
	Tags                    []string      `json:"tags"`
}

// MarketsResponse 表示完整市场列表接口的分页响应。
type MarketsResponse struct {
	Limit      int64    `json:"limit"`
	Count      int64    `json:"count"`
	NextCursor string   `json:"next_cursor"`
	Data       []Market `json:"data"`
}

// SimplifiedMarketsResponse 表示精简市场列表接口的分页响应。
type SimplifiedMarketsResponse struct {
	Limit      int64              `json:"limit"`
	Count      int64              `json:"count"`
	NextCursor string             `json:"next_cursor"`
	Data       []SimplifiedMarket `json:"data"`
}

// GetMarkets 按游标分页获取完整市场列表。
func (c *Client) GetMarkets(ctx context.Context, nextCursor string) (*MarketsResponse, error) {
	var response MarketsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointMarkets, cursorQuery(nextCursor), nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetSamplingMarkets 按游标分页获取开启激励的完整市场列表。
func (c *Client) GetSamplingMarkets(ctx context.Context, nextCursor string) (*MarketsResponse, error) {
	var response MarketsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointSamplingMarkets, cursorQuery(nextCursor), nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetSimplifiedMarkets 按游标分页获取精简市场列表。
func (c *Client) GetSimplifiedMarkets(ctx context.Context, nextCursor string) (*SimplifiedMarketsResponse, error) {
	var response SimplifiedMarketsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointSimplifiedMarkets, cursorQuery(nextCursor), nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetSamplingSimplifiedMarkets 按游标分页获取开启激励的精简市场列表。
func (c *Client) GetSamplingSimplifiedMarkets(ctx context.Context, nextCursor string) (*SimplifiedMarketsResponse, error) {
	var response SimplifiedMarketsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointSamplingSimplifiedMarkets, cursorQuery(nextCursor), nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetMarket 按 condition ID 查询单个完整市场对象。
func (c *Client) GetMarket(ctx context.Context, conditionID string) (*Market, error) {
	conditionID = strings.TrimSpace(conditionID)
	if conditionID == "" {
		return nil, fmt.Errorf("conditionID is required")
	}

	var response Market
	endpoint := endpointMarket + url.PathEscape(conditionID)
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// cursorQuery 仅在游标非空时构造 next_cursor 查询参数。
func cursorQuery(nextCursor string) url.Values {
	nextCursor = strings.TrimSpace(nextCursor)
	if nextCursor == "" {
		return nil
	}
	return url.Values{"next_cursor": []string{nextCursor}}
}
