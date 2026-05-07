# go-clob-client 开发计划

制定日期：2026-04-22

## 1. 目标

实现一个面向 Polymarket CLOB V2 的 Go 客户端，优先满足以下场景：

- 读取订单簿、价格、点差、市场参数
- 创建或派生 API 凭证
- 构造 L1/L2 认证头
- 查询订单、撤单、提交已签名订单
- 后续再支持自动构造和签名 V2 订单

## 2. 范围界定

### 2.1 第一阶段必须完成

- 公共行情接口
- L1 认证
- L2 认证
- 已签名订单提交
- 用户订单查询
- 单笔撤单
- 基础错误处理
- 基础测试
- 使用文档

### 2.2 第一阶段明确不做

- 自动构造并签名 V2 订单
- WebSocket
- Relayer
- Builder Program 专用能力
- Gasless 交易
- 全量 Gamma/Data API 封装

## 3. 建议交付顺序

### Phase 0: 文档基线

输出：

- `docs/polymarket/OFFICIAL_DOCS_SUMMARY.md`
- `docs/polymarket/DEVELOPMENT_PLAN.md`

目标：

- 固定实现边界
- 标记官方文档冲突点
- 明确哪些地方必须参考官方 SDK 源码

### Phase 1: 核心 HTTP Client

输出：

- 统一的 `Client` 结构
- base URL、chain ID、timeout、custom http client 配置
- 公共请求方法
- JSON 编解码
- API error 结构

验收标准：

- 能稳定发起 GET/POST/DELETE
- 能正确解析成功/失败响应

### Phase 2: Public Market Data

首批接口：

- `GetOK()`
- `GetServerTime()`
- `GetOrderBook(tokenID)`
- `GetPrice(tokenID, side)`
- `GetMidpoint(tokenID)`
- `GetSpread(tokenID)`
- `GetTickSize(tokenID)`
- `GetCLOBMarketInfo(conditionID)`

验收标准：

- 所有公共接口均无需 signer/creds 即可调用
- 返回结构体字段按官方文档映射

### Phase 3: L1 认证

能力：

- `CreateAPIKey()`
- `DeriveAPIKey()`
- `CreateOrDeriveAPIKey()`
- 生成 L1 header

关键实现点：

- `ClobAuthDomain`
- `ClobAuth` typed data
- `POLY_NONCE`
- 服务器时间同步

验收标准：

- 可以根据私钥生成 L1 请求头
- 可以完成 API 凭证的创建或派生

### Phase 4: L2 认证与订单管理

能力：

- 生成 L2 header
- `GetUserOrders()`
- `GetOrder(orderID)`
- `CancelOrder(orderID)`
- 视时间补充：
  - `CancelOrders(orderIDs)`
  - `CancelAllOrders()`
  - `CancelMarketOrders()`
  - `SendHeartbeat()`

关键实现点：

- HMAC canonical string 必须以官方 SDK 源码为准
- query string 与 body 必须按最终发送内容签名

验收标准：

- 已有凭证的用户可完成查询和撤单

### Phase 5: 已签名订单提交

能力：

- `PostOrder(signedOrderPayload)`
- 可选支持 `PostOrders()`

设计原则：

- 第一版只接受“已签名订单 payload”
- 暂不在库内自动计算 maker/taker amount、salt、签名

原因：

- 当前官方文档对 V2 order struct 存在冲突
- `owner` 字段获取方式仍需二次确认

验收标准：

- 只要外部提供的 payload 与官方接口兼容，库即可完成提交

### Phase 6: 自动构造与签名订单

已确认并落地的 V2 限价单规则：

- EIP-712 domain: `Polymarket CTF Exchange`, version `2`
- V2 签名字段：`salt`、`maker`、`signer`、`tokenId`、`makerAmount`、`takerAmount`、`side`、`signatureType`、`timestamp`、`metadata`、`builder`
- `expiration` 保留在 wire payload 中，但不参与 V2 typed-data 签名
- V2 限价单不再把 `taker`、`nonce`、`feeRateBps` 放入签名结构
- 普通 market 与 neg-risk market 通过不同 V2 exchange `verifyingContract` 区分

已实现：

