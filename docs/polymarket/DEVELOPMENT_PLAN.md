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

### Phase 6: 自动构造与签名 V2 订单

前置条件：

- 必须核对官方 SDK 源码
- 必须确认以下问题：
  - V2 order struct 最终字段集合
  - `expiration` 是否仍参与签名
  - `taker`、`nonce`、`feeRateBps` 是否在 wire payload 中必须保留
  - `owner` 的真实来源
  - neg-risk 与普通 market 的 `verifyingContract` 选择逻辑

完成后再实现：

- `CreateOrder()`
- `SignOrder()`
- `CreateAndPostOrder()`

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

在开始写订单签名代码前，必须回答下面几个问题：

1. V2 最终签名结构到底保留哪些字段
2. `owner` 到底如何获得
3. L2 HMAC 的 canonical string 具体规则
4. neg-risk 市场下如何自动切换 `verifyingContract`
5. `builder` 字段是否允许全零默认值，以及何时必须携带

## 10. 下一步建议

按当前计划，下一步最合理的动作不是直接做下单，而是：

1. 先实现 `Phase 1 + Phase 2`
2. 然后完成 `Phase 3`
3. 再去核对官方 SDK 源码，决定 `Phase 5/6` 的 order schema

这样可以尽快得到一个稳定可用、且不容易因为 V2 文档冲突而返工的 Go 客户端基础版本。
