# 统一 SDK 对照与迁移说明

核对日期：2026-10-12。范围是本库的 CLOB 订单签名、提交及账户 HTTP 接口，不是整个 TypeScript monorepo 的 Go 移植。

## 来源与可验证范围

官方源码固定到 [Polymarket/ts-sdk @ 1462212874971ce60d1ab56a6df13eb6eda89daa](https://github.com/Polymarket/ts-sdk/tree/1462212874971ce60d1ab56a6df13eb6eda89daa)。
重点核对 `packages/client/src/` 下的 `exchange.ts`、`environments.ts`、`actions/orders/{context,amounts,orders,post,limit,market}.ts` 和 `actions/account.ts`。

用户提供的[中文迁移文档](https://docs.polymarket.com/cn/migrate/clob-sdk-to-unified-sdk)在本次环境中无法访问；浏览工具、Markdown 版本和直接 HTTPS 请求均失败。因此以下结论来自实际下载的官方源码，不声称已逐条验证该页面。也未将文档无法访问解释为页面不存在。

## 已核对和更新的行为

| 事项 | 官方源码行为 | Go 实现 |
| --- | --- | --- |
| 订单结构 | salt、maker、signer、tokenId、makerAmount、takerAmount、side、signatureType、timestamp、metadata、builder | 保留现有 V2 格式；expiration 仅作为提交字段，不加入订单 EIP-712 结构 |
| 签名域 | CTF token 使用 version `2`；协议 V2 position 使用 version `3` | 新增 `ExchangeProtocolVersion`，显式选择 V3；默认 V2 兼容原有调用 |
| POLY_1271 | ERC-7739 TypedDataSign，内层 DepositWallet version `1` | 两种 Exchange 域均支持，包裹签名的 domain separator 同步切换 |
| tick | `0.1`、`0.01`、`0.005`、`0.0025`、`0.001`、`0.0001` | 补充 `TickSizeHalfCent` (`0.005`) |
| 限价金额精度 | size 向下保留两位；乘积按 tick 分别保留 3/4/5/6/5/6 位 | 修复此前所有计算金额最多保留四位的问题；BUY/SELL 均覆盖 |
| salt | 53 位随机数，提交范围 `0..9007199254740991`，JSON number | 扩大随机空间；`SignedOrderV2.Payload()` 拒绝超范围值 |
| 签名订单提交 | 转换成 wire order，salt 为 number | `PostOrder` / `PostOrders` 自动转换 `SignedOrderV2` 值和指针，调用方不再必须先调用 `Payload()` |
| post-only | 仅允许 GTC/GTD | 构造与提交入口均拒绝 FOK/FAK + post-only，非法 orderType 在本地拒绝；提交类型留空默认 GTC |
| 余额刷新 | GET `/balance-allowance/update` 成功后再 GET `/balance-allowance` | 新增 `UpdateBalanceAllowance`；刷新失败不执行第二个请求；不是链上 approve |
| 批量 scoring | POST `/orders-scoring`，body 为订单 ID 数组 | 新增 `GetOrdersScoringStatus`，返回 `map[string]bool`；清理空白和重复 ID |

## 签名版本选择

“V2 订单格式”不等于“资产协议 V2”。官方当前实现通过 asset ID 的 reserved bits 选择签名域和合约；本库为保留既有行为，要求调用者明确指定版本，不会通过小整数或测试 token ID 自动切换。

```go
// client 已按实际账户配置 signer、signature type、funder 及 API 凭证。
signed, err := client.CreateOrder(ctx, clobclient.CreateOrderParams{
    TokenID:         positionID, // 本库沿用 TokenID 字段；传入十进制 uint256 position ID
    Price:           "0.505",
    Size:            "1.23",
    Side:            clobclient.SideBuy,
    TickSize:        clobclient.TickSizeHalfCent,
    ProtocolVersion: clobclient.ExchangeProtocolV3,
})
if err != nil {
    return err
}
// 这一调用将真实提交订单；只在准备好交易时执行。
result, err := client.PostOrder(ctx, clobclient.PostOrderRequest{
    Order: signed,
    OrderType: string(clobclient.OrderTypeGTC),
})
```

`CreateMarketOrderParams` 和 `SignOrderV2Options` 同样支持 `ProtocolVersion`。参数为空或 `ExchangeProtocolV2` 时继续使用原 V2 域。`BuildOrderV2TypedData` 保持只生成 version `2`；使用外部签名器时，通过 `SignOrderV2` 的 `L1Signer` 接口接收最终的 typed data。

V3 的 Polygon 默认合约来自官方 `environments.ts`：`0xe3333700cA9d93003F00f0F71f8515005F6c00Aa`。V3 不按 NegRisk 分流。没有根据 Polygon 地址猜测 Amoy 部署；其他链默认报错。确有部署依据时，可通过低层 `SignOrderV2Options.ExchangeAddress` 覆盖。

## HTTP 示例

余额刷新与读取均签名各自的路径，沿用当前账户 signature type：

```text
GET /balance-allowance/update?asset_type=CONDITIONAL&signature_type=3&token_id=123
GET /balance-allowance?asset_type=CONDITIONAL&signature_type=3&token_id=123
```

批量 scoring 的 body 是数组，不是 `{ "orderIds": ... }`：

```json
["order-one", "order-two"]
```

响应示例：

```json
{"order-one": true, "order-two": false}
```

签名订单直接传入 `PostOrder` 后，wire payload 中的 salt 为数字，而金额与 tokenId 仍为字符串。请求体编码完成后再生成 L2 HMAC，测试核对实际发送的字节与签名一致。

## 兼容性与未对齐部分

- 修正精度会改变部分小 tick 限价单的 makerAmount/takerAmount 和签名；这是有意修复，不保证旧错误金额的字节兼容。
- 旧限价单接口仍将不在 tick 网格上的价格四舍五入到 tick；统一 SDK 采用严格网格校验。调用者应提供已在网格上的价格。本次未改变旧接口的价格舍入约定。
- 旧 market order 的四位计算金额上限继续保留（仓库已有专门回归测试）。统一 SDK 的金额计算及 maxPrice/minPrice 保护语义尚未完整移植；不能据此宣称 market order 完全等价。
- TickSize 和 NegRisk 仍由调用者提供；未移植统一 SDK 的市场 metadata 缓存、自动协议选择、GTD 三分钟校验或异步分页迭代器。
- 原始 JSON / 自定义 order 对象继续作为低层透传入口；`Payload()` 的 salt 保证适用于通过 `SignedOrderV2` 提交的订单。原始对象的完整结构和范围由调用方负责。
- 不包括 Gamma/Relayer/链上批准、钱包部署、合并/拆分/赎回、WebSocket、RFQ、Combos、Perps 等统一 SDK 的其他模块。DepositWallet 本次沿用项目原有派生模型，没有宣称覆盖统一 SDK 的所有钱包部署方式。
- 没有新增自动重试下单。HTTP 错误和业务 success/errorMsg 的现有处理约定保持不变。

## 验证记录

测试位于 `unified_sdk_test.go`，使用本地 `httptest` 与仓库已有公开测试私钥，不读取 `.env`。

| 保证 | 测试 |
| --- | --- |
| 0.005 tick 的 BUY/SELL 保留第五位金额 | `TestUnifiedHalfCentTick` |
| salt 边界与随机范围 | `TestUnifiedSafeSalt`、`TestUnifiedGeneratedSalt` |
| post-only 类型限制 | `TestUnifiedPostOnly`、`TestUnifiedValidationAndFailures` |
| V3 EOA 签名恢复、POLY_1271 外层域与内层签名 | `TestUnifiedExchangeV3` |
| 签名订单转换后 salt 为 JSON number，HMAC 等于实际 body 的签名 | `TestUnifiedPostingSignedOrder` |
| 刷新顺序、刷新失败短路及最新余额返回 | `TestUnifiedUpdateBalanceAllowance` |
| scoring 路径、请求体、鉴权、反序列化及异常 | `TestUnifiedOrdersScoring`、`TestUnifiedValidationAndFailures` |
| 15 笔批量签名订单转换、HMAC、空/超限/非法订单在网络前拒绝 | `TestUnifiedBatchPosting` |

TDD RED：`go test ./... -run '^TestUnified' -count=1` 首次因缺少版本参数和新增账户方法而编译失败。随后运行测试还复现了 half-cent 金额为 `621100` 而非 `621150` 的精度错误。GREEN：同一测试集在实现修复后通过；原有非 live 测试亦通过。

验证不覆盖链上执行或生产服务的接单情况。完整本地覆盖率基线为 53.8%，不是历史说明中的约 73.7%；没有为提升覆盖率而运行生产集成测试。

最终检查：

- `go build ./...`：通过。
- `go vet ./...`：通过。
- `go test ./... -run '^Test[^I]' -count=1 -cover`：通过，覆盖率 **61.6%**（未达到全库 80%）。
- `git diff --check`：通过。
- `go test ./... -run '^Test[^I]' -race -count=1`：未能启动，环境 `CGO_ENABLED=0`，且未找到 gcc/clang；不作为通过的验证项目。

本地保留 TDD 检查点：`49e579d`（RED）和 `ecf288b`（GREEN）。后续测试增加了批量提交和错误路径覆盖。未执行任何 `TestIntegration...` 测试，也未提交真实交易。

## 后续增补：签名类型识别

新增 `DetectSignatureType(signerAddress, walletAddress, chainID)` 和客户端方法
`WithDetectedSignatureType(walletAddress)`。使用相同官方源码提交中 `wallet.ts` 的 Proxy/Safe
CREATE2 公式、`environments.ts` 的 Polygon 配置以及本库已有 Deposit Wallet 派生逻辑。
`signature_detection_test.go` 的 Polygon 四类型固定向量来自官方 `wallet.test.ts`，不是本库自生成期望值。

该接口只识别当前签名实现支持的钱包，未知地址返回 `SignatureTypeUnknown` 与 `ErrUnknownWallet`。
与官方针对未知地址的 session-key 默认回退不同，本库不支持 session signer，因此不回退。
不自动查询资金地址，不验证部署或授权，不修改 `AutoDiscoverFunder` 的行为；用法与链支持范围见根 README。

验证：新增测试首次因 API 缺失编译失败（RED，`22c6e83`），实现后通过（GREEN）。
非 live 全量测试通过，覆盖率 62.9%；识别主函数覆盖率 93.1%，地址校验与客户端复制方法均为 100%。
`go build ./...`、`go vet ./...`、`git diff --check` 通过。未运行 live 测试。
同时保留前次四签名类型的 64 组合测试 `signature_types_test.go`。
