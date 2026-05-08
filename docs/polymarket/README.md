# Polymarket 文档整理

整理日期：2026-04-25

这个目录用于沉淀 `go-clob-client` 相关的 Polymarket CLOB API 官方资料来源、本地验证结论和 live 测试运行说明。这里不是对官网的逐字镜像，而是围绕 Go 客户端实现维护的本地摘要。

## 文件说明

这个目录当前只保留 `README.md`，用于记录已经验证过的 Polymarket CLOB API 结论、
live 测试入口和账户上下文注意事项。早期开发计划和官方文档摘要已删除，避免仓库继续
维护会过期的长篇镜像资料。

## 主要官方来源

- 文档首页: <https://docs.polymarket.com/>
- Trading Overview: <https://docs.polymarket.com/trading/overview>
- Authentication: <https://docs.polymarket.com/api-reference/authentication>
- Public Methods: <https://docs.polymarket.com/trading/clients/public>
- V2 Migration: <https://docs.polymarket.com/v2-migration>
- Builder Fees: <https://docs.polymarket.com/builders/fees>
- Rate Limits: <https://docs.polymarket.com/api-reference/rate-limits>
- Get Server Time: <https://docs.polymarket.com/api-reference/data/get-server-time>
- Get Order Book: <https://docs.polymarket.com/api-reference/market-data/get-order-book>
- Get CLOB Market Info: <https://docs.polymarket.com/api-reference/markets/get-clob-market-info>
- Post a New Order: <https://docs.polymarket.com/api-reference/trade/post-a-new-order>
- Get User Orders: <https://docs.polymarket.com/api-reference/trade/get-user-orders>
- Cancel Single Order: <https://docs.polymarket.com/api-reference/trade/cancel-single-order>

## 当前本地结论

- 新项目应直接按 CLOB V2 设计，不再以旧版 `clob-client` 为基线。
- 第一阶段先完成“公共行情 + L1/L2 认证 + 已签名订单提交/查询/撤单”。
- 本地代码已收敛为 CLOB V2-only，不再保留 V1 订单签名、payload 或 host 分流。
- V2 限价单与 market order 构造签名已按官方 SDK 源码落地，订单 typed-data 使用 `timestamp`、`metadata`、`builder`。
- 下单 payload 的 `owner` 在客户端持有 L2 凭证时会默认使用 API key；订单 maker/funder 可通过 `ClientConfig.FunderAddress` 配置，未配置时回退到 signer 地址。
- `ClientConfig.AutoDiscoverFunder` / `WithAutoDiscoverFunder()` 可在下单前自动聚合 `positions.proxyWallet` 与当前用户订单的 `maker_address`，并在候选唯一时自动回填 maker/funder。
- `ClientConfig.BuilderCode` / `BuilderCode` 参数会把 V2 订单标记为 builder order；market BUY 可通过 `UserUSDCBalance` 按官方 SDK 的 fee-aware 逻辑自动下调 amount。
- 当 `signatureType != EOA` 且既未显式提供 funder、自动发现也无法唯一确定时，本地会直接报错，避免把不明确的上下文交给服务端。
- 对多数直接在 Polymarket.com 使用的账户，`signatureType=2 (GNOSIS_SAFE)` 更符合实际账户上下文；`signatureType=0 (EOA)` 可能会把余额查成 `0`，即使页面上显示账户有资金。
- 余额查询和下单必须使用同一套账户上下文。若余额只能在 `signatureType=2` 下查到，下单通常也应使用 `2`，并配合正确的 proxy wallet / funder 地址。

## 当前代码进度

截至 2026-04-25，本地仓库已落地以下能力：

- 公共接口：
  - `GetOK`
  - `GetServerTime`
  - `GetOrderBook`
  - `GetPrice`
  - `GetMidpoint`
  - `GetSpread`
  - `GetTickSize`
  - `GetNegRisk`
  - `GetMarketByToken`
  - `GetCLOBMarketInfo`
  - `GetBuilderFeeRates`
- L1：
  - `BuildL1Headers`
  - `CreateAPIKey`
  - `DeriveAPIKey`
  - `CreateOrDeriveAPIKey`
- L2 / 账户：
  - `BuildL2Headers`
  - `GetBalanceAllowance`
  - `GetPositions`
  - `DiscoverFunder`
  - `WithDiscoveredFunder`
- L2 / 订单：
  - `BuildOrderV2TypedData`
  - `CreateOrder`
  - `CreateMarketOrder`
  - `CalculateMarketOrderPrice`
  - `SignOrderV2`
  - `CreatePostOrderRequest`
  - `CreatePostMarketOrderRequest`
  - `CreateAndPostOrder`
  - `CreateAndPostMarketOrder`
  - `ClientConfig.BuilderCode`
  - `WithBuilderCode`
  - `NormalizeBuilderCode`
  - `UserUSDCBalance` fee-aware market BUY amount adjustment
  - `GetOrderScoringStatus`
  - `GetUserOrders`
  - `GetOrder`
  - `CancelOrder`
  - `CancelOrders`
  - `CancelAllOrders`
  - `CancelMarketOrders`
  - `PostOrder`
  - `PostOrders`
  - `SendHeartbeat`

