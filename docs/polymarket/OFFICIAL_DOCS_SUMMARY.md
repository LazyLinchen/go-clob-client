# Polymarket 官方文档整理摘要

整理日期：2026-04-22

## 1. 总览

Polymarket 当前对外主要有三套 API：

- Gamma API: `https://gamma-api.polymarket.com`
  - 市场、事件、标签、搜索、公开资料
- Data API: `https://data-api.polymarket.com`
  - 持仓、成交、活动、排行榜等数据
- CLOB API: `https://clob.polymarket.com`
  - 订单簿、价格、点差、下单、撤单、交易相关能力

对 `go-clob-client` 来说，核心目标是 CLOB API。

官方说明 CLOB 是“撮合离线、结算上链”的混合式架构：

- 订单在本地签名
- 撮合在 CLOB 后端完成
- 成交在 Polygon 上原子结算
- 交易是非托管的

参考：

- <https://docs.polymarket.com/api-reference>
- <https://docs.polymarket.com/trading/overview>

## 2. V2 当前状态

根据 2026-04-22 可见的官方迁移文档：

- CLOB V2 的正式切换时间写为 `2026-04-28 11:00 UTC` 左右
- 切换前测试地址是 `https://clob-v2.polymarket.com`
- 切换后生产地址仍然是 `https://clob.polymarket.com`
- 官方 SDK 已切换到新包名：
  - TypeScript: `@polymarket/clob-client-v2`
  - Python: `py-clob-client-v2`

V2 的主要变化：

- 交易所合约更新
- CLOB 后端重写
- 抵押资产从 `USDC.e` 迁移到 `pUSD`
- Builder 集成方式简化
- Exchange 的 EIP-712 domain version 从 `1` 升到 `2`
- API 认证使用的 `ClobAuthDomain` 仍然是 version `1`

参考：

- <https://docs.polymarket.com/v2-migration>
- <https://docs.polymarket.com/trading/overview>

## 3. CLOB 认证模型

官方明确把 CLOB 认证拆成两层。

### 3.1 Public

以下能力无需认证：

- CLOB 读接口
- 订单簿
- 价格
- 点差
- 市场参数

### 3.2 L1 认证

L1 使用钱包私钥对 EIP-712 结构做签名，主要用途：

- 创建 API 凭证
- 派生已有 API 凭证
- 本地签名订单

L1 请求头：

- `POLY_ADDRESS`
- `POLY_SIGNATURE`
- `POLY_TIMESTAMP`
- `POLY_NONCE`

创建和派生 API 凭证的 REST 端点：

- `POST /auth/api-key`
- `GET /auth/derive-api-key`

官方文档给出的 L1 签名结构：

- domain
  - `name = "ClobAuthDomain"`
  - `version = "1"`
  - `chainId = 137`
- type `ClobAuth`
  - `address: address`
  - `timestamp: string`
  - `nonce: uint256`
  - `message: string`
- 固定 message
  - `This message attests that I control the given wallet`

文档同时说明：

- `timestamp` 应该使用 CLOB API server timestamp
- `nonce` 默认可以为 `0`
- 如果 `createApiKey()` 里 nonce 已被使用，应改用 `deriveApiKey()` 或换一个 nonce

### 3.3 L2 认证

L2 使用 L1 生成的 API 凭证：

- `apiKey`
- `secret`
- `passphrase`

L2 请求头一共 5 个：

- `POLY_ADDRESS`
- `POLY_SIGNATURE`
- `POLY_TIMESTAMP`
- `POLY_API_KEY`
- `POLY_PASSPHRASE`

L2 的 `POLY_SIGNATURE` 是基于 `secret` 的 HMAC-SHA256。官网文档页说明了这是 HMAC，但没有把“待签名字符串的拼接规则”直接写在文档正文里，而是让开发者去看官方客户端参考实现。

这意味着：

- Go 版客户端在实现 HMAC 时，必须再对照官方 SDK 源码核实
- 不能只根据文档页的描述自行猜测 canonical string

参考：

- <https://docs.polymarket.com/api-reference/authentication>

## 4. Signature Type 与 Funder

官方要求初始化交易客户端时提供：

- `signatureType`
- `funderAddress`

文档给出的类型：

- `0 = EOA`
  - 独立钱包
  - funder 就是 EOA 自己
- `1 = POLY_PROXY`
  - Magic Link / 邮箱 / Google 登录场景
  - funder 是代理钱包地址
- `2 = GNOSIS_SAFE`
  - Browser wallet、嵌入式钱包、常见 Polymarket 账户场景
  - funder 是代理钱包地址

官方强调：

- 如果用户是在 Polymarket.com 内持有资产，显示给用户的钱包地址通常是 proxy wallet
- 这个地址应作为 funder 使用
- 如果 funder 地址不对，会触发 `Invalid Funder Address`

参考：

- <https://docs.polymarket.com/trading/overview>
- <https://docs.polymarket.com/api-reference/authentication>

## 5. 推荐先覆盖的公共接口

### 5.1 健康检查

- `GET /ok`

官方 Public Methods 把它定义为健康检查接口。

### 5.2 市场与订单簿读取

和 CLOB Go client 直接相关的公共能力：

- `GET /book`
  - 查询单个 token 的订单簿
  - 关键参数：`token_id`
- `GET /price`
  - 查询指定 token 的买/卖最佳价
- `GET /midpoint`
  - 查询中间价
- `GET /spread`
  - 查询点差
- `GET /tick-size`
  - 查询最小价格变动单位
- `GET /clob-markets/{condition_id}`
  - 查询 market 级别参数
