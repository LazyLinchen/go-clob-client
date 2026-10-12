package clobclient

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Address rules match ts-sdk 1462212 packages/client/src/wallet.ts:
// maker = account.wallet; signer = wallet for POLY_1271, owner EOA otherwise.
func TestSignatureTypesOrderMatrix(t *testing.T) {
	for _, signatureType := range []SignatureType{SignatureTypeEOA, SignatureTypePolyProxy, SignatureTypePolyGnosisSafe, SignatureTypePoly1271} {
		for _, version := range []ExchangeProtocolVersion{ExchangeProtocolV2, ExchangeProtocolV3} {
			for _, negRisk := range []bool{false, true} {
				for _, side := range []Side{SideBuy, SideSell} {
					for _, market := range []bool{false, true} {
						name := fmt.Sprintf("type%d/v%s/negRisk%t/%s/market%t", signatureType, version, negRisk, side, market)
						t.Run(name, func(t *testing.T) {
							c := newOrderBuilderTestClient(t)
							c.signatureType = signatureType
							maker := c.signer.Address()
							if signatureType == SignatureTypePolyProxy || signatureType == SignatureTypePolyGnosisSafe {
								maker = "0x1111111111111111111111111111111111111111"
								c.funderAddress = maker
							}
							if signatureType == SignatureTypePoly1271 {
								var err error
								maker, err = c.DepositWalletAddress()
								if err != nil {
									t.Fatal(err)
								}
							}
							var order *SignedOrderV2
							var err error
							if market {
								order, err = c.CreateMarketOrder(context.Background(), CreateMarketOrderParams{TokenID: "123", Amount: "10", Price: "0.5", TickSize: TickSizeCent, Side: side, NegRisk: negRisk, ProtocolVersion: version, Salt: "12345", TimestampMS: 1700000000000})
							} else {
								order, err = c.CreateOrder(context.Background(), CreateOrderParams{TokenID: "123", Size: "10", Price: "0.5", TickSize: TickSizeCent, Side: side, NegRisk: negRisk, ProtocolVersion: version, Salt: "12345", TimestampMS: 1700000000000})
							}
							if err != nil {
								t.Fatal(err)
							}
							expectedSigner := c.signer.Address()
							if signatureType == SignatureTypePoly1271 {
								expectedSigner = maker
							}
							if order.Maker != maker || order.Signer != expectedSigner || order.SignatureType != signatureType {
								t.Fatalf("wrong identity: maker=%s signer=%s type=%d", order.Maker, order.Signer, order.SignatureType)
							}
							exchange := "0xE111180000d2663C0091e4f400237545B87B996B"
							if negRisk {
								exchange = "0xe2222d279d744050d28e00520010520000310F59"
							}
							if version == ExchangeProtocolV3 {
								exchange = "0xe3333700cA9d93003F00f0F71f8515005F6c00Aa"
							}
							td, err := BuildOrderV2TypedData(order.OrderV2, 137, exchange)
							if signatureType == SignatureTypePoly1271 {
								td, err = buildPoly1271TypedData(order.OrderV2, 137, exchange)
							}
							if err != nil {
								t.Fatal(err)
							}
							td.Domain.Version = string(version)
							sig, err := hexutil.Decode(order.Signature)
							if err != nil {
								t.Fatal(err)
							}
							wantLen := 65
							if signatureType == SignatureTypePoly1271 {
								wantLen += 64 + len(orderTypeString) + 2
							}
							if len(sig) != wantLen {
								t.Fatalf("signature length = %d, want %d", len(sig), wantLen)
							}
							if signatureType == SignatureTypePoly1271 {
								domain, err := td.HashStruct("EIP712Domain", td.Domain.Map())
								if err != nil {
									t.Fatal(err)
								}
								contents, err := td.HashStruct("Order", td.Message["contents"].(apitypes.TypedDataMessage))
								if err != nil {
									t.Fatal(err)
								}
								if !bytes.Equal(sig[65:97], domain) || !bytes.Equal(sig[97:129], contents) || string(sig[129:len(sig)-2]) != orderTypeString || int(binary.BigEndian.Uint16(sig[len(sig)-2:])) != len(orderTypeString) {
									t.Fatal("invalid ERC-7739 envelope")
								}
							}
							digest, _, err := apitypes.TypedDataAndHash(td)
							if err != nil {
								t.Fatal(err)
							}
							sig = append([]byte(nil), sig[:65]...)
							if sig[64] != 27 && sig[64] != 28 {
								t.Fatalf("invalid recovery byte %d", sig[64])
							}
							sig[64] -= 27
							pub, err := crypto.SigToPub(digest, sig)
							if err != nil {
								t.Fatal(err)
							}
							if !strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), c.signer.Address()) {
								t.Fatal("signature does not recover owner EOA")
							}
							payload, err := order.Payload()
							if err != nil {
								t.Fatal(err)
							}
							if payload.SignatureType != signatureType || payload.Signature != order.Signature || payload.Maker != maker || payload.Signer != expectedSigner {
								t.Fatal("wire payload changed identity or signature")
							}
						})
					}
				}
			}
		}
	}
}

func TestSignatureTypesRejectInvalidConfiguration(t *testing.T) {
	for _, signatureType := range []SignatureType{-1, 4, SignatureTypePolyProxy, SignatureTypePolyGnosisSafe} {
		c := newOrderBuilderTestClient(t)
		c.signatureType = signatureType
		// Invalid enum values reach the signing validator with a valid maker.
		if signatureType < 0 || signatureType > 3 {
			c.funderAddress = c.signer.Address()
		}
		if _, err := c.CreateOrder(context.Background(), CreateOrderParams{TokenID: "123", Price: "0.5", Size: "10", Side: SideBuy, TickSize: TickSizeCent}); err == nil {
			t.Fatalf("type %d accepted invalid config or missing funder", signatureType)
		}
	}
}