## 最新验证进展

截至当前，最新一轮 live 验证结论如下：

- `orders_integration_test.go` 已收敛为两个核心生产写入测试：
  - `TestIntegrationCreatePostAndCancelOrder`
  - `TestIntegrationCreatePostMarketBuyThenSellOrder`
- live integration 不再使用额外开关；设置好 `.env` 后，按 `-run` 精确选择目标测试。
- 默认交易 API host 是 `https://clob.polymarket.com`。
  - `POLYMARKET_HOST`、`POLYMARKET_PROXY_URL`、`POLYMARKET_SIGNATURE_TYPE`、`POLYMARKET_FUNDER_ADDRESS` 仍可显式覆盖默认值。
- funder 自动发现已落地：
  - `DiscoverFunder()`
  - `WithDiscoveredFunder()`
  - `ClientConfig.AutoDiscoverFunder`
  - `WithAutoDiscoverFunder()`
  - `EOA` 场景会直接推荐 signer 自身
  - proxy / safe 场景会聚合 `GetPositions()` 返回的 `proxyWallet` 与当前用户订单中的 `maker_address`
- builder 相关封装已落地：
  - `ClientConfig.BuilderCode`
  - `WithBuilderCode()`
  - `GetBuilderFeeRates()`
  - `CreateOrderParams.BuilderCode`
  - `CreateMarketOrderParams.BuilderCode`
  - `CreateMarketOrderParams.UserUSDCBalance`
  - `CreateMarketOrderParams.FeeInfo`
  - `CreateMarketOrderParams.BuilderTakerFeeBps`
- 对当前测试账户：
  - 不显式设置 `POLYMARKET_FUNDER_ADDRESS` 时，`signatureType=0` 下 `DiscoverFunder()` live 验证通过，推荐地址就是 signer 自身
  - 不显式设置 `POLYMARKET_FUNDER_ADDRESS` 时，`signatureType=1` / `2` 下 `DiscoverFunder()` live 验证通过，但当前没有发现到 proxy wallet / safe 候选，因此返回空候选结果而不是错误
- 已完成真实写入闭环验证：
  - GTC `BUY + postOnly` 挂单后立即 `CancelOrder`
  - Market BUY 成交后读取 `TakingAmount`，再按买到的 shares 执行 Market SELL
- Market order 测试默认只传 `tokenID / amount / side / tickSize`，`price` 仅作为可选保护价覆盖。

## 运行真实 L1 集成测试

仓库里已经提供了一个会真正访问 Polymarket 的 L1 集成测试：

- 测试名：`TestIntegrationCreateOrDeriveAPIKey`
- 文件：`auth_integration_test.go`

测试启动时会先尝试加载仓库根目录下的 `.env`。

运行前需要在 `.env` 或当前 shell 中配置：

- `POLYMARKET_PRIVATE_KEY=<your_private_key>`

可选环境变量：

- `POLYMARKET_HOST`
  - 默认 `https://clob.polymarket.com`
- `POLYMARKET_CHAIN_ID`
  - 默认 `137`
- `POLYMARKET_NONCE`
  - 默认 `0`
- `POLYMARKET_PROXY_URL`
  - 例如 `http://127.0.0.1:7897`
  - 不设置则直连
- `POLYMARKET_FUNDER_ADDRESS`
  - proxy wallet / safe 账户下单时的资金地址
  - 只做认证测试时可不设置
- `POLYMARKET_USE_SERVER_TIME`
  - 可选，填 `1` / `true` 或 `0` / `false`
  - live 集成测试默认使用服务端时间；显式填 `0` 或 `false` 才会退回本地 Unix 秒时间

建议运行方式：

```powershell
go test ./... -run TestIntegrationCreateOrDeriveAPIKey -count=1
```

推荐先编辑根目录 `.env`：

```dotenv
POLYMARKET_PRIVATE_KEY=0x...
POLYMARKET_PROXY_URL=http://127.0.0.1:7897
# POLYMARKET_USE_SERVER_TIME=true
```

注意：

- 这是一个真实线上认证测试
- 它会调用 `CreateOrDeriveAPIKey`
- 如果 nonce 未被使用过，可能会为该账户真实创建一组 API 凭证

## 运行真实 L2 余额集成测试

仓库里已经提供了一个会真正访问 Polymarket 的 L2 集成测试：

- 测试名：`TestIntegrationGetBalanceAllowance`
- 文件：`account_integration_test.go`

这个测试会：

1. 从 `.env` 加载配置
2. 使用私钥执行 `CreateOrDeriveAPIKey`
3. 调用 `GetBalanceAllowance`

需要的环境变量：