- `GET /time`
  - 查询服务器 Unix 时间

`GET /book` 返回的关键字段：

- `market`
- `asset_id`
- `timestamp`
- `hash`
- `bids`
- `asks`
- `min_order_size`
- `tick_size`
- `neg_risk`
- `last_trade_price`

`GET /clob-markets/{condition_id}` 返回的关键字段：

- `t`
  - token 列表
- `mos`
  - minimum order size
- `mts`
  - minimum tick size
- `mbf`
  - maker base fee
- `tbf`
  - taker base fee
- `rfqe`
  - 是否支持 RFQ
- `fd`
  - fee details
- `oas`
  - order age seconds

`GET /time` 返回服务器 Unix 时间，适合用于 L1 签名的时间同步。

参考：

- <https://docs.polymarket.com/trading/clients/public>
- <https://docs.polymarket.com/api-reference/market-data/get-order-book>
- <https://docs.polymarket.com/api-reference/markets/get-clob-market-info>
- <https://docs.polymarket.com/api-reference/data/get-server-time>

## 6. 推荐先覆盖的交易接口

### 6.1 提交订单

- `POST /order`

官方文档给出的请求体结构包含：

- `order`
  - `maker`
  - `signer`
  - `tokenId`
  - `makerAmount`
  - `takerAmount`
  - `side`
  - `expiration`
  - `timestamp`
  - `metadata`
  - `builder`
  - `signature`
  - `salt`
  - `signatureType`
- `owner`
- `orderType`
- `deferExec`

返回值包含：

- `success`
- `orderID`
- `status`
  - `live`
  - `matched`
  - `delayed`
- `makingAmount`
- `takingAmount`
- `transactionsHashes`
- `tradeIDs`
- `errorMsg`

### 6.2 查询用户订单

- `GET /data/orders`

查询参数：

- `id`
- `market`
- `asset_id`
- `next_cursor`

返回包含分页信息：

- `limit`
- `next_cursor`
- `count`
- `data`

### 6.3 撤单

- `DELETE /order`
  - body: `{"orderID":"..."}`

返回：

- `canceled`
- `not_canceled`

### 6.4 其他交易端点

从 API 目录能看到还存在：

- `POST /orders`
- `DELETE /orders`
- `DELETE /cancel-all`
- `DELETE /cancel-market-orders`
- `GET /order`
- `GET /order-scoring-status`
- `POST /heartbeat`

这些都适合纳入 Go 客户端第二阶段。

参考：

- <https://docs.polymarket.com/api-reference/trade/post-a-new-order>
- <https://docs.polymarket.com/api-reference/trade/get-user-orders>
- <https://docs.polymarket.com/api-reference/trade/cancel-single-order>

## 7. 限流信息

官方文档给了明确限流规则，Go client 设计时应把这些值当成默认约束：

- CLOB general: `9000 req / 10s`
- `/book`: `1500 req / 10s`
- `/price`: `1500 req / 10s`
- `/midpoint`: `1500 req / 10s`
- `/prices-history`: `1000 req / 10s`
- `/data/orders`: `500 req / 10s`
- API key endpoints: `100 req / 10s`
- `POST /order`:
  - burst: `3500 req / 10s`
  - sustained: `36000 req / 10 min`
- `DELETE /order`:
  - burst: `3000 req / 10s`
  - sustained: `30000 req / 10 min`

这对 Go 版客户端意味着：

- 默认不要自动重试所有请求
- 应区分公共读请求和交易写请求的重试策略
- 后续可以增加可选的 client-side rate limiter

参考：

- <https://docs.polymarket.com/api-reference/rate-limits>

## 8. 当前官方文档里需要特别警惕的冲突与缺口

这是实现 Go client 前最重要的一段。

### 8.1 V2 订单结构存在文档冲突

迁移文档写的是：

- V2 签名结构移除了 `taker`
- 移除了 `expiration`
- 移除了 `nonce`
- 移除了 `feeRateBps`
- 增加了 `timestamp`
- 增加了 `metadata`
- 增加了 `builder`

但同一个迁移文档下面给的“Order value to sign”和“POST /order body”示例又仍然带着：

- `taker`
- `expiration`
- `nonce`
- `feeRateBps`

同时 API Reference 的 `POST /order` 页面示例也包含 `expiration`。

结论：

- 在没有再核对官方 SDK 源码前，不能把某一个版本的 order struct 直接固化进 Go 库
- 第一版 Go client 更稳妥的做法是先支持“提交已签名订单 payload”

### 8.2 `owner` 字段来源不清晰

`POST /order` 文档把 `owner` 描述为：

- `UUID of the API key owner`

但创建/派生 API key 的文档返回只展示：

- `apiKey`
- `secret`
- `passphrase`

没有直接解释 `owner` 应如何获取。

结论：

- 自动下单前，需要再核对官方 SDK 或更多 API 文档
- 否则容易把 `owner` 错当成 `apiKey`

### 8.3 HMAC 具体拼接规则未在文档正文写明

认证页只说明 L2 是 HMAC-SHA256，并把开发者指向官方客户端参考实现。

结论：

- 实现 L2 前必须再看官方 SDK 源码
- 否则签名错误会直接导致认证失败

## 9. 对 Go 客户端的本地落地建议

基于当前官方资料，比较稳的实现顺序是：

1. 先做公共读接口
2. 再做 L1 签名和 API key 创建/派生
3. 再做 L2 请求头和订单查询/撤单
4. 下单先支持“提交外部已签名订单”
5. 最后再补自动构造和签名 V2 订单

这样可以在不误判 V2 order schema 的前提下，先把最稳定的 80% 能力做出来。