- `CreateOrder()`
- `CreateMarketOrder()`
- `CalculateMarketOrderPrice()`
- `SignOrderV2()`
- `CreatePostOrderRequest()`
- `CreatePostMarketOrderRequest()`
- `CreateAndPostOrder()`
- `CreateAndPostMarketOrder()`
- `DiscoverFunder()`
- `WithDiscoveredFunder()`
- `ClientConfig.BuilderCode`
- `WithBuilderCode()`
- `GetBuilderFeeRates()`
- `UserUSDCBalance` fee-aware market BUY amount adjustment
- 下单 `owner` 留空时默认使用 L2 API key
- 客户端级 `FunderAddress` 配置，用于 proxy wallet / safe 资金地址
- 客户端级 `BuilderCode` 配置，用于默认把 V2 订单标记为 builder order
- 非 EOA `signatureType` 缺少 funder 时，客户端可通过 `AutoDiscoverFunder` 先尝试自动推导；仍无法唯一确定时本地直接报错，避免服务端 `invalid signature`
- funder 自动发现会返回候选与推荐值；无候选或多候选时仍要求调用方显式决定
- market BUY 在提供 `UserUSDCBalance` 时，会按官方 SDK 公式把 platform fee 和 builder taker fee 纳入余额保护

后续仍需补齐：

- 真实资金/授权下的 `CreateAndPostOrder -> CancelOrder` live 闭环结果

## 4. 建议目录结构

第一版建议保持扁平，避免过早拆包：

```text
go-clob-client/
├─ client.go
├─ auth.go
├─ market_data.go
├─ orders.go
├─ types.go
├─ errors.go
├─ signer.go
├─ doc.go
└─ docs/
   └─ polymarket/
```

如果后续补齐自动签名，再考虑增加：

```text
internal/eip712/
internal/hmacsign/
internal/httputil/
```

## 5. 关键数据模型

第一阶段建议先定义这些结构：

- `ClientConfig`
- `APICredentials`
- `L1Headers`
- `L2Headers`
- `OrderBook`
- `BookLevel`
- `PriceResponse`
- `SpreadResponse`
- `TickSizeResponse`
- `CLOBMarketInfo`
- `UserOrder`
- `UserOrdersResponse`
- `CancelOrderRequest`
- `CancelOrderResponse`
- `PostOrderRequest`
- `PostOrderResponse`

注意：

- 涉及金额、价格、token 数量的字段尽量先按 `string` 保留
- 不要过早转成 `float64`
- 下单相关整数建议保留字符串或大整数表示

## 6. 测试计划

### 单元测试

- L1 typed data 编码
- L1 签名输出格式
- L2 HMAC 输出
- query/body 签名一致性
- 请求体序列化
- API error 解析

### 集成测试

在具备测试凭证后补：

- `GetServerTime`
- `CreateOrDeriveAPIKey`
- `GetUserOrders`
- `CancelOrder`

### 测试注意事项

- 不在仓库中存放真实私钥和 API secret
- 通过环境变量注入测试凭证
- 区分只读测试与真实交易测试

## 7. 风险清单

### 风险 1: 官方文档与迁移文档不一致

影响：

- 容易把错误的 order schema 固化进代码

应对：

- 自动签名功能延期到源码核对之后

### 风险 2: L2 HMAC 规范在文档页不完整

影响：

- 请求签名错误，所有 L2 请求失效

应对：

- 以官方 SDK 源码为准补完本地实现

### 风险 3: `owner` 字段来源不明

影响：

- 无法稳定自动下单

应对：

- 第一版 `PostOrder` 先接受显式传入的 `owner`
- 在源码核对阶段确认是否能从 API 凭证或其他接口推导

### 风险 4: Proxy wallet / funder 处理复杂

影响：

- 对 Polymarket.com 账户支持不完整

应对：

- 第一版先支持显式传入 `signatureType` 和 `funderAddress`
- 自动推导 funder 放到后续阶段
- 实测上不要把 `signatureType=0 (EOA)` 当作 Polymarket.com 账户的安全默认值；常见账户更可能需要 `2 (GNOSIS_SAFE)`
- 余额查询与下单必须复用同一套 `signatureType` / funder 上下文，否则容易出现“能登录但余额为 0”或“余额正常但下单失败”

