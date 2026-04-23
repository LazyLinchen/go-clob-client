# Polymarket 文档整理

整理日期：2026-04-22

这个目录用于沉淀实现 `go-clob-client` 前必须确认的官方资料与落地计划。这里不是对官网的逐字镜像，而是围绕 Go 客户端实现做的本地摘要、归档和开发拆解。

## 文件说明

- `OFFICIAL_DOCS_SUMMARY.md`
  - 基于 Polymarket 官方文档整理的 CLOB V2 关键事实
  - 覆盖 API 分层、认证、签名类型、关键 REST 端点、V2 迁移变化、当前文档冲突点
- `DEVELOPMENT_PLAN.md`
  - `go-clob-client` 的开发计划
  - 包含范围、阶段拆解、建议目录结构、测试策略、风险和待确认事项

## 主要官方来源

- 文档首页: <https://docs.polymarket.com/>
- Trading Overview: <https://docs.polymarket.com/trading/overview>
- Authentication: <https://docs.polymarket.com/api-reference/authentication>
- Public Methods: <https://docs.polymarket.com/trading/clients/public>
- V2 Migration: <https://docs.polymarket.com/v2-migration>
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
- V1 与 V2 订单签名结构不兼容；本地代码通过 `ClientConfig.CLOBVersion` / `POLYMARKET_CLOB_VERSION` 显式切换。
- V2 限价单与 market order 构造签名已按官方 SDK 源码落地，V1 兼容分支保留 `taker`、`nonce`、`feeRateBps`。
- 下单 payload 的 `owner` 在客户端持有 L2 凭证时会默认使用 API key；订单 maker/funder 可通过 `ClientConfig.FunderAddress` 配置，未配置时回退到 signer 地址。
- proxy wallet / safe 的 funder 地址仍需要调用方显式提供；当 `signatureType != EOA` 且缺少 funder 时，本地会直接报错，避免服务端返回难排查的 `invalid signature`。
- 对多数直接在 Polymarket.com 使用的账户，`signatureType=2 (GNOSIS_SAFE)` 更符合实际账户上下文；`signatureType=0 (EOA)` 可能会把余额查成 `0`，即使页面上显示账户有资金。
- 余额查询和下单必须使用同一套账户上下文。若余额只能在 `signatureType=2` 下查到，下单通常也应使用 `2`，并配合正确的 proxy wallet / funder 地址。

## 当前代码进度

截至 2026-04-23，本地仓库已落地以下能力：

- 公共接口：
  - `GetOK`
  - `GetServerTime`
  - `GetOrderBook`
  - `GetPrice`
  - `GetMidpoint`
  - `GetSpread`
  - `GetTickSize`
  - `GetCLOBMarketInfo`
- L1：
  - `BuildL1Headers`
  - `CreateAPIKey`
  - `DeriveAPIKey`
  - `CreateOrDeriveAPIKey`
- L2 / 账户：
  - `BuildL2Headers`
  - `GetBalanceAllowance`
  - `GetPositions`
- L2 / 订单：
  - `BuildOrderV2TypedData`
  - `BuildOrderV1TypedData`
  - `CreateOrder`
  - `CreateMarketOrder`
  - `CalculateMarketOrderPrice`
  - `SignOrderV2`
  - `CreatePostOrderRequest`
  - `CreatePostMarketOrderRequest`
  - `CreateAndPostOrder`
  - `CreateAndPostMarketOrder`
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

当前仍未实现：

- proxy wallet / safe funder 自动发现
- builder code / builder fee 高级封装

## 运行真实 L1 集成测试

仓库里已经提供了一个会真正访问 Polymarket 的 L1 集成测试：

- 测试名：`TestIntegrationCreateOrDeriveAPIKey`
- 文件：`auth_integration_test.go`

测试启动时会先尝试加载仓库根目录下的 `.env`。

默认不会执行。只有在显式设置下面的环境变量后才会运行：

- `POLYMARKET_RUN_LIVE_AUTH_TEST=run-live`
- `POLYMARKET_PRIVATE_KEY=<your_private_key>`

可选环境变量：

- `POLYMARKET_HOST`
  - V2 默认 `https://clob-v2.polymarket.com`
  - V1 默认 `https://clob.polymarket.com`
- `POLYMARKET_CLOB_VERSION`
  - 默认 `v2`
  - 可选 `v1` / `v2`
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
POLYMARKET_RUN_LIVE_AUTH_TEST=run-live
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

- `POLYMARKET_RUN_LIVE_L2_TEST=run-live`
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
POLYMARKET_RUN_LIVE_L2_TEST=run-live
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

## 运行真实订单查询集成测试

仓库里还提供了一组默认只读的 live 订单集成测试：

- `TestIntegrationGetUserOrders`
- `TestIntegrationGetOrder`
- `TestIntegrationGetOrderScoringStatus`
- `TestIntegrationGetPositions`
- 文件：`orders_integration_test.go`

这些测试会：