- `POLYMARKET_PRIVATE_KEY=<your_private_key>`

可选环境变量：

- `POLYMARKET_SIGNATURE_TYPE`
  - 客户端默认值仍为 `0`
  - `0=EOA`, `1=POLY_PROXY`, `2=GNOSIS_SAFE`, `3=POLY_1271`
  - 对多数直接在 Polymarket.com 使用的账户，建议显式设置为 `2`
  - 如果 `COLLATERAL` 余额返回 `0`，但页面里确认有资金，优先排查是否应改为 `2`
- `POLYMARKET_BALANCE_ASSET_TYPE`
  - 默认 `COLLATERAL`
  - 可选 `CONDITIONAL`
- `POLYMARKET_BALANCE_TOKEN_ID`
  - 当 `POLYMARKET_BALANCE_ASSET_TYPE=CONDITIONAL` 时必填
- `POLYMARKET_PROXY_URL`
  - 例如 `http://127.0.0.1:7897`
- `POLYMARKET_FUNDER_ADDRESS`
  - proxy wallet / safe 账户的资金地址；余额查询本身不需要，但建议与后续下单配置保持一致
- `POLYMARKET_USE_SERVER_TIME`
  - 可选，填 `1` / `true` 或 `0` / `false`
  - live 集成测试默认使用服务端时间；显式填 `0` 或 `false` 才会退回本地 Unix 秒时间

建议运行方式：

```powershell
go test ./... -run TestIntegrationGetBalanceAllowance -count=1
```

推荐 `.env` 片段：

```dotenv
POLYMARKET_PRIVATE_KEY=0x...
POLYMARKET_PROXY_URL=http://127.0.0.1:7897
POLYMARKET_SIGNATURE_TYPE=2
POLYMARKET_BALANCE_ASSET_TYPE=COLLATERAL
```

补充说明：

- `GetBalanceAllowance` 返回的 `balance` 是当前 Polymarket 账户上下文下的 collateral / conditional 资产余额，不是“任意钱包资产余额”。
- 如果账户是在 Polymarket.com 内创建或管理的，页面里展示给用户的钱包地址通常是 proxy wallet，不一定等于私钥直接对应的 EOA 地址。
- 目前仓库里已验证：同一账户在 `POLYMARKET_SIGNATURE_TYPE=0` 下余额可能为 `0`，切到 `POLYMARKET_SIGNATURE_TYPE=2` 后可查到正确余额。
- 后续真实下单时也应沿用同一个 `POLYMARKET_SIGNATURE_TYPE`；不要出现“查询余额用 2、下单再切回 0”的混用。

## 运行真实订单写入集成测试

`orders_integration_test.go` 现在只保留两个核心生产写入场景：

- `TestIntegrationCreatePostAndCancelOrder`
  - 创建一笔低价 `BUY + GTC + postOnly` 订单，拿到 `orderID` 后立即调用 `CancelOrder`。
- `TestIntegrationCreatePostMarketBuyThenSellOrder`
  - 执行 Market BUY，读取 BUY 响应的 `TakingAmount`，换算为 shares 后立即执行 Market SELL。

建议只用 `-run` 精确执行目标测试：

```powershell
go test ./... -run TestIntegrationCreatePostAndCancelOrder -count=1 -v
go test ./... -run TestIntegrationCreatePostMarketBuyThenSellOrder -count=1 -v
```

常用 `.env`：

```dotenv
POLYMARKET_PRIVATE_KEY=0x...
POLYMARKET_SIGNATURE_TYPE=2
POLYMARKET_FUNDER_ADDRESS=0x...
POLYMARKET_PROXY_URL=http://127.0.0.1:7897
```

GTC 挂单可选覆盖：

```dotenv
POLYMARKET_LIMIT_POST_TOKEN_ID=<token_id>
POLYMARKET_LIMIT_POST_SIZE=5
POLYMARKET_LIMIT_POST_PRICE=0.001
POLYMARKET_LIMIT_POST_TICK_SIZE=0.001
POLYMARKET_LIMIT_POST_NEG_RISK=false
```

Market BUY -> SELL 可选覆盖：

```dotenv
POLYMARKET_MARKET_TOKEN_ID=<token_id>
POLYMARKET_MARKET_BUY_AMOUNT=1
POLYMARKET_MARKET_BUY_PRICE=0.50
POLYMARKET_MARKET_SELL_PRICE=0.49
POLYMARKET_MARKET_BUY_TICK_SIZE=0.01
POLYMARKET_MARKET_BUY_NEG_RISK=false
```

注意：

- 这些测试会访问生产 Polymarket，并可能真实成交。
- Market order 默认 `FOK`，不传 `price` 时客户端会从盘口计算保护价。
- `BUY` 的 `amount` 是 USDC 金额；`SELL` 的 `amount` 是 shares 数量。BUY -> SELL 测试会自动从 BUY 响应的 `TakingAmount` 推导 SELL 数量。
