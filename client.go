package clobclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// DefaultHost 是当前 V2 交易 API 的默认主机地址。
	DefaultHost = "https://clob-v2.polymarket.com"
	// ProductionHost 是 Polymarket CLOB 生产主机地址；实际订单 schema 取决于服务端当前版本。
	ProductionHost = "https://clob.polymarket.com"
	// DefaultDataHost 是公开数据 API 的默认主机地址。
	DefaultDataHost = "https://data-api.polymarket.com"
	// DefaultChainID 是 Polymarket 当前默认使用的链 ID。
	DefaultChainID = 137
	// DefaultTimeout 是内置 HTTP 客户端的默认超时时间。
	DefaultTimeout   = 15 * time.Second
	defaultUserAgent = "go-clob-client/0.1"
)

// CLOBVersion 表示 Polymarket CLOB 订单签名和 payload schema 版本。
type CLOBVersion string

const (
	// CLOBVersionV1 使用旧版 CTF Exchange 订单结构。
	CLOBVersionV1 CLOBVersion = "v1"
	// CLOBVersionV2 使用当前 CTF Exchange V2 订单结构。
	CLOBVersionV2 CLOBVersion = "v2"
)

var sharedTransport = newDefaultTransport()
var proxyTransportCache sync.Map

// ClientConfig 定义创建客户端时可选的基础配置。
type ClientConfig struct {
	// Host 指定交易 API 的根地址，留空时使用 DefaultHost。
	Host string
	// DataHost 指定公开数据 API 的根地址，留空时使用 DefaultDataHost。
	DataHost string
	// ChainID 用于 L1 签名中的 EIP-712 域信息，留空时使用 DefaultChainID。
	ChainID int64
	// HTTPClient 允许调用方注入自定义 HTTP 客户端。
	HTTPClient *http.Client
	// Credentials 是用于 L2 鉴权的 API 凭证。
	Credentials *APICredentials
	// Timeout 会覆盖默认 HTTP 客户端的超时配置。
	Timeout time.Duration
	// UserAgent 指定请求头中的 User-Agent，留空时使用默认值。
	UserAgent string
	// Signer 是用于 L1 鉴权和地址识别的签名器。
	Signer L1Signer
	// ProxyURL 为请求配置显式代理地址。
	ProxyURL string
	// UseServerTime 表示签名时优先读取服务端时间，减少本地时钟漂移影响。
	UseServerTime bool
	// SignatureType 指定账户相关接口使用的签名类型枚举值。
	SignatureType SignatureType
	// FunderAddress 是实际持有资金的地址；留空时订单 maker 默认使用 signer 地址。
	FunderAddress string
	// CLOBVersion 控制订单签名和 payload schema，留空时默认使用 V2。
	CLOBVersion CLOBVersion
}

// Client 是 Polymarket CLOB API 的客户端，可安全地被多个 goroutine 并发复用。
type Client struct {
	baseURL       *url.URL
	dataBaseURL   *url.URL
	chainID       int64
	httpClient    *http.Client
	userAgent     string
	signer        L1Signer
	creds         *APICredentials
	useServerTime bool
	signatureType SignatureType
	funderAddress string
	clobVersion   CLOBVersion
}

