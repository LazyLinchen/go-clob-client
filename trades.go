package clobclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const endpointTrades = "/data/trades"

// TradesParams 描述成交历史查询的过滤条件。
type TradesParams struct {
	// ID 按单条成交 ID 过滤。
	ID string
	// MakerAddress 按 maker 地址过滤。
	MakerAddress string
	// Market 按市场 condition ID 过滤。
	Market string
	// AssetID 按 token ID 过滤。
	AssetID string
	// Before 仅返回该 Unix 时间戳之前的成交。
	Before string
	// After 仅返回该 Unix 时间戳之后的成交。
	After string
	// NextCursor 用于分页拉取后续页。
	NextCursor string
}

// MakerOrder 表示一笔成交中匹配到的单个 maker 订单。
type MakerOrder struct {
	OrderID       string       `json:"order_id"`
	MakerAddress  string       `json:"maker_address"`
	Owner         string       `json:"owner"`
	MatchedAmount NumberString `json:"matched_amount"`
	Price         NumberString `json:"price"`
	FeeRateBps    NumberString `json:"fee_rate_bps"`
	AssetID       string       `json:"asset_id"`
	Outcome       string       `json:"outcome"`
	// Side 表示该 maker 订单的买卖方向，V2 新增字段。
	Side string `json:"side"`
	// BuilderFee 与 BuilderCode 仅在 builder order 成交时返回。
	BuilderFee  NumberString `json:"builder_fee"`
	BuilderCode string       `json:"builder_code"`
}

// Trade 表示成交历史接口返回的一条成交记录。
type Trade struct {
	ID              string       `json:"id"`
	TakerOrderID    string       `json:"taker_order_id"`
	Market          string       `json:"market"`
	AssetID         string       `json:"asset_id"`
	Side            string       `json:"side"`
	Size            NumberString `json:"size"`
	FeeRateBps      NumberString `json:"fee_rate_bps"`
	Price           NumberString `json:"price"`
	Status          string       `json:"status"`
	MatchTime       string       `json:"match_time"`
	MatchTimeNano   string       `json:"match_time_nano"`
	LastUpdate      string       `json:"last_update"`
	ErrorMessage    string       `json:"err_msg"`
	Outcome         string       `json:"outcome"`
	BucketIndex     NumberString `json:"bucket_index"`
	Owner           string       `json:"owner"`
	MakerAddress    string       `json:"maker_address"`
	TransactionHash string       `json:"transaction_hash"`
	// TraderSide 表示当前鉴权用户在该成交中处于 taker 还是 maker。
	TraderSide  string       `json:"trader_side"`
	MakerOrders []MakerOrder `json:"maker_orders"`
	CreatedAt   NumberString `json:"created_at"`
}

// TradesResponse 表示成交历史接口的分页响应；同时兼容裸数组和分页信封两种返回形态。
type TradesResponse struct {
	Limit      int64   `json:"limit"`
	Count      int64   `json:"count"`
	NextCursor string  `json:"next_cursor"`
	Data       []Trade `json:"data"`
}

// UnmarshalJSON 兼容服务端可能直接返回成交数组、也可能返回 {data,...} 信封两种格式。
func (r *TradesResponse) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, &r.Data)
	}

	type alias TradesResponse
	var wire alias
	if err := json.Unmarshal(trimmed, &wire); err != nil {
		return err
	}
	*r = TradesResponse(wire)
	return nil
}

// GetTrades 查询当前 L2 凭证对应用户的成交历史。
func (c *Client) GetTrades(ctx context.Context, params TradesParams) (*TradesResponse, error) {
	query := make(url.Values)
	if value := strings.TrimSpace(params.ID); value != "" {
		query.Set("id", value)
	}
	if value := strings.TrimSpace(params.MakerAddress); value != "" {
		query.Set("maker_address", value)
	}
	if value := strings.TrimSpace(params.Market); value != "" {
		query.Set("market", value)
	}
	if value := strings.TrimSpace(params.AssetID); value != "" {
		query.Set("asset_id", value)
	}
	if value := strings.TrimSpace(params.Before); value != "" {
		query.Set("before", value)
	}
	if value := strings.TrimSpace(params.After); value != "" {
		query.Set("after", value)
	}
	if value := strings.TrimSpace(params.NextCursor); value != "" {
		query.Set("next_cursor", value)
	}

	var response TradesResponse
	if err := c.doL2JSON(ctx, http.MethodGet, endpointTrades, query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
