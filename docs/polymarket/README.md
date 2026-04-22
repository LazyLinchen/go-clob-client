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
- “自动构造并签名 V2 订单”需要在实现前继续核对官方 SDK 源码，因为当前官网文档在订单结构上存在不一致。

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
  - 默认 `https://clob.polymarket.com`
- `POLYMARKET_CHAIN_ID`
  - 默认 `137`
- `POLYMARKET_NONCE`
  - 默认 `0`
- `POLYMARKET_PROXY_URL`
  - 例如 `http://127.0.0.1:7897`
  - 不设置则直连
- `POLYMARKET_USE_SERVER_TIME`
  - 可选，填 `1` 或 `true`
  - 不填时按官方 SDK 默认行为，使用本地 Unix 秒时间签名 L1 请求

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
