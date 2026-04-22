package clobclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) GetOK(ctx context.Context) ([]byte, error) {
	return c.doBytes(ctx, http.MethodGet, "/ok", nil, nil)
}

func (c *Client) GetServerTime(ctx context.Context) (int64, error) {
	var ts int64
	if err := c.doJSON(ctx, http.MethodGet, "/time", nil, nil, nil, &ts); err != nil {
		return 0, err
	}
	return ts, nil
}

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
