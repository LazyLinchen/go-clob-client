# go-clob-client

[![CI](https://github.com/LazyLinchen/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/LazyLinchen/go-clob-client/actions/workflows/ci.yml)

`go-clob-client` 是 Polymarket CLOB API 的 Go 客户端库，包名为 `clobclient`。

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

不要提交私钥、API Secret 或 `.env` 文件。

## 测试

本地确定性测试会排除 `TestIntegration...` live 测试：

```powershell
go test ./... -run '^Test[^I]' -count=1
go build ./...
```

live 测试会访问真实 Polymarket 服务，部分订单测试可能真实成交。运行前请配置 `.env`，
并只用精确的 `-run TestIntegration...` 目标执行。
