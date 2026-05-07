package clobclient

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Side 表示订单或报价方向。
type Side string

const (
	// SideBuy 表示买入方向。
	SideBuy Side = "BUY"
	// SideSell 表示卖出方向。
	SideSell Side = "SELL"
)

// NumberString 用字符串形式保存数值，避免精度丢失。
type NumberString string

// String 返回原始字符串表示。
func (n NumberString) String() string {
	return string(n)
}

// UnmarshalJSON 兼容字符串、数字和 null 三种常见 JSON 数值表示。
func (n *NumberString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*n = ""
		return nil
	}

	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*n = NumberString(s)
		return nil
	}

	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&num); err != nil {
		return fmt.Errorf("decode numeric value: %w", err)
	}

	*n = NumberString(num.String())
	return nil
}

// BookLevel 表示盘口中的一个价位档位。
type BookLevel struct {
	Price NumberString `json:"price"`
	Size  NumberString `json:"size"`
}

// OrderBook 表示某个 token 的完整盘口快照。
type OrderBook struct {
	Market         string       `json:"market"`
	AssetID        string       `json:"asset_id"`
	Timestamp      string       `json:"timestamp"`
	Hash           string       `json:"hash"`
	Bids           []BookLevel  `json:"bids"`
	Asks           []BookLevel  `json:"asks"`
	MinOrderSize   NumberString `json:"min_order_size"`
	TickSize       NumberString `json:"tick_size"`
	NegRisk        bool         `json:"neg_risk"`
	LastTradePrice NumberString `json:"last_trade_price"`
}

// PriceResponse 表示价格查询接口响应。
type PriceResponse struct {
	Price NumberString `json:"price"`
}

// UnmarshalJSON 兼容标量和对象两种价格返回格式。
func (r *PriceResponse) UnmarshalJSON(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	if looksLikeScalarJSON(data) {
		var price NumberString
		if err := json.Unmarshal(data, &price); err != nil {
			return err
		}
		r.Price = price
		return nil
	}

	type alias PriceResponse
	var wire alias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	*r = PriceResponse(wire)
	return nil
}

// MidpointResponse 表示中间价查询接口响应。
type MidpointResponse struct {
	Midpoint NumberString
}

// UnmarshalJSON 兼容 mid_price、mid 以及纯标量三种返回格式。
func (r *MidpointResponse) UnmarshalJSON(data []byte) error {
	if looksLikeScalarJSON(data) {
		return json.Unmarshal(data, &r.Midpoint)
	}

	var wire struct {
		MidPrice NumberString `json:"mid_price"`
		Mid      NumberString `json:"mid"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.MidPrice != "":
		r.Midpoint = wire.MidPrice
	case wire.Mid != "":
		r.Midpoint = wire.Mid
	}

	return nil
}

// SpreadResponse 表示价差查询接口响应。
type SpreadResponse struct {
	Spread NumberString `json:"spread"`
}

// UnmarshalJSON 兼容标量和对象两种价差返回格式。
func (r *SpreadResponse) UnmarshalJSON(data []byte) error {
	if looksLikeScalarJSON(data) {
		return json.Unmarshal(data, &r.Spread)
	}

	type alias SpreadResponse
	var wire alias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	*r = SpreadResponse(wire)
	return nil
}

// TickSizeResponse 表示最小跳动单位查询接口响应。
type TickSizeResponse struct {
	MinimumTickSize NumberString `json:"minimum_tick_size"`
}

// NegRiskResponse 表示 neg-risk 查询接口响应。
type NegRiskResponse struct {
	NegRisk bool `json:"neg_risk"`
}

// MarketByTokenResponse 表示按 token 查询市场元数据的最小响应。
type MarketByTokenResponse struct {
	ConditionID string `json:"condition_id"`
}

// UnmarshalJSON 兼容 condition_id、conditionId 和 market 三种常见字段名。
func (r *MarketByTokenResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		ConditionID      string `json:"condition_id"`
		ConditionIDCamel string `json:"conditionId"`
		Market           string `json:"market"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.ConditionID != "":
		r.ConditionID = wire.ConditionID
	case wire.ConditionIDCamel != "":
		r.ConditionID = wire.ConditionIDCamel
	default:
		r.ConditionID = wire.Market
	}
	return nil
}

// UnmarshalJSON 兼容 minimum_tick_size、tick_size 以及纯标量格式。
func (r *TickSizeResponse) UnmarshalJSON(data []byte) error {
	if looksLikeScalarJSON(data) {
		return json.Unmarshal(data, &r.MinimumTickSize)
	}

	var wire struct {
		MinimumTickSize NumberString `json:"minimum_tick_size"`
		TickSize        NumberString `json:"tick_size"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.MinimumTickSize != "":
		r.MinimumTickSize = wire.MinimumTickSize
	case wire.TickSize != "":
		r.MinimumTickSize = wire.TickSize
	}

	return nil
}

// CLOBMarketToken 表示市场信息中的单个 outcome token。
type CLOBMarketToken struct {
	TokenID string       `json:"t"`
	Outcome string       `json:"o"`
	Price   NumberString `json:"price"`
	Winner  bool         `json:"winner"`
}

// UnmarshalJSON 兼容长字段名和短字段名两套返回格式。
func (t *CLOBMarketToken) UnmarshalJSON(data []byte) error {
	var wire struct {
		TokenIDShort string       `json:"t"`
		OutcomeShort string       `json:"o"`
		TokenID      string       `json:"token_id"`
		Outcome      string       `json:"outcome"`
		Price        NumberString `json:"price"`
		Winner       bool         `json:"winner"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.TokenID != "":
		t.TokenID = wire.TokenID
	default:
		t.TokenID = wire.TokenIDShort
	}

	switch {
	case wire.Outcome != "":
		t.Outcome = wire.Outcome
	default:
		t.Outcome = wire.OutcomeShort
	}

	t.Price = wire.Price
	t.Winner = wire.Winner
	return nil
}

