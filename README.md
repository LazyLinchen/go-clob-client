# go-clob-client

[![CI](https://github.com/LazyLinchen/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/LazyLinchen/go-clob-client/actions/workflows/ci.yml)

`go-clob-client` 是 Polymarket CLOB API 的 Go 客户端库，包名为 `clobclient`。

已对照官方统一 SDK `Polymarket/ts-sdk` 补充订单精度、签名域及账户接口。
具体来源版本、兼容变化和未覆盖能力见 [统一 SDK 对照与迁移说明](docs/polymarket/unified-sdk-migration.md)。

本次新增 `TickSizeHalfCent`、`UpdateBalanceAllowance`、`GetOrdersScoringStatus`，
并支持通过 `ProtocolVersion: ExchangeProtocolV3` 显式签名协议 V2 position 的订单。
原有调用默认继续使用 Exchange V2 域；SDK 的版本、资产协议版本和签名域版本是不同概念。

## 安装

```powershell
go get github.com/LazyLinchen/go-clob-client
```

## 基本用法

```go
package main

import (
	"context"
	"fmt"
	"log"

	clobclient "github.com/LazyLinchen/go-clob-client"
)

func main() {
	ctx := context.Background()

	client, err := clobclient.NewClient(clobclient.ClientConfig{})
	if err != nil {
		log.Fatal(err)
	}

	serverTime, err := client.GetServerTime(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(serverTime)
}
```

## 私有接口

需要 L1/L2 鉴权的接口可以配置签名器和 API 凭证：

```go
signer, err := clobclient.NewPrivateKeySigner("0x...")
if err != nil {
	log.Fatal(err)
}

client, err := clobclient.NewClient(clobclient.ClientConfig{
	Signer: signer,
	Credentials: &clobclient.APICredentials{
		Key:        "...",
		Secret:     "...",
		Passphrase: "...",
	},
})
if err != nil {
	log.Fatal(err)
}
```

## POLY_1271 / Deposit Wallet

针对 2026 年后开通的新 API 用户（每用户一个 ERC-1967 proxy deposit wallet），库内置 CREATE2
地址派生与 ERC-7739 包裹签名，与官方 `@polymarket/clob-client-v2` 字节级一致。

```go
signer, err := clobclient.NewPrivateKeySigner("0x...") // 你的 EOA 私钥
if err != nil {
    log.Fatal(err)
}

client, err := clobclient.NewClient(clobclient.ClientConfig{
    ChainID:       137, // 137 = Polygon 主网，80002 = Amoy
    Signer:        signer,
    SignatureType: clobclient.SignatureTypePoly1271,
    Credentials:   &clobclient.APICredentials{Key: "...", Secret: "...", Passphrase: "..."},
})
if err != nil {
    log.Fatal(err)
}

// deposit wallet 地址自动从 EOA 派生（CREATE2），无需手填 FunderAddress
addr, _ := client.DepositWalletAddress()
fmt.Println("deposit wallet:", addr)

// 下单：maker / signer 字段自动设为 deposit wallet，签名走 ERC-7739 包裹
signedOrder, err := client.CreateOrder(ctx, clobclient.CreateOrderParams{
    TokenID: "...", Price: "0.5", Size: "10",
    Side: clobclient.SideBuy, TickSize: clobclient.TickSizeCent,
})
```

**链上前置条件（库不管，需提前完成）**：

1. Deposit wallet 已部署 —— 用 Polymarket 网页登录一次或 `@polymarket/builder-relayer-client`
   的 `WALLET-CREATE` 触发。
2. 三笔授权已设置（一次性）：`USDC→Exchange V2`、`USDC→CTF`、`CTF→Exchange V2` 的
   `setApprovalForAll`。授权必须由 deposit wallet 发起（通过 relayer `WALLET` 批次）。
3. Deposit wallet 中已存入 pUSD（买单）或对应 outcome token（卖单）。

签名类型 0/1/2（EOA / Polymarket Proxy / Gnosis Safe）行为不变，老账户继续可用。

## 行情、市场、成交与激励接口

公开行情（无需鉴权）：

- 单条：`GetOrderBook` / `GetPrice` / `GetMidpoint` / `GetSpread` / `GetTickSize` / `GetNegRisk`
- 批量：`GetOrderBooks` / `GetPrices` / `GetMidpoints` / `GetSpreads`（`GetPrices` 每项需带方向）

市场列表（游标分页，`next_cursor` 到达末页返回 `LTE=`，即常量 `MarketsEndCursor`）：

- `GetMarkets` / `GetSamplingMarkets`（完整 `Market`）
- `GetSimplifiedMarkets` / `GetSamplingSimplifiedMarkets`（精简 `SimplifiedMarket`）
- `GetMarket(conditionID)` 查询单个市场

成交历史（需 L2 鉴权）：

- `GetTrades(ctx, TradesParams{...})`，支持 `ID` / `MakerAddress` / `Market` / `AssetID` /
  `Before` / `After` 过滤与 `NextCursor` 分页；响应兼容裸数组与分页信封。

激励 / Rewards：

- `GetUserRewardPercentages`（L2）：各市场实时激励占比
- `GetUserEarnings(ctx, date, nextCursor)`（L2）：某日收益与各市场配置
- `GetCurrentRewards(ctx, sponsored, nextCursor)`：当前开启激励的市场配置列表

> 注：rewards 三个接口的路径均已对照官方 `@polymarket/clob-client-v2` 的 `endpoints.ts` 确认，
> `GetCurrentRewards` 与市场列表接口已通过线上只读调用验证。

## 测试

本地确定性测试会排除 `TestIntegration...` live 测试：

```powershell
go test ./... -run '^Test[^I]' -count=1
go build ./...
```

live 测试会访问真实 Polymarket 服务，部分订单测试可能真实成交。运行前请配置 `.env`，
并只用精确的 `-run TestIntegration...` 目标执行。
