package clobclient

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// FunderCandidateSource 描述 funder 候选地址的来源。
type FunderCandidateSource string

const (
	// FunderCandidateSourceConfigured 表示候选地址来自客户端显式配置。
	FunderCandidateSourceConfigured FunderCandidateSource = "client.funderAddress"
	// FunderCandidateSourceSigner 表示候选地址来自当前 signer / user。
	FunderCandidateSourceSigner FunderCandidateSource = "signer"
	// FunderCandidateSourceOrdersMakerAddress 表示候选地址来自当前用户订单的 maker_address 字段。
	FunderCandidateSourceOrdersMakerAddress FunderCandidateSource = "orders.maker_address"
	// FunderCandidateSourcePositionsProxyWallet 表示候选地址来自 positions 响应的 proxyWallet 字段。
	FunderCandidateSourcePositionsProxyWallet FunderCandidateSource = "positions.proxyWallet"
)

// FunderCandidate 表示一个可用于下单 maker/funder 的候选地址。
type FunderCandidate struct {
	Address       string                `json:"address"`
	Source        FunderCandidateSource `json:"source"`
	PositionCount int                   `json:"positionCount,omitempty"`
	OrderCount    int                   `json:"orderCount,omitempty"`
}

// FunderDiscoveryParams 描述 funder 半自动发现时的可选查询条件。
type FunderDiscoveryParams struct {
	// User 指定 positions 查询用户；留空时优先使用 signer 地址。
	User string
	// Market 将 positions 查询限制在指定市场。
	Market string
	// AssetID 将用户订单查询限制在指定 asset_id。
	AssetID string
	// EventID 将 positions 查询限制在指定事件。
	EventID string
	// Limit 控制 positions 查询条数；留空时由服务端决定默认值。
	Limit int
}

// FunderDiscoveryResult 表示 funder 自动发现的输出。
type FunderDiscoveryResult struct {
	LookupUser    string            `json:"lookupUser"`
	SignatureType SignatureType     `json:"signatureType"`
	Candidates    []FunderCandidate `json:"candidates"`
	Preferred     *FunderCandidate  `json:"preferred,omitempty"`
}

// DiscoverFunder 返回 funder 候选列表；当结果唯一或已显式配置时会同时给出 Preferred。
func (c *Client) DiscoverFunder(ctx context.Context, params FunderDiscoveryParams) (*FunderDiscoveryResult, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	result := &FunderDiscoveryResult{
		SignatureType: c.signatureType,
	}

	if configured := strings.TrimSpace(c.funderAddress); configured != "" {
		candidate := FunderCandidate{
			Address: configured,
			Source:  FunderCandidateSourceConfigured,
		}
		result.Candidates = []FunderCandidate{candidate}
		result.Preferred = &result.Candidates[0]
		if c.signer != nil {
			result.LookupUser = common.HexToAddress(c.signer.Address()).Hex()
		}
		return result, nil
	}

	lookupUser, hasLookupUser, err := c.resolveFunderDiscoveryUser(params.User)
	if err != nil {
		return nil, err
	}
	if hasLookupUser {
		result.LookupUser = lookupUser
	}

	if c.signatureType == SignatureTypeEOA {
		if !hasLookupUser {
			return nil, errors.New("user is required when signer is not configured")
		}
		candidate := FunderCandidate{
			Address: lookupUser,
			Source:  FunderCandidateSourceSigner,
		}
		result.Candidates = []FunderCandidate{candidate}
		result.Preferred = &result.Candidates[0]
		return result, nil
	}

	collector := newFunderCandidateCollector()
	usedDiscoverySource := false

	if hasLookupUser {
		positions, err := c.GetPositions(ctx, PositionsParams{
			User:    lookupUser,
			Market:  params.Market,
			EventID: params.EventID,
			Limit:   params.Limit,
		})
		if err != nil {
			return nil, err
		}
		collector.addPositions(positions)
		usedDiscoverySource = true
	}

	if c.canDiscoverFunderFromOrders(params.User, lookupUser) {
		orders, err := c.GetUserOrders(ctx, UserOrdersParams{
			Market:  params.Market,
			AssetID: params.AssetID,
		})
		if err != nil {
			return nil, err
		}
		collector.addOrders(orders.Data)
		usedDiscoverySource = true
	}

	if !usedDiscoverySource {
		return nil, errors.New("user is required when signer is not configured and L2 credentials are not configured")
	}

	result.Candidates = collector.candidates()
	if preferredIndex, ok := preferredFunderCandidateIndex(result.Candidates); ok {
		result.Preferred = &result.Candidates[preferredIndex]
	}
	return result, nil
}

