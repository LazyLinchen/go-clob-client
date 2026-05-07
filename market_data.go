package clobclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetOK 调用健康检查接口并返回原始响应体。
func (c *Client) GetOK(ctx context.Context) ([]byte, error) {
	return c.doBytes(ctx, http.MethodGet, "/ok", nil, nil)
}

// GetServerTime 获取服务端当前 Unix 时间戳。
func (c *Client) GetServerTime(ctx context.Context) (int64, error) {
	var ts int64
	if err := c.doJSON(ctx, http.MethodGet, "/time", nil, nil, nil, &ts); err != nil {
		return 0, err
	}
	return ts, nil
}

// GetOrderBook 获取指定 token 的盘口深度。
func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (*OrderBook, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var book OrderBook
	if err := c.doJSON(ctx, http.MethodGet, "/book", url.Values{"token_id": []string{tokenID}}, nil, nil, &book); err != nil {
		return nil, err
	}
	return &book, nil
}

// GetPrice 获取指定 token 在给定方向上的价格。
func (c *Client) GetPrice(ctx context.Context, tokenID string, side Side) (*PriceResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	normalizedSide, err := normalizeSide(side)
	if err != nil {
		return nil, err
	}

	var response PriceResponse
	query := url.Values{
		"token_id": []string{tokenID},
		"side":     []string{string(normalizedSide)},
	}
	if err := c.doJSON(ctx, http.MethodGet, "/price", query, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetMidpoint 获取指定 token 的中间价。
func (c *Client) GetMidpoint(ctx context.Context, tokenID string) (*MidpointResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var response MidpointResponse
	if err := c.doJSON(ctx, http.MethodGet, "/midpoint", url.Values{"token_id": []string{tokenID}}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetSpread 获取指定 token 的买卖价差。
func (c *Client) GetSpread(ctx context.Context, tokenID string) (*SpreadResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var response SpreadResponse
	if err := c.doJSON(ctx, http.MethodGet, "/spread", url.Values{"token_id": []string{tokenID}}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetTickSize 获取指定 token 的最小跳动单位。
func (c *Client) GetTickSize(ctx context.Context, tokenID string) (*TickSizeResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var response TickSizeResponse
	if err := c.doJSON(ctx, http.MethodGet, "/tick-size", url.Values{"token_id": []string{tokenID}}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetNegRisk 获取指定 token 是否属于 neg-risk 市场。
func (c *Client) GetNegRisk(ctx context.Context, tokenID string) (*NegRiskResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var response NegRiskResponse
	if err := c.doJSON(ctx, http.MethodGet, "/neg-risk", url.Values{"token_id": []string{tokenID}}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetMarketByToken 根据 token ID 查询其所属市场元数据。
func (c *Client) GetMarketByToken(ctx context.Context, tokenID string) (*MarketByTokenResponse, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, fmt.Errorf("tokenID is required")
	}

	var response MarketByTokenResponse
	endpoint := "/markets-by-token/" + url.PathEscape(tokenID)
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetCLOBMarketInfo 获取指定条件市场的扩展市场信息。
func (c *Client) GetCLOBMarketInfo(ctx context.Context, conditionID string) (*CLOBMarketInfo, error) {
	conditionID = strings.TrimSpace(conditionID)
	if conditionID == "" {
		return nil, fmt.Errorf("conditionID is required")
	}

	var response CLOBMarketInfo
	endpoint := "/clob-markets/" + url.PathEscape(conditionID)
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// normalizeSide 将买卖方向标准化为服务端接受的枚举值。
func normalizeSide(side Side) (Side, error) {
	switch normalized := Side(strings.ToUpper(strings.TrimSpace(string(side)))); normalized {
	case SideBuy:
		return SideBuy, nil
	case SideSell:
		return SideSell, nil
	default:
		return "", fmt.Errorf("invalid side %q", side)
	}
}
