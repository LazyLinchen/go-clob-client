package clobclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	endpointOrder              = "/order"
	endpointOrders             = "/orders"
	endpointGetOrderSDK        = "/data/order/"
	endpointGetOrderDocs       = "/order/"
	endpointGetOrders          = "/data/orders"
	endpointCancelAll          = "/cancel-all"
	endpointCancelMarketOrders = "/cancel-market-orders"
	endpointOrderScoring       = "/order-scoring"
	endpointHeartbeats         = "/heartbeats"
)

// UserOrdersParams 描述用户订单列表查询条件。
type UserOrdersParams struct {
	ID         string
	Market     string
	AssetID    string
	NextCursor string
}

// UserOrdersResponse 表示用户订单列表接口的分页响应。
type UserOrdersResponse struct {
	Limit      int64       `json:"limit"`
	Count      int64       `json:"count"`
	NextCursor string      `json:"next_cursor"`
	Data       []OpenOrder `json:"data"`
}

// OpenOrder 表示订单查询接口返回的一条订单记录。
type OpenOrder struct {
	ID              string   `json:"id"`
	Status          string   `json:"status"`
	Owner           string   `json:"owner"`
	MakerAddress    string   `json:"maker_address"`
	Market          string   `json:"market"`
	AssetID         string   `json:"asset_id"`
	Side            string   `json:"side"`
	OriginalSize    string   `json:"original_size"`
	SizeMatched     string   `json:"size_matched"`
	Price           string   `json:"price"`
	AssociateTrades []string `json:"associate_trades"`
	Outcome         string   `json:"outcome"`
	CreatedAt       int64    `json:"created_at"`
	Expiration      string   `json:"expiration"`
	OrderType       string   `json:"order_type"`
}

// CancelOrderRequest 表示单笔撤单请求体。
type CancelOrderRequest struct {
	OrderID string `json:"orderID"`
}

// CancelOrderResponse 表示撤单接口的响应结果。
type CancelOrderResponse struct {
	Canceled    []string        `json:"canceled"`
	NotCanceled json.RawMessage `json:"not_canceled"`
}

// CancelMarketOrdersParams 描述按市场批量撤单的过滤条件。
type CancelMarketOrdersParams struct {
	Market  string `json:"market,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
}

// PostOrderRequest 表示提交单笔订单时的请求体。
type PostOrderRequest struct {
	Order     any    `json:"order"`
	Owner     string `json:"owner"`
	OrderType string `json:"orderType,omitempty"`
	PostOnly  *bool  `json:"postOnly,omitempty"`
	DeferExec *bool  `json:"deferExec,omitempty"`
}

// PostOrderResponse 表示下单接口返回的结果。
type PostOrderResponse struct {
	Success            bool         `json:"success"`
	OrderID            string       `json:"orderID"`
	Status             string       `json:"status"`
	MakingAmount       NumberString `json:"makingAmount"`
	TakingAmount       NumberString `json:"takingAmount"`
	TransactionsHashes []string     `json:"transactionsHashes"`
	TransactionHashes  []string     `json:"transactionHashes"`
	TradeIDs           []string     `json:"tradeIDs"`
	ErrorMsg           string       `json:"errorMsg"`
}

// OrderScoringResponse 表示订单评分状态查询结果。
type OrderScoringResponse struct {
	Scoring bool `json:"scoring"`
}

// HeartbeatResponse 表示心跳接口响应。
type HeartbeatResponse struct {
	Status      string `json:"status"`
	HeartbeatID string `json:"heartbeat_id"`
}

// UnmarshalJSON 兼容 SDK 与服务端在字段命名上的差异。
func (r *PostOrderResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		Success            bool         `json:"success"`
		OrderID            string       `json:"orderID"`
		OrderIDAlt         string       `json:"orderId"`
		Status             string       `json:"status"`
		MakingAmount       NumberString `json:"makingAmount"`
		MakingAmountAlt    NumberString `json:"making_amount"`
		TakingAmount       NumberString `json:"takingAmount"`
		TakingAmountAlt    NumberString `json:"taking_amount"`
		TransactionsHashes []string     `json:"transactionsHashes"`
		TransactionHashes  []string     `json:"transactionHashes"`
		TradeIDs           []string     `json:"tradeIDs"`
		TradeIDsAlt        []string     `json:"tradeIds"`
		ErrorMsg           string       `json:"errorMsg"`
		ErrorMsgAlt        string       `json:"error_message"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	r.Success = wire.Success
	switch {
	case wire.OrderID != "":
		r.OrderID = wire.OrderID
	default:
		r.OrderID = wire.OrderIDAlt
	}

	r.Status = wire.Status
	switch {
	case wire.MakingAmount != "":
		r.MakingAmount = wire.MakingAmount
	default:
		r.MakingAmount = wire.MakingAmountAlt
	}
	switch {
	case wire.TakingAmount != "":
		r.TakingAmount = wire.TakingAmount
	default:
		r.TakingAmount = wire.TakingAmountAlt
	}
	if len(wire.TransactionsHashes) > 0 {
		r.TransactionsHashes = wire.TransactionsHashes
	} else {
		r.TransactionsHashes = wire.TransactionHashes
	}
	r.TransactionHashes = append([]string(nil), r.TransactionsHashes...)
	if len(wire.TradeIDs) > 0 {
		r.TradeIDs = wire.TradeIDs
	} else {
		r.TradeIDs = wire.TradeIDsAlt
	}
	switch {
	case wire.ErrorMsg != "":
		r.ErrorMsg = wire.ErrorMsg
	default:
		r.ErrorMsg = wire.ErrorMsgAlt
	}
	return nil
}