// WithDiscoveredFunder 返回一个带自动发现 funder 的客户端副本；当候选不唯一时返回 discovery 结果和错误。
func (c *Client) WithDiscoveredFunder(ctx context.Context, params FunderDiscoveryParams) (*Client, *FunderDiscoveryResult, error) {
	result, err := c.DiscoverFunder(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	if result.Preferred == nil {
		subject := funderDiscoverySubject(result)
		if len(result.Candidates) == 0 {
			return nil, result, fmt.Errorf("no funder candidates found for %s; set FunderAddress explicitly", subject)
		}
		return nil, result, fmt.Errorf("multiple funder candidates found for %s; choose one explicitly", subject)
	}

	cloned, err := c.WithFunderAddress(result.Preferred.Address)
	if err != nil {
		return nil, result, err
	}
	return cloned, result, nil
}

func (c *Client) resolveFunderDiscoveryUser(explicitUser string) (string, bool, error) {
	user := strings.TrimSpace(explicitUser)
	if user == "" && c.signer != nil {
		user = strings.TrimSpace(c.signer.Address())
	}
	if user == "" {
		return "", false, nil
	}
	if !common.IsHexAddress(user) {
		return "", false, fmt.Errorf("invalid user address %q", user)
	}
	return common.HexToAddress(user).Hex(), true, nil
}

func (c *Client) canDiscoverFunderFromOrders(explicitUser string, normalizedUser string) bool {
	if c == nil || c.creds == nil || c.signer == nil {
		return false
	}

	explicitUser = strings.TrimSpace(explicitUser)
	if explicitUser == "" {
		return true
	}
	if normalizedUser == "" {
		return false
	}
	return strings.EqualFold(normalizedUser, common.HexToAddress(c.signer.Address()).Hex())
}

func funderDiscoverySubject(result *FunderDiscoveryResult) string {
	if result == nil {
		return "current context"
	}
	if lookupUser := strings.TrimSpace(result.LookupUser); lookupUser != "" {
		return fmt.Sprintf("user %s", lookupUser)
	}
	return "current authenticated user"
}

type funderCandidateCollector struct {
	counts map[string]*funderCandidateCount
}

type funderCandidateCount struct {
	positionCount int
	orderCount    int
}

func newFunderCandidateCollector() *funderCandidateCollector {
	return &funderCandidateCollector{
		counts: make(map[string]*funderCandidateCount),
	}
}

func (c *funderCandidateCollector) addPositions(positions []Position) {
	for _, position := range positions {
		c.add(position.ProxyWallet, FunderCandidateSourcePositionsProxyWallet)
	}
}

func (c *funderCandidateCollector) addOrders(orders []OpenOrder) {
	for _, order := range orders {
		c.add(order.MakerAddress, FunderCandidateSourceOrdersMakerAddress)
	}
}

func (c *funderCandidateCollector) add(address string, source FunderCandidateSource) {
	address = strings.TrimSpace(address)
	if address == "" || !common.IsHexAddress(address) {
		return
	}

	normalized := common.HexToAddress(address).Hex()
	count := c.counts[normalized]
	if count == nil {
		count = &funderCandidateCount{}
		c.counts[normalized] = count
	}
	switch source {
	case FunderCandidateSourceOrdersMakerAddress:
		count.orderCount++
	case FunderCandidateSourcePositionsProxyWallet:
		count.positionCount++
	}
}

func (c *funderCandidateCollector) candidates() []FunderCandidate {
	candidates := make([]FunderCandidate, 0, len(c.counts))
	for address, count := range c.counts {
		source := FunderCandidateSourcePositionsProxyWallet
		if count.orderCount > 0 {
			source = FunderCandidateSourceOrdersMakerAddress
		}

		candidates = append(candidates, FunderCandidate{
			Address:       address,
			Source:        source,
			PositionCount: count.positionCount,
			OrderCount:    count.orderCount,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].OrderCount != candidates[j].OrderCount {
			return candidates[i].OrderCount > candidates[j].OrderCount
		}
		if candidates[i].PositionCount != candidates[j].PositionCount {
			return candidates[i].PositionCount > candidates[j].PositionCount
		}
		return strings.ToLower(candidates[i].Address) < strings.ToLower(candidates[j].Address)
	})
	return candidates
}

func preferredFunderCandidateIndex(candidates []FunderCandidate) (int, bool) {
	if len(candidates) == 1 {
		return 0, true
	}

	orderBackedIndex := -1
	for i, candidate := range candidates {
		if candidate.OrderCount == 0 {
			continue
		}
		if orderBackedIndex >= 0 {
			return 0, false
		}
		orderBackedIndex = i
	}
	if orderBackedIndex >= 0 {
		return orderBackedIndex, true
	}
	return 0, false
}
