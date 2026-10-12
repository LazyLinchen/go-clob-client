package clobclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func TestUnifiedHalfCentTick(t *testing.T) {
	c := newOrderBuilderTestClient(t)
	for _, side := range []Side{SideBuy, SideSell} {
		o, err := c.CreateOrder(context.Background(), CreateOrderParams{TokenID: "123", Price: "0.505", Size: "1.23", Side: side, TickSize: TickSize("0.005")})
		if err != nil {
			t.Fatal(err)
		}
		maker, taker := "621150", "1230000"
		if side == SideSell {
			maker, taker = taker, maker
		}
		if o.MakerAmount != maker || o.TakerAmount != taker {
			t.Fatalf("amounts = %s/%s", o.MakerAmount, o.TakerAmount)
		}
	}
}

func TestUnifiedSafeSalt(t *testing.T) {
	for _, salt := range []string{"0", "9007199254740991", "9007199254740992", "-1", "bad"} {
		_, err := (SignedOrderV2{OrderV2: OrderV2{Salt: salt}}).Payload()
		wantErr := salt != "0" && salt != "9007199254740991"
		if (err != nil) != wantErr {
			t.Errorf("salt %s: %v", salt, err)
		}
	}
}

func TestUnifiedPostOnly(t *testing.T) {
	c := newOrderBuilderTestClient(t)
	postOnly := true
	for _, typ := range []string{"", "GTC", "GTD", "FOK", "FAK", "invalid"} {
		_, err := c.normalizePostOrderRequest(PostOrderRequest{Order: json.RawMessage(`{}`), Owner: "key", OrderType: typ, PostOnly: &postOnly})
		wantErr := typ == "FOK" || typ == "FAK" || typ == "invalid"
		if (err != nil) != wantErr {
			t.Errorf("type %q: %v", typ, err)
		}
	}
}

func TestUnifiedExchangeV3(t *testing.T) {
	for _, sigType := range []SignatureType{SignatureTypeEOA, SignatureTypePoly1271} {
		c := newOrderBuilderTestClient(t)
		c.signatureType = sigType
		params := CreateOrderParams{TokenID: "123", Price: "0.5", Size: "10", Side: SideBuy, TickSize: TickSizeCent, Salt: "12345", TimestampMS: 1700000000000, ProtocolVersion: ExchangeProtocolV3}
		o, err := c.CreateOrder(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		td, err := BuildOrderV2TypedData(o.OrderV2, 137, "0xe3333700cA9d93003F00f0F71f8515005F6c00Aa")
		if sigType == SignatureTypePoly1271 {
			td, err = buildPoly1271TypedData(o.OrderV2, 137, "0xe3333700cA9d93003F00f0F71f8515005F6c00Aa")
		}
		if err != nil {
			t.Fatal(err)
		}
		td.Domain.Version = "3"
		digest, _, err := apitypes.TypedDataAndHash(td)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := hexutil.Decode(o.Signature)
		if err != nil {
			t.Fatal(err)
		}
		if sigType == SignatureTypePoly1271 {
			domain, err := td.HashStruct("EIP712Domain", td.Domain.Map())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sig[65:97], []byte(domain)) {
				t.Fatal("wrong wrapped domain")
			}
		}
		sig = append([]byte(nil), sig[:65]...)
		sig[64] -= 27
		pub, err := crypto.SigToPub(digest, sig)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), c.signer.Address()) {
			t.Fatal("signature uses wrong exchange domain")
		}
		params.ProtocolVersion = "4"
		if _, err := c.CreateOrder(context.Background(), params); err == nil {
			t.Fatal("unsupported version accepted")
		}
	}
}

func unifiedHTTPClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c := newOrderBuilderTestClient(t)
	client, err := NewClient(ClientConfig{Host: s.URL, Signer: c.signer, SignatureType: SignatureTypePoly1271, Credentials: &APICredentials{Key: "test-key", Secret: "c2VjcmV0", Passphrase: "test-pass"}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestUnifiedUpdateBalanceAllowance(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var paths []string
		c := unifiedHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			if r.Method != "GET" || r.URL.Query().Get("token_id") != "123" || r.URL.Query().Get("signature_type") != "3" || r.Header.Get("POLY_SIGNATURE") == "" {
				t.Error("invalid authenticated request")
			}
			if fail {
				http.Error(w, "refresh failed", 500)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/update") {
				w.WriteHeader(204)
				return
			}
			io.WriteString(w, `{"balance":"42","allowances":{"exchange":"99"}}`)
		})
		got, err := c.UpdateBalanceAllowance(context.Background(), BalanceAllowanceParams{AssetType: AssetTypeConditional, TokenID: "123"})
		if fail {
			if err == nil || len(paths) != 1 {
				t.Fatalf("failed refresh: %v, %v", paths, err)
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if got.Balance != "42" || !reflect.DeepEqual(paths, []string{"/balance-allowance/update", "/balance-allowance"}) {
				t.Fatalf("response = %+v, paths = %v", got, paths)
			}
		}
	}
}