// NewClient 根据配置创建一个可复用的 API 客户端。
func NewClient(cfg ClientConfig) (*Client, error) {
	clobVersion, err := normalizeCLOBVersion(cfg.CLOBVersion)
	if err != nil {
		return nil, err
	}

	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = defaultHostForCLOBVersion(clobVersion)
	}

	baseURL, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid host %q: expected absolute http(s) URL", host)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported host scheme %q", baseURL.Scheme)
	}

	dataHost := strings.TrimSpace(cfg.DataHost)
	if dataHost == "" {
		dataHost = DefaultDataHost
	}
	dataBaseURL, err := url.Parse(dataHost)
	if err != nil {
		return nil, fmt.Errorf("parse data host: %w", err)
	}
	if dataBaseURL.Scheme == "" || dataBaseURL.Host == "" {
		return nil, fmt.Errorf("invalid data host %q: expected absolute http(s) URL", dataHost)
	}
	if dataBaseURL.Scheme != "http" && dataBaseURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported data host scheme %q", dataBaseURL.Scheme)
	}

	chainID := cfg.ChainID
	if chainID == 0 {
		chainID = DefaultChainID
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	defaultTransport, err := transportForProxy(cfg.ProxyURL)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   timeout,
			Transport: defaultTransport,
		}
	} else {
		cloned := *httpClient
		if cfg.Timeout > 0 && cloned.Timeout != timeout {
			cloned.Timeout = timeout
		}
		if cfg.ProxyURL != "" {
			transport, err := cloneTransportWithProxy(cloned.Transport, cfg.ProxyURL)
			if err != nil {
				return nil, err
			}
			cloned.Transport = transport
		} else if cloned.Transport == nil {
			cloned.Transport = defaultTransport
		}
		httpClient = &cloned
	}

	userAgent := cfg.UserAgent
	if strings.TrimSpace(userAgent) == "" {
		userAgent = defaultUserAgent
	}

	funderAddress := strings.TrimSpace(cfg.FunderAddress)
	if funderAddress != "" {
		if !common.IsHexAddress(funderAddress) {
			return nil, fmt.Errorf("invalid funder address %q", funderAddress)
		}
		funderAddress = common.HexToAddress(funderAddress).Hex()
	}

	return &Client{
		baseURL:       baseURL,
		dataBaseURL:   dataBaseURL,
		chainID:       chainID,
		httpClient:    httpClient,
		userAgent:     userAgent,
		signer:        cfg.Signer,
		creds:         normalizedCredentials(cfg.Credentials),
		useServerTime: cfg.UseServerTime,
		signatureType: cfg.SignatureType,
		funderAddress: funderAddress,
		clobVersion:   clobVersion,
	}, nil
}

// BaseURL 返回交易 API 的根地址。
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// DataBaseURL 返回公开数据 API 的根地址。
func (c *Client) DataBaseURL() string {
	return c.dataBaseURL.String()
}

// ChainID 返回当前客户端用于签名的链 ID。
func (c *Client) ChainID() int64 {
	return c.chainID
}

// Signer 返回当前客户端配置的 L1 签名器。
func (c *Client) Signer() L1Signer {
	return c.signer
}

// Credentials 返回当前客户端持有的 L2 API 凭证副本。
func (c *Client) Credentials() *APICredentials {
	return normalizedCredentials(c.creds)
}

// WithCredentials 返回一个共享底层配置、但使用新凭证的客户端副本。
func (c *Client) WithCredentials(creds *APICredentials) *Client {
	cloned := *c
	cloned.creds = normalizedCredentials(creds)
	return &cloned
}

// FunderAddress 返回订单 maker 默认使用的资金地址；为空表示回退到 signer 地址。
func (c *Client) FunderAddress() string {
	return c.funderAddress
}

// WithFunderAddress 返回一个共享底层配置、但使用新资金地址的客户端副本。
func (c *Client) WithFunderAddress(funderAddress string) (*Client, error) {
	funderAddress = strings.TrimSpace(funderAddress)
	if funderAddress != "" {
		if !common.IsHexAddress(funderAddress) {
			return nil, fmt.Errorf("invalid funder address %q", funderAddress)
		}
		funderAddress = common.HexToAddress(funderAddress).Hex()
	}

	cloned := *c
	cloned.funderAddress = funderAddress
	return &cloned, nil
}

// SignatureType 返回当前客户端使用的签名类型。
func (c *Client) SignatureType() SignatureType {
	return c.signatureType
}

// CLOBVersion 返回当前客户端默认使用的订单 schema 版本。
func (c *Client) CLOBVersion() CLOBVersion {
	return c.clobVersion
}

// CloseIdleConnections 主动关闭底层 HTTP 连接池中的空闲连接。
func (c *Client) CloseIdleConnections() {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func newDefaultTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 128
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second
	transport.ForceAttemptHTTP2 = true
	return transport
}

// transportForProxy 为指定代理地址返回可复用的传输层实例。
func transportForProxy(rawProxyURL string) (*http.Transport, error) {
	rawProxyURL = strings.TrimSpace(rawProxyURL)
	if rawProxyURL == "" {
		return sharedTransport, nil
	}

	if cached, ok := proxyTransportCache.Load(rawProxyURL); ok {
		return cached.(*http.Transport), nil
	}

	proxyURL, err := parseProxyURL(rawProxyURL)
	if err != nil {
		return nil, err
	}

	transport := newDefaultTransport()
	transport.Proxy = http.ProxyURL(proxyURL)

	actual, _ := proxyTransportCache.LoadOrStore(rawProxyURL, transport)
	return actual.(*http.Transport), nil
}

