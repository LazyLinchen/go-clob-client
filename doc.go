// Package clobclient 提供面向 Polymarket CLOB API 的 Go 客户端。
//
// 当前版本主要覆盖以下能力：
//   - 可复用且支持并发安全的 HTTP 客户端核心
//   - 无需鉴权的公开市场数据接口
//   - L1/L2 鉴权辅助方法与 API Key 生命周期管理
//   - 账户余额、授权额度与持仓查询
//   - 订单查询、评分/心跳、撤单、V2 限价单与 market order 构造签名以及订单提交
//
// Client 设计为长生命周期对象，建议在进程内复用。默认情况下，
// 多个客户端实例会共享连接池传输层，以便复用 Keep-Alive 连接。
package clobclient
