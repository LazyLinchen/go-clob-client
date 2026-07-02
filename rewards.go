package clobclient

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const (
	// endpointUserRewardPercentages 与 endpointUserEarnings 的路径已对照官方 docs 的
	// "Get reward percentages for user" / "Get user earnings and markets configuration" 确认。
	endpointUserRewardPercentages = "/rewards/user/percentages"
	endpointUserEarnings          = "/rewards/user/markets"
	// endpointCurrentRewards 为市场级激励配置；路径依据历史 SDK 与官方 CLI 的 current-rewards
	// 命令推断，响应结构对照官方 docs "current active rewards configurations" 示例。
	endpointCurrentRewards = "/rewards/markets/current"
)

// RewardConfigEntry 表示某市场激励配置中的单条资产发放规则。
type RewardConfigEntry struct {
	ID           int64        `json:"id"`
	AssetAddress string       `json:"asset_address"`
	StartDate    string       `json:"start_date"`
	EndDate      string       `json:"end_date"`
	RatePerDay   NumberString `json:"rate_per_day"`
	TotalRewards NumberString `json:"total_rewards"`
}

// RewardsMarket 表示一个开启激励市场的当前激励配置。
type RewardsMarket struct {
	ConditionID        string              `json:"condition_id"`
	RewardsMaxSpread   NumberString        `json:"rewards_max_spread"`
	RewardsMinSize     NumberString        `json:"rewards_min_size"`
	RewardsConfig      []RewardConfigEntry `json:"rewards_config"`
	SponsoredDailyRate NumberString        `json:"sponsored_daily_rate"`
	SponsorsCount      int64               `json:"sponsors_count"`
	NativeDailyRate    NumberString        `json:"native_daily_rate"`
	TotalDailyRate     NumberString        `json:"total_daily_rate"`
}

// RewardsMarketsResponse 表示当前激励市场列表接口的分页响应。
type RewardsMarketsResponse struct {
	Limit      int64           `json:"limit"`
	Count      int64           `json:"count"`
	NextCursor string          `json:"next_cursor"`
	Data       []RewardsMarket `json:"data"`
}

// UserRewardsMarket 表示用户在某市场的激励配置与收益信息。
type UserRewardsMarket struct {
	ConditionID           string              `json:"condition_id"`
	MarketID              NumberString        `json:"market_id"`
	EventID               NumberString        `json:"event_id"`
	Question              string              `json:"question"`
	MarketSlug            string              `json:"market_slug"`
	RewardsMaxSpread      NumberString        `json:"rewards_max_spread"`
	RewardsMinSize        NumberString        `json:"rewards_min_size"`
	Volume24hr            NumberString        `json:"volume_24hr"`
	Spread                NumberString        `json:"spread"`
	MarketCompetitiveness NumberString        `json:"market_competitiveness"`
	Tokens                []MarketToken       `json:"tokens"`
	RewardsConfig         []RewardConfigEntry `json:"rewards_config"`
}

// UserEarningsResponse 表示用户激励收益列表接口的分页响应。
type UserEarningsResponse struct {
	Limit      int64               `json:"limit"`
	Count      int64               `json:"count"`
	NextCursor string              `json:"next_cursor"`
	Data       []UserRewardsMarket `json:"data"`
}

// GetUserRewardPercentages 查询当前用户在各市场实时领取的激励占比，返回 condition_id -> 占比 的映射。
func (c *Client) GetUserRewardPercentages(ctx context.Context) (map[string]NumberString, error) {
	var response map[string]NumberString
	if err := c.doL2JSON(ctx, http.MethodGet, endpointUserRewardPercentages, nil, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// GetUserEarnings 查询当前用户某一天的激励收益与各市场配置，支持游标分页。
//
// date 为可选的 YYYY-MM-DD 日期；留空时由服务端决定默认值。
func (c *Client) GetUserEarnings(ctx context.Context, date string, nextCursor string) (*UserEarningsResponse, error) {
	query := make(url.Values)
	if value := strings.TrimSpace(date); value != "" {
		query.Set("date", value)
	}
	if value := strings.TrimSpace(nextCursor); value != "" {
		query.Set("next_cursor", value)
	}
	if len(query) == 0 {
		query = nil
	}

	var response UserEarningsResponse
	if err := c.doL2JSON(ctx, http.MethodGet, endpointUserEarnings, query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetCurrentRewards 查询当前开启激励的市场配置列表，支持游标分页。
//
// sponsored 为 true 时返回赞助激励配置，否则返回常规激励配置。
func (c *Client) GetCurrentRewards(ctx context.Context, sponsored bool, nextCursor string) (*RewardsMarketsResponse, error) {
	query := make(url.Values)
	if sponsored {
		query.Set("sponsored", "true")
	}
	if value := strings.TrimSpace(nextCursor); value != "" {
		query.Set("next_cursor", value)
	}
	if len(query) == 0 {
		query = nil
	}

	var response RewardsMarketsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointCurrentRewards, query, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
