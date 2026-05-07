package clobclient

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
)

const (
	endpointBuilderFees = "/fees/builder-fees/"
	builderFeesBPS      = 10000
)

// BuilderFeeRates 表示服务端配置的 builder maker/taker fee，单位为 bps。
type BuilderFeeRates struct {
	MakerFeeRateBps int64 `json:"builder_maker_fee_rate_bps"`
	TakerFeeRateBps int64 `json:"builder_taker_fee_rate_bps"`
}

// MarketFeeInfo 表示 V2 market fee 参数；Rate 和 Exponent 与 CLOB market info 的 fd 字段一致。
type MarketFeeInfo struct {
	Rate     string `json:"rate"`
	Exponent int64  `json:"exponent"`
}

// GetBuilderFeeRates 查询指定 builder code 对应的 maker/taker fee 配置。
func (c *Client) GetBuilderFeeRates(ctx context.Context, builderCode string) (*BuilderFeeRates, error) {
	builderCode, err := normalizeBuilderCode(builderCode)
	if err != nil {
		return nil, err
	}
	if builderCode == ZeroBytes32 {
		return nil, errors.New("builderCode is required")
	}

	var response BuilderFeeRates
	endpoint := endpointBuilderFees + url.PathEscape(builderCode)
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, nil, nil, &response); err != nil {
		return nil, err
	}
	if response.MakerFeeRateBps < 0 || response.TakerFeeRateBps < 0 {
		return nil, fmt.Errorf("builder fee rates must be non-negative")
	}
	return &response, nil
}

// NormalizeBuilderCode validates and normalizes a builder code into a lowercase 0x-prefixed bytes32 value.
func NormalizeBuilderCode(builderCode string) (string, error) {
	return normalizeBuilderCode(builderCode)
}

func normalizeBuilderCode(builderCode string) (string, error) {
	return normalizeBytes32(builderCode, "builderCode")
}

func normalizeOptionalBuilderCode(builderCode string) (string, error) {
	normalized, err := normalizeBuilderCode(builderCode)
	if err != nil {
		return "", err
	}
	if normalized == ZeroBytes32 {
		return "", nil
	}
	return normalized, nil
}

func isBuilderOrder(builderCode string) bool {
	value := strings.TrimSpace(builderCode)
	return value != "" && !strings.EqualFold(value, ZeroBytes32)
}

func builderFeeRateFromBps(bps int64) (*big.Rat, error) {
	if bps < 0 {
		return nil, errors.New("builder fee rate bps must be non-negative")
	}
	return new(big.Rat).SetFrac(big.NewInt(bps), big.NewInt(builderFeesBPS)), nil
}

func (f MarketFeeInfo) normalized() (MarketFeeInfo, error) {
	rate := strings.TrimSpace(f.Rate)
	if rate == "" {
		rate = "0"
	}
	if _, err := parseNonNegativeDecimal(rate, "fee rate"); err != nil {
		return MarketFeeInfo{}, err
	}
	if f.Exponent < 0 {
		return MarketFeeInfo{}, errors.New("fee exponent must be non-negative")
	}
	return MarketFeeInfo{Rate: rate, Exponent: f.Exponent}, nil
}

func (f FeeDetails) marketFeeInfo() MarketFeeInfo {
	return MarketFeeInfo{
		Rate:     strings.TrimSpace(string(f.Rate)),
		Exponent: f.Exponent,
	}
}