## 8. 开发里程碑

建议按下面节奏推进：

1. 文档整理完成
2. 核心 HTTP client 完成
3. Public methods 完成
4. L1 完成
5. L2 查询/撤单完成
6. 已签名订单提交完成
7. 自动签名能力补齐

## 9. 当前待确认问题

订单签名第一版已按官方 SDK 源码完成。当前仍需确认下面几个问题：

1. funder / proxy wallet 是否能从公开接口自动推导
2. 真实 `PostOrder` 在不同 `signatureType` / funder 组合下的服务端行为
3. 真实余额/allowance 下 `CreateAndPostOrder -> CancelOrder` 的完整服务端闭环

## 10. 下一步建议

截至 2026-04-25，本地代码已覆盖 `Phase 1` 到 `Phase 5`，并完成 `Phase 6` 的 V2 自动构造、签名和提交第一版：

- `Phase 1`: 核心 HTTP client
- `Phase 2`: Public methods
- `Phase 3`: L1 auth / API key
- `Phase 4`: L2 headers、余额查询、订单查询/撤单、全撤、按市场撤单、heartbeat、order scoring
- `Phase 5`: `PostOrder` / `PostOrders` 已支持“提交外部已签名 payload”
- `Phase 6`: `CreateOrder` / `CreateMarketOrder` / `SignOrderV2` / `CreateAndPostOrder` / `CreateAndPostMarketOrder` 已收敛为 V2-only 构造、签名和提交链路
- V1 订单签名、payload 和 host 分流已移除，避免继续维护即将淘汰的 schema 分支

补充进展：

- live integration 不再使用额外开关；当前通过 `go test -run ...` 精确选择要访问生产环境的测试。
- `orders_integration_test.go` 已收敛为两个核心生产写入场景：
  - `TestIntegrationCreatePostAndCancelOrder`
  - `TestIntegrationCreatePostMarketBuyThenSellOrder`
- 默认使用生产 host，并支持自动发现活跃 token / tick size / neg-risk / 默认下单参数；也可通过 `.env` 显式指定 token、amount、price 等参数。
- funder 自动发现已落地：
  - `DiscoverFunder()`
  - `WithDiscoveredFunder()`
  - `ClientConfig.AutoDiscoverFunder`
  - `WithAutoDiscoverFunder()`
- builder code / builder fee 封装已落地：
  - `ClientConfig.BuilderCode`
  - `WithBuilderCode()`
  - `NormalizeBuilderCode()`
  - `GetBuilderFeeRates()`
  - `CreateOrderParams.BuilderCode`
  - `CreateMarketOrderParams.BuilderCode`
  - `CreateMarketOrderParams.UserUSDCBalance`
  - `CreateMarketOrderParams.FeeInfo`
  - `CreateMarketOrderParams.BuilderTakerFeeBps`
- 最新 live 结论：
  - `.env` 里原先的 `POLYMARKET_PROXY_URL=http://127.0.0.1` 缺少端口，会导致真实请求超时；当前本机验证通过的代理地址是 `http://127.0.0.1:7897`
  - 修正代理后，`CreateOrDeriveAPIKey()` 已通过
  - 对当前测试账户，在不显式设置 `FunderAddress` 时：
    - `signatureType=0` 的 `DiscoverFunder()` 返回 signer 自身
    - `signatureType=1` / `2` 均未发现 proxy wallet / safe 候选
  - `CreateAndPostOrder -> CancelOrder` 已完成真实 GTC 挂单和撤单闭环。
  - `CreateAndPostMarketOrder` 已完成真实 Market BUY，再用 BUY 响应 `TakingAmount` 作为 shares 执行 Market SELL。
后续最合理的动作变为：

1. 继续扩大 funder 自动发现的候选来源和命中率，减少调用方在 proxy / safe 场景下手工传参
2. 补齐更多订单/账户接口的 live 验证，但保持生产写入测试数量受控
3. 修正和验证 market order 价格计算、价格区间校验等边界逻辑

这样可以继续扩大可用接口面，同时保持真实下单测试范围清晰。