1. 从 `.env` 加载配置
2. 使用私钥执行 `CreateOrDeriveAPIKey`
3. 调用只读订单 / 持仓接口

执行开关：

- `POLYMARKET_RUN_LIVE_ORDER_READ_TEST=run-live`

可选环境变量：

- `POLYMARKET_ORDER_MARKET`
- `POLYMARKET_ORDER_ASSET_ID`
- `POLYMARKET_ORDER_ID`
  - `GetOrder` / `GetOrderScoringStatus` 需要
- `POLYMARKET_POSITIONS_USER`
  - `GetPositions` 需要
- `POLYMARKET_POSITIONS_MARKET`
- `POLYMARKET_POSITIONS_EVENT_ID`
- `POLYMARKET_POSITIONS_LIMIT`

建议运行方式：

```powershell
go test ./... -run "TestIntegration(GetUserOrders|GetOrder|GetOrderScoringStatus|GetPositions)" -count=1
```

## 运行真实订单写入集成测试

仓库里还提供了一组显式 gated 的 live 写操作测试：

- `TestIntegrationSendHeartbeat`
- `TestIntegrationCancelOrder`
- `TestIntegrationCancelOrders`
- `TestIntegrationCancelMarketOrders`
- 文件：`orders_integration_test.go`

这些测试默认不会执行。只有显式设置下面开关后才会运行：

- `POLYMARKET_RUN_LIVE_ORDER_WRITE_TEST=run-live`

对应测试所需环境变量：

- `POLYMARKET_HEARTBEAT_ID`
  - `TestIntegrationSendHeartbeat` 需要
- `POLYMARKET_CANCEL_ORDER_ID`
  - `TestIntegrationCancelOrder` 需要
- `POLYMARKET_CANCEL_ORDER_IDS`
  - 逗号分隔，`TestIntegrationCancelOrders` 需要
- `POLYMARKET_CANCEL_MARKET`
- `POLYMARKET_CANCEL_ASSET_ID`
  - `TestIntegrationCancelMarketOrders` 至少需要其中一个

建议运行方式：

```powershell
go test ./... -run "TestIntegration(SendHeartbeat|CancelOrder|CancelOrders|CancelMarketOrders)" -count=1
```

注意：

- 这组测试会对真实订单状态产生影响
- 取消成功后，相关订单通常无法再用于重复测试

## 运行真实下单集成测试

仓库里提供了单独开关保护的真实下单测试：

- `TestIntegrationCreateAndPostMarketOrder`
- 文件：`orders_integration_test.go`

这个测试会构造、签名并提交一笔 V2 market order。默认不会执行，必须显式设置：

- `POLYMARKET_RUN_LIVE_POST_ORDER_TEST=run-live`

主机说明：

- `POLYMARKET_CLOB_VERSION=v2` 使用 V2 typed-data：`timestamp`、`metadata`、`builder`
- `POLYMARKET_CLOB_VERSION=v1` 使用 V1 typed-data：`taker`、`nonce`、`feeRateBps`
- host 与版本必须匹配；例如 V2 签名打到 V1 服务端可能出现 `feeRateBps` 解析错误，V1 签名打到 V2 服务端可能出现 `invalid signature`

必填环境变量：

- `POLYMARKET_POST_TOKEN_ID`
- `POLYMARKET_POST_AMOUNT`
  - `BUY` 时表示投入的 collateral 金额
  - `SELL` 时表示卖出的条件 token 数量
- `POLYMARKET_POST_SIDE`
  - `BUY` 或 `SELL`
- `POLYMARKET_POST_TICK_SIZE`
  - 可选值：`0.1`、`0.01`、`0.001`、`0.0001`

可选环境变量：

- `POLYMARKET_POST_PRICE`
  - 留空时会通过当前订单簿计算保护价格
- `POLYMARKET_POST_ORDER_TYPE`
  - 默认 `FOK`，避免价格不成交时留下挂单
- `POLYMARKET_POST_NEG_RISK`
  - `true` / `false`
- `POLYMARKET_FUNDER_ADDRESS`
  - `POLYMARKET_SIGNATURE_TYPE` 为 `1`、`2` 或 `3` 时必须设置，通常是 proxy wallet / safe 地址

建议运行方式：

```powershell
go test ./... -run TestIntegrationCreateAndPostMarketOrder -count=1
```

注意：

- 这是生产写入测试，可能真实成交。
- 默认 `FOK` 只降低挂单风险，不消除成交风险。
- 仅在你明确接受该 token、方向、金额和价格保护条件时开启。

## 运行真实全撤集成测试

`CancelAllOrders` 风险最高，因此单独使用更严格的开关：

- `TestIntegrationCancelAllOrders`
- `POLYMARKET_RUN_LIVE_CANCEL_ALL_TEST=run-live`

建议运行方式：

```powershell
go test ./... -run TestIntegrationCancelAllOrders -count=1
```

注意：

- 这会尝试撤销当前账户上下文下的全部可撤订单
- 除非你明确要验证全撤链路，否则不要开启