// GetOrder 根据订单 ID 查询单笔订单详情。
func (c *Client) GetOrder(ctx context.Context, orderID string) (*OpenOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("orderID is required")
	}

	var order OpenOrder
	requestPath := endpointGetOrderSDK + url.PathEscape(orderID)
	err := c.doL2JSON(ctx, http.MethodGet, requestPath, nil, nil, &order)
	if err == nil {
		return &order, nil
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return nil, err
	}

	// 官方文档与官方 SDK 对查询路径存在分歧，这里优先走 SDK 路径，
	// 只有在明确 404 时才回退到文档路径。
	requestPath = endpointGetOrderDocs + url.PathEscape(orderID)
	if retryErr := c.doL2JSON(ctx, http.MethodGet, requestPath, nil, nil, &order); retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return &order, nil
}

// GetUserOrders 查询当前凭证对应用户的订单列表。
func (c *Client) GetUserOrders(ctx context.Context, params UserOrdersParams) (*UserOrdersResponse, error) {
	query := make(url.Values)
	if value := strings.TrimSpace(params.ID); value != "" {
		query.Set("id", value)
	}
	if value := strings.TrimSpace(params.Market); value != "" {
		query.Set("market", value)
	}
	if value := strings.TrimSpace(params.AssetID); value != "" {
		query.Set("asset_id", value)
	}
	if value := strings.TrimSpace(params.NextCursor); value != "" {
		query.Set("next_cursor", value)
	}

	var response UserOrdersResponse
	if err := c.doL2JSON(ctx, http.MethodGet, endpointGetOrders, query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CancelOrder 撤销单笔订单。
func (c *Client) CancelOrder(ctx context.Context, orderID string) (*CancelOrderResponse, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("orderID is required")
	}

	body, err := encodeJSON(CancelOrderRequest{OrderID: orderID})
	if err != nil {
		return nil, fmt.Errorf("encode cancel order payload: %w", err)
	}

	var response CancelOrderResponse
	if err := c.doL2JSON(ctx, http.MethodDelete, endpointOrder, nil, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// PostOrder 提交一笔已完成签名的订单。
func (c *Client) PostOrder(ctx context.Context, request PostOrderRequest) (*PostOrderResponse, error) {
	payload, err := c.normalizePostOrderRequest(request)
	if err != nil {
		return nil, err
	}

	body, err := encodeJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("encode post order payload: %w", err)
	}

	var response PostOrderResponse
	if err := c.doL2JSON(ctx, http.MethodPost, endpointOrder, nil, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// PostOrders 批量提交多笔已完成签名的订单。
func (c *Client) PostOrders(ctx context.Context, requests []PostOrderRequest) ([]PostOrderResponse, error) {
	payload, err := c.normalizePostOrderRequests(requests)
	if err != nil {
		return nil, err
	}

	body, err := encodeJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("encode post orders payload: %w", err)
	}

	var response []PostOrderResponse
	if err := c.doL2JSON(ctx, http.MethodPost, endpointOrders, nil, body, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// CancelOrders 按订单 ID 列表批量撤单。
func (c *Client) CancelOrders(ctx context.Context, orderIDs []string) (*CancelOrderResponse, error) {
	normalizedOrderIDs, err := normalizeOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}

	body, err := encodeJSON(normalizedOrderIDs)
	if err != nil {
		return nil, fmt.Errorf("encode cancel orders payload: %w", err)
	}

	var response CancelOrderResponse
	if err := c.doL2JSON(ctx, http.MethodDelete, endpointOrders, nil, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CancelAllOrders 撤销当前用户的全部订单。
func (c *Client) CancelAllOrders(ctx context.Context) (*CancelOrderResponse, error) {
	var response CancelOrderResponse
	if err := c.doL2JSON(ctx, http.MethodDelete, endpointCancelAll, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CancelMarketOrders 按市场或资产维度批量撤单。
func (c *Client) CancelMarketOrders(ctx context.Context, params CancelMarketOrdersParams) (*CancelOrderResponse, error) {
	payload := CancelMarketOrdersParams{
		Market:  strings.TrimSpace(params.Market),
		AssetID: strings.TrimSpace(params.AssetID),
	}
	if payload.Market == "" && payload.AssetID == "" {
		return nil, errors.New("market or assetID is required")
	}

	body, err := encodeJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("encode cancel market orders payload: %w", err)
	}

	var response CancelOrderResponse
	if err := c.doL2JSON(ctx, http.MethodDelete, endpointCancelMarketOrders, nil, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetOrderScoringStatus 查询订单是否已进入 scoring 状态。
func (c *Client) GetOrderScoringStatus(ctx context.Context, orderID string) (*OrderScoringResponse, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, errors.New("orderID is required")
	}

	query := url.Values{
		"order_id": []string{orderID},
	}
	var response OrderScoringResponse
	if err := c.doL2JSON(ctx, http.MethodGet, endpointOrderScoring, query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// SendHeartbeat 发送心跳请求以维持对应会话或订单状态。
func (c *Client) SendHeartbeat(ctx context.Context, heartbeatID string) (*HeartbeatResponse, error) {
	heartbeatID = strings.TrimSpace(heartbeatID)

	var body []byte
	var err error
	if heartbeatID != "" {
		body, err = encodeJSON(struct {
			HeartbeatID string `json:"heartbeat_id"`
		}{
			HeartbeatID: heartbeatID,
		})
		if err != nil {
			return nil, fmt.Errorf("encode heartbeat payload: %w", err)
		}
	}

	var response HeartbeatResponse
	if err := c.doL2JSON(ctx, http.MethodPost, endpointHeartbeats, nil, body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// normalizePostOrderRequest 清洗单笔下单请求，避免把空 owner 或无效 JSON 传给服务端。
func (c *Client) normalizePostOrderRequest(request PostOrderRequest) (PostOrderRequest, error) {
	order, err := normalizeEmbeddedJSON(request.Order, "order")
	if err != nil {
		return PostOrderRequest{}, err
	}

	owner := strings.TrimSpace(request.Owner)
	if owner == "" {
		owner = c.defaultOwner()
	}
	if owner == "" {
		return PostOrderRequest{}, errors.New("owner is required")
	}

	return PostOrderRequest{
		Order:     order,
		Owner:     owner,
		OrderType: strings.TrimSpace(request.OrderType),
		PostOnly:  request.PostOnly,
		DeferExec: request.DeferExec,
	}, nil
}

// normalizePostOrderRequests 清洗批量下单请求，并应用服务端允许的数量限制。
func (c *Client) normalizePostOrderRequests(requests []PostOrderRequest) ([]PostOrderRequest, error) {
	if len(requests) == 0 {
		return nil, errors.New("at least one order is required")
	}
	if len(requests) > 15 {
		return nil, errors.New("a maximum of 15 orders is allowed per request")
	}

	payload := make([]PostOrderRequest, 0, len(requests))
	for i, request := range requests {
		normalized, err := c.normalizePostOrderRequest(request)
		if err != nil {
			return nil, fmt.Errorf("order %d: %w", i, err)
		}
		payload = append(payload, normalized)
	}
	return payload, nil
}

func (c *Client) defaultOwner() string {
	if c == nil || c.creds == nil {
		return ""
	}
	return strings.TrimSpace(c.creds.credentialKey())
}

// normalizeOrderIDs 去重并清理订单 ID，避免把空值和重复值传给批量撤单接口。
func normalizeOrderIDs(orderIDs []string) ([]string, error) {
	if len(orderIDs) == 0 {
		return nil, errors.New("at least one orderID is required")
	}

	seen := make(map[string]struct{}, len(orderIDs))
	normalized := make([]string, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		orderID = strings.TrimSpace(orderID)
		if orderID == "" {
			continue
		}
		if _, ok := seen[orderID]; ok {
			continue
		}
		seen[orderID] = struct{}{}
		normalized = append(normalized, orderID)
	}

	if len(normalized) == 0 {
		return nil, errors.New("at least one orderID is required")
	}
	if len(normalized) > 3000 {
		return nil, errors.New("a maximum of 3000 orderIDs is allowed per request")
	}
	return normalized, nil
}

// normalizeEmbeddedJSON 确保透传给服务端的嵌套 JSON 片段是有效值。
func normalizeEmbeddedJSON(value any, fieldName string) (any, error) {
	switch raw := value.(type) {
	case nil:
		return nil, fmt.Errorf("%s is required", fieldName)
	case []byte:
		return normalizeEmbeddedJSON(json.RawMessage(bytes.TrimSpace(raw)), fieldName)
	case json.RawMessage:
		raw = json.RawMessage(bytes.TrimSpace(raw))
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return nil, fmt.Errorf("%s is required", fieldName)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("%s must be valid JSON", fieldName)
		}
		return raw, nil
	default:
		return value, nil
	}
}