// FeeDetails 表示市场费率细节。
type FeeDetails struct {
	Rate      NumberString `json:"r"`
	Exponent  int64        `json:"e"`
	TakerOnly bool         `json:"to"`
}

// CLOBMarketInfo 表示扩展市场信息接口返回的市场元数据。
type CLOBMarketInfo struct {
	GameStartTime          string            `json:"gst"`
	Rewards                json.RawMessage   `json:"r"`
	Tokens                 []CLOBMarketToken `json:"t"`
	MinimumOrderSize       NumberString      `json:"mos"`
	MinimumTickSize        NumberString      `json:"mts"`
	MakerBaseFeeBPS        int64             `json:"mbf"`
	TakerBaseFeeBPS        int64             `json:"tbf"`
	RFQEnabled             bool              `json:"rfqe"`
	TakerOrderDelayEnabled bool              `json:"itode"`
	BlockaidEnabled        bool              `json:"ibce"`
	FeeDetails             FeeDetails        `json:"fd"`
	MinimumOrderAgeSeconds int64             `json:"oas"`
}

// UnmarshalJSON 兼容官方文档中的长字段名和线上接口的短字段名。
func (m *CLOBMarketInfo) UnmarshalJSON(data []byte) error {
	var wire struct {
		GameStartTimeShort     string            `json:"gst"`
		GameStartTime          string            `json:"game_start_time"`
		RewardsShort           json.RawMessage   `json:"r"`
		Rewards                json.RawMessage   `json:"rewards"`
		TokensShort            []CLOBMarketToken `json:"t"`
		Tokens                 []CLOBMarketToken `json:"tokens"`
		MinimumOrderSizeShort  NumberString      `json:"mos"`
		MinimumOrderSize       NumberString      `json:"minimum_order_size"`
		MinimumTickSizeShort   NumberString      `json:"mts"`
		MinimumTickSize        NumberString      `json:"minimum_tick_size"`
		MakerBaseFeeShort      int64             `json:"mbf"`
		MakerBaseFee           int64             `json:"maker_base_fee"`
		TakerBaseFeeShort      int64             `json:"tbf"`
		TakerBaseFee           int64             `json:"taker_base_fee"`
		RFQEnabled             bool              `json:"rfqe"`
		TakerOrderDelayEnabled bool              `json:"itode"`
		BlockaidEnabled        bool              `json:"ibce"`
		FeeDetails             FeeDetails        `json:"fd"`
		MinimumOrderAgeSeconds int64             `json:"oas"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.GameStartTime != "":
		m.GameStartTime = wire.GameStartTime
	default:
		m.GameStartTime = wire.GameStartTimeShort
	}

	switch {
	case len(wire.Rewards) > 0:
		m.Rewards = wire.Rewards
	default:
		m.Rewards = wire.RewardsShort
	}

	if len(wire.Tokens) > 0 {
		m.Tokens = wire.Tokens
	} else {
		m.Tokens = wire.TokensShort
	}

	switch {
	case wire.MinimumOrderSize != "":
		m.MinimumOrderSize = wire.MinimumOrderSize
	default:
		m.MinimumOrderSize = wire.MinimumOrderSizeShort
	}

	switch {
	case wire.MinimumTickSize != "":
		m.MinimumTickSize = wire.MinimumTickSize
	default:
		m.MinimumTickSize = wire.MinimumTickSizeShort
	}

	switch {
	case wire.MakerBaseFee != 0:
		m.MakerBaseFeeBPS = wire.MakerBaseFee
	default:
		m.MakerBaseFeeBPS = wire.MakerBaseFeeShort
	}

	switch {
	case wire.TakerBaseFee != 0:
		m.TakerBaseFeeBPS = wire.TakerBaseFee
	default:
		m.TakerBaseFeeBPS = wire.TakerBaseFeeShort
	}

	m.RFQEnabled = wire.RFQEnabled
	m.TakerOrderDelayEnabled = wire.TakerOrderDelayEnabled
	m.BlockaidEnabled = wire.BlockaidEnabled
	m.FeeDetails = wire.FeeDetails
	m.MinimumOrderAgeSeconds = wire.MinimumOrderAgeSeconds
	return nil
}

// looksLikeScalarJSON 用于区分标量响应和对象响应，方便兼容历史接口格式。
func looksLikeScalarJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return false
	}

	switch data[0] {
	case '{', '[':
		return false
	default:
		return true
	}
}
