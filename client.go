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
)

const (
	DefaultHost      = "https://clob.polymarket.com"
	DefaultChainID   = 137
	DefaultTimeout   = 15 * time.Second
	defaultUserAgent = "go-clob-client/0.1"
)

var sharedTransport = newDefaultTransport()
var proxyTransportCache sync.Map

type ClientConfig struct {
	Host          string
	ChainID       int64
	HTTPClient    *http.Client
	Timeout       time.Duration
	UserAgent     string
	Signer        L1Signer
	ProxyURL      string
	UseServerTime bool
}

// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL       *url.URL
	chainID       int64
	httpClient    *http.Client
	userAgent     string
	signer        L1Signer
	useServerTime bool
}

func NewClient(cfg ClientConfig) (*Client, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = DefaultHost
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

	return &Client{
		baseURL:       baseURL,
		chainID:       chainID,
		httpClient:    httpClient,
		userAgent:     userAgent,
		signer:        cfg.Signer,
		useServerTime: cfg.UseServerTime,
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

func (c *Client) ChainID() int64 {
	return c.chainID
}

func (c *Client) Signer() L1Signer {
	return c.signer
}

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

func (c *Client) buildURL(endpoint string, query url.Values) string {
	u := *c.baseURL
	u.Path = joinURLPath(c.baseURL.Path, endpoint)
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
	var bodyReader io.Reader
	if body != nil {
		payload, err := encodeJSON(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(endpoint, query), bodyReader)
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
	req, err := c.newRequest(ctx, method, endpoint, query, body, headers)
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
	req, err := c.newRequest(ctx, method, endpoint, query, nil, headers)
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
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}