func TestUnifiedOrdersScoring(t *testing.T) {
	c := unifiedHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/orders-scoring" || string(body) != `["one","two"]` || r.Header.Get("POLY_SIGNATURE") == "" {
			t.Errorf("invalid request %s %s %s", r.Method, r.URL.Path, body)
		}
		io.WriteString(w, `{"one":true,"two":false}`)
	})
	got, err := c.GetOrdersScoringStatus(context.Background(), []string{" one ", "two", "one"})
	if err != nil {
		t.Fatal(err)
	}
	if !got["one"] || got["two"] || len(got) != 2 {
		t.Fatal(got)
	}
	if _, err := c.GetOrdersScoringStatus(context.Background(), nil); err == nil {
		t.Fatal("empty IDs accepted")
	}
}

func TestUnifiedPostingSignedOrder(t *testing.T) {
	c := unifiedHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Order struct {
				Salt json.Number `json:"salt"`
			} `json:"order"`
			Owner     string `json:"owner"`
			OrderType string `json:"orderType"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(body), `"salt":12345`) || payload.Owner != "test-key" || payload.OrderType != "GTC" {
			t.Errorf("unexpected payload: %s", body)
		}
		want, err := buildPolyHMACSignature("c2VjcmV0", mustTimestamp(t, r.Header.Get("POLY_TIMESTAMP")), r.Method, r.URL.Path, body)
		if err != nil || r.Header.Get("POLY_SIGNATURE") != want {
			t.Error("body and HMAC differ")
		}
		io.WriteString(w, `{"success":true,"orderID":"id"}`)
	})
	o, err := c.CreateOrder(context.Background(), CreateOrderParams{TokenID: "123", Price: "0.5", Size: "10", Side: SideBuy, TickSize: TickSizeCent, Salt: "12345"})
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range []any{o, *o} {
		response, err := c.PostOrder(context.Background(), PostOrderRequest{Order: order})
		if err != nil || !response.Success {
			t.Fatalf("post: %+v %v", response, err)
		}
	}
	var nilOrder *SignedOrderV2
	if _, err := c.PostOrder(context.Background(), PostOrderRequest{Order: nilOrder}); err == nil {
		t.Fatal("nil signed order accepted")
	}
	o.Salt = "9007199254740992"
	if _, err := c.PostOrder(context.Background(), PostOrderRequest{Order: o}); err == nil {
		t.Fatal("unsafe salt accepted")
	}
}

func mustTimestamp(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUnifiedValidationAndFailures(t *testing.T) {
	ctx := context.Background()
	c := newOrderBuilderTestClient(t)
	for _, params := range []BalanceAllowanceParams{{}, {AssetType: AssetTypeConditional}} {
		if _, err := c.UpdateBalanceAllowance(ctx, params); err == nil {
			t.Fatal("invalid balance query accepted")
		}
	}
	if _, err := c.UpdateBalanceAllowance(ctx, BalanceAllowanceParams{AssetType: AssetTypeCollateral}); err == nil {
		t.Fatal("missing auth accepted")
	}
	if _, err := c.GetOrdersScoringStatus(ctx, []string{"id"}); err == nil {
		t.Fatal("missing auth accepted")
	}
	if _, err := c.CreatePostOrderRequest(ctx, CreatePostOrderRequestParams{OrderType: OrderTypeFOK, PostOnly: true}); err == nil || !strings.Contains(err.Error(), "post-only") {
		t.Fatalf("post-only limit: %v", err)
	}
	if _, err := c.CreatePostMarketOrderRequest(ctx, CreatePostMarketOrderRequestParams{PostOnly: true}); err == nil || !strings.Contains(err.Error(), "post-only") {
		t.Fatalf("post-only market: %v", err)
	}
	c.chainID = 80002
	if _, err := c.resolveExchangeV2Address(SignOrderV2Options{ProtocolVersion: ExchangeProtocolV3}); err == nil {
		t.Fatal("unverified chain deployment accepted")
	}
	if _, err := c.resolveExchangeV2Address(SignOrderV2Options{ProtocolVersion: "bad", ExchangeAddress: polygonExchangeV3}); err == nil {
		t.Fatal("invalid version accepted")
	}
	if _, err := c.resolveExchangeV2Address(SignOrderV2Options{ProtocolVersion: ExchangeProtocolV3, ExchangeAddress: "bad"}); err == nil {
		t.Fatal("invalid exchange accepted")
	}
	if got, err := c.resolveExchangeV2Address(SignOrderV2Options{ProtocolVersion: ExchangeProtocolV3, ExchangeAddress: polygonExchangeV3}); err != nil || got != polygonExchangeV3 {
		t.Fatalf("custom exchange: %s %v", got, err)
	}
	for _, response := range []string{`not-json`, `{"error":"failed"}`} {
		c := unifiedHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(response, "{") {
				w.WriteHeader(500)
			}
			io.WriteString(w, response)
		})
		if _, err := c.GetOrdersScoringStatus(ctx, []string{"id"}); err == nil {
			t.Fatal("invalid scoring response accepted")
		}
	}
}

func TestUnifiedGeneratedSalt(t *testing.T) {
	for i := 0; i < 32; i++ {
		salt, err := generateOrderSalt()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := (SignedOrderV2{OrderV2: OrderV2{Salt: salt}}).Payload(); err != nil {
			t.Fatal(err)
		}
	}
}
