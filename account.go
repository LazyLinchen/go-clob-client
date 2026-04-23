package clobclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	endpointBalanceAllowance = "/balance-allowance"
	endpointPositions        = "/positions"
)

// SignatureType 表示账户类接口请求使用的签名主体类型。
type SignatureType int

const (
	// SignatureTypeEOA 表示普通外部账户签名。
	SignatureTypeEOA SignatureType = iota
	// SignatureTypePolyProxy 表示 Polymarket 代理钱包签名。
	SignatureTypePolyProxy
	// SignatureTypePolyGnosisSafe 表示 Gnosis Safe 钱包签名。
	SignatureTypePolyGnosisSafe
	// SignatureTypePoly1271 表示遵循 EIP-1271 的合约钱包签名。
	SignatureTypePoly1271
)

// AssetType 表示查询余额或授权额度时的资产类别。
type AssetType string

const (
	// AssetTypeCollateral 表示抵押资产。
	AssetTypeCollateral AssetType = "COLLATERAL"
	// AssetTypeConditional 表示条件资产。
	AssetTypeConditional AssetType = "CONDITIONAL"
)

// BalanceAllowanceParams 描述余额与授权额度查询参数。
type BalanceAllowanceParams struct {
	// AssetType 指定要查询的资产类型。
	AssetType AssetType
	// TokenID 在查询条件资产时必填。
	TokenID string
}

// BalanceAllowanceResponse 表示余额与授权额度接口的响应。
type BalanceAllowanceResponse struct {
	Balance    string            `json:"balance"`
	Allowance  string            `json:"allowance"`
	Allowances map[string]string `json:"allowances"`
}

// UnmarshalJSON 兼容服务端当前返回的余额与授权额度字段结构。
func (r *BalanceAllowanceResponse) UnmarshalJSON(data []byte) error {
	type wireBalanceAllowanceResponse struct {
		Balance    string            `json:"balance"`
		Allowance  string            `json:"allowance"`
		Allowances map[string]string `json:"allowances"`
	}

	var wire wireBalanceAllowanceResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	r.Balance = wire.Balance
	r.Allowance = wire.Allowance
	r.Allowances = wire.Allowances
	return nil
}

// PositionsParams 描述持仓列表查询时可选的过滤条件。
type PositionsParams struct {
	User           string
	Market         string
	EventID        string
	OrderBy        string
	OrderDirection string
	SizeThreshold  string
	Limit          int
	Offset         int
	Title          string
	CashPnlOnly    *bool
	Redeemable     *bool
	Mergeable      *bool
}

// Position 表示公开数据接口返回的一条持仓记录。
type Position struct {
	User               string       `json:"user"`
	ProxyWallet        string       `json:"proxyWallet"`
	Asset              string       `json:"asset"`
	ConditionID        string       `json:"conditionId"`
	Size               NumberString `json:"size"`
	AvgPrice           NumberString `json:"avgPrice"`
	InitialValue       NumberString `json:"initialValue"`
	CurrentValue       NumberString `json:"currentValue"`
	CashPnl            NumberString `json:"cashPnl"`
	PercentPnl         NumberString `json:"percentPnl"`
	TotalBought        NumberString `json:"totalBought"`
	TotalSold          NumberString `json:"totalSold"`
	RealizedPnl        NumberString `json:"realizedPnl"`
	PercentRealizedPnl NumberString `json:"percentRealizedPnl"`
	CurPrice           NumberString `json:"curPrice"`
	Title              string       `json:"title"`
	Slug               string       `json:"slug"`
	Icon               string       `json:"icon"`
	EventSlug          string       `json:"eventSlug"`
	Outcome            string       `json:"outcome"`
	OutcomeIndex       int64        `json:"outcomeIndex"`
	OppositeOutcome    string       `json:"oppositeOutcome"`
	OppositeAsset      string       `json:"oppositeAsset"`
	EndDate            string       `json:"endDate"`
	NegativeRisk       bool         `json:"negativeRisk"`
	Mergeable          bool         `json:"mergeable"`
	Redeemable         bool         `json:"redeemable"`
	TransactionHashes  []string     `json:"transactionHashes"`
}

// GetBalanceAllowance 查询指定资产的余额与授权额度。
func (c *Client) GetBalanceAllowance(ctx context.Context, params BalanceAllowanceParams) (*BalanceAllowanceResponse, error) {
	if c.creds == nil {
		return nil, errors.New("l2 credentials are not configured")
	}
	if c.signer == nil {
		return nil, errors.New("l1 signer is not configured")
	}

	assetType := AssetType(strings.ToUpper(strings.TrimSpace(string(params.AssetType))))
	switch assetType {
	case AssetTypeCollateral:
	case AssetTypeConditional:
		if strings.TrimSpace(params.TokenID) == "" {
			return nil, errors.New("tokenID is required for conditional assets")
		}
	default:
		return nil, fmt.Errorf("invalid asset type %q", params.AssetType)
	}

	query := url.Values{
		"asset_type":     []string{string(assetType)},
		"signature_type": []string{strconv.Itoa(int(c.signatureType))},
	}
	if tokenID := strings.TrimSpace(params.TokenID); tokenID != "" {
		query.Set("token_id", tokenID)
	}

	var response BalanceAllowanceResponse
	if err := c.doL2JSON(ctx, http.MethodGet, endpointBalanceAllowance, query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetPositions 按给定条件查询用户持仓列表。
func (c *Client) GetPositions(ctx context.Context, params PositionsParams) ([]Position, error) {
	user := strings.TrimSpace(params.User)
	if user == "" {
		return nil, errors.New("user is required")
	}

	query := url.Values{
		"user": []string{user},
	}
	if value := strings.TrimSpace(params.Market); value != "" {
		query.Set("market", value)
	}
	if value := strings.TrimSpace(params.EventID); value != "" {
		query.Set("eventId", value)
	}
	if value := strings.TrimSpace(params.OrderBy); value != "" {
		query.Set("orderBy", value)
	}
	if value := strings.TrimSpace(params.OrderDirection); value != "" {
		query.Set("orderDirection", value)
	}
	if value := strings.TrimSpace(params.SizeThreshold); value != "" {
		query.Set("sizeThreshold", value)
	}
	if value := strings.TrimSpace(params.Title); value != "" {
		query.Set("title", value)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.CashPnlOnly != nil {
		query.Set("cashPnlOnly", strconv.FormatBool(*params.CashPnlOnly))
	}
	if params.Redeemable != nil {
		query.Set("redeemable", strconv.FormatBool(*params.Redeemable))
	}
	if params.Mergeable != nil {
		query.Set("mergeable", strconv.FormatBool(*params.Mergeable))
	}

	var response []Position
	if err := c.doJSONBase(ctx, c.dataBaseURL, http.MethodGet, endpointPositions, query, nil, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}
