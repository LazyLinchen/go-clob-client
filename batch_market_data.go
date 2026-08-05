package clobclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

const (
	endpointBooks     = "/books"
	endpointPrices    = "/prices"
	endpointMidpoints = "/midpoints"
	endpointSpreads   = "/spreads"
)

// BookParams 标识一次批量行情查询里的单个 token（部分接口附带方向）。
type BookParams struct {
	TokenID string `json:"token_id"`
	Side    Side   `json:"side,omitempty"`
}

// GetOrderBooks 批量获取多个 token 的盘口快照。
func (c *Client) GetOrderBooks(ctx context.Context, tokenIDs []string) ([]OrderBook, error) {
	params, err := bookParamsFromTokenIDs(tokenIDs)
	if err != nil {
		return nil, err
	}

	var response []OrderBook
	if err := c.doJSON(ctx, http.MethodPost, endpointBooks, nil, params, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// GetPrices 批量获取多个 token 在指定方向上的价格，返回 token_id -> side -> price 的映射。
func (c *Client) GetPrices(ctx context.Context, params []BookParams) (map[string]map[string]NumberString, error) {
	normalized, err := normalizeBookParams(params, true)
	if err != nil {
		return nil, err
	}

	var response map[string]map[string]NumberString
	if err := c.doJSON(ctx, http.MethodPost, endpointPrices, nil, normalized, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// GetMidpoints 批量获取多个 token 的中间价，返回 token_id -> midpoint 的映射。
func (c *Client) GetMidpoints(ctx context.Context, tokenIDs []string) (map[string]NumberString, error) {
	params, err := bookParamsFromTokenIDs(tokenIDs)
	if err != nil {
		return nil, err
	}

	var response map[string]NumberString
	if err := c.doJSON(ctx, http.MethodPost, endpointMidpoints, nil, params, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// GetSpreads 批量获取多个 token 的买卖价差，返回 token_id -> spread 的映射。
func (c *Client) GetSpreads(ctx context.Context, tokenIDs []string) (map[string]NumberString, error) {
	params, err := bookParamsFromTokenIDs(tokenIDs)
	if err != nil {
		return nil, err
	}

	var response map[string]NumberString
	if err := c.doJSON(ctx, http.MethodPost, endpointSpreads, nil, params, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// bookParamsFromTokenIDs 将 token ID 列表整理为去重、非空的 BookParams 列表。
func bookParamsFromTokenIDs(tokenIDs []string) ([]BookParams, error) {
	if len(tokenIDs) == 0 {
		return nil, errors.New("at least one tokenID is required")
	}

	seen := make(map[string]struct{}, len(tokenIDs))
	params := make([]BookParams, 0, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		tokenID = strings.TrimSpace(tokenID)
		if tokenID == "" {
			continue
		}
		if _, ok := seen[tokenID]; ok {
			continue
		}
		seen[tokenID] = struct{}{}
		params = append(params, BookParams{TokenID: tokenID})
	}

	if len(params) == 0 {
		return nil, errors.New("at least one tokenID is required")
	}
	return params, nil
}

// normalizeBookParams 校验批量行情参数；requireSide 为 true 时要求每项带合法方向（如 /prices）。
func normalizeBookParams(params []BookParams, requireSide bool) ([]BookParams, error) {
	if len(params) == 0 {
		return nil, errors.New("at least one book param is required")
	}

	normalized := make([]BookParams, 0, len(params))
	for _, param := range params {
		tokenID := strings.TrimSpace(param.TokenID)
		if tokenID == "" {
			return nil, errors.New("tokenID is required for every book param")
		}

		entry := BookParams{TokenID: tokenID}
		if requireSide || strings.TrimSpace(string(param.Side)) != "" {
			side, err := normalizeSide(param.Side)
			if err != nil {
				return nil, err
			}
			entry.Side = side
		}
		normalized = append(normalized, entry)
	}
	return normalized, nil
}