// cloneTransportWithProxy 基于已有 transport 克隆一个带代理配置的新实例。
func cloneTransportWithProxy(existing http.RoundTripper, rawProxyURL string) (*http.Transport, error) {
	proxyURL, err := parseProxyURL(rawProxyURL)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return transportForProxy(rawProxyURL)
	}

	transport, ok := existing.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("proxy requires *http.Transport, got %T", existing)
	}

	cloned := transport.Clone()
	cloned.Proxy = http.ProxyURL(proxyURL)
	return cloned, nil
}

// parseProxyURL 校验并解析代理地址。
func parseProxyURL(rawProxyURL string) (*url.URL, error) {
	proxyURL, err := url.Parse(strings.TrimSpace(rawProxyURL))
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}
	if proxyURL.Scheme == "" || proxyURL.Host == "" {
		return nil, fmt.Errorf("invalid proxy URL %q: expected absolute URL", rawProxyURL)
	}
	return proxyURL, nil
}

func normalizeCLOBVersion(version CLOBVersion) (CLOBVersion, error) {
	normalized := CLOBVersion(strings.ToLower(strings.TrimSpace(string(version))))
	if normalized == "" {
		return CLOBVersionV2, nil
	}
	switch normalized {
	case CLOBVersionV1, CLOBVersionV2:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported CLOB version %q", version)
	}
}

func defaultHostForCLOBVersion(version CLOBVersion) string {
	if version == CLOBVersionV1 {
		return ProductionHost
	}
	return DefaultHost
}

func (c *Client) buildURL(endpoint string, query url.Values) string {
	return c.buildURLFromBase(c.baseURL, endpoint, query)
}

func (c *Client) buildDataURL(endpoint string, query url.Values) string {
	return c.buildURLFromBase(c.dataBaseURL, endpoint, query)
}

func (c *Client) buildURLFromBase(base *url.URL, endpoint string, query url.Values) string {
	u := *c.baseURL
	if base != nil {
		u = *base
	}
	u.Path = joinURLPath(c.baseURL.Path, endpoint)
	if base != nil {
		u.Path = joinURLPath(base.Path, endpoint)
	}
	u.RawQuery = ""
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func joinURLPath(basePath, endpoint string) string {
	basePath = strings.TrimRight(basePath, "/")
	endpoint = strings.TrimLeft(endpoint, "/")

	switch {
	case basePath == "" && endpoint == "":
		return "/"
	case basePath == "":
		return "/" + endpoint
	case endpoint == "":
		return basePath
	default:
		return basePath + "/" + endpoint
	}
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	endpoint string,
	query url.Values,
	body any,
	headers http.Header,
) (*http.Request, error) {
	return c.newRequestToBase(ctx, c.baseURL, method, endpoint, query, body, headers)
}

func (c *Client) newRequestToBase(
	ctx context.Context,
	base *url.URL,
	method string,
	endpoint string,
	query url.Values,
	body any,
	headers http.Header,
) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := encodeJSON(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.buildURLFromBase(base, endpoint, query), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return req, nil
}

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	endpoint string,
	query url.Values,
	body any,
	headers http.Header,
	dest any,
) error {
	return c.doJSONBase(ctx, c.baseURL, method, endpoint, query, body, headers, dest)
}

func (c *Client) doJSONBase(
	ctx context.Context,
	base *url.URL,
	method string,
	endpoint string,
	query url.Values,
	body any,
	headers http.Header,
	dest any,
) error {
	req, err := c.newRequestToBase(ctx, base, method, endpoint, query, body, headers)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, req.URL.Redacted(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(req, resp)
	}

	if dest == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) doBytes(
	ctx context.Context,
	method string,
	endpoint string,
	query url.Values,
	headers http.Header,
) ([]byte, error) {
	req, err := c.newRequestToBase(ctx, c.baseURL, method, endpoint, query, nil, headers)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, req.URL.Redacted(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, newAPIError(req, resp)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return payload, nil
}

func encodeJSON(v any) ([]byte, error) {
	switch body := v.(type) {
	case nil:
		return nil, nil
	case []byte:
		return body, nil
	case json.RawMessage:
		return []byte(body), nil
	}

	// 统一关闭 HTML 转义，避免签名和透传 JSON 时出现不必要的内容变化。
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}
