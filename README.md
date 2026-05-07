# go-clob-client

`go-clob-client` 是 Polymarket CLOB API 的 Go 客户端库，包名为 `clobclient`。

## 安装

当前仓库托管在私有 Gogs 服务上，远程地址为 `https://gogs.xelaly.com:3010/lorry/go-clob-client.git`。因为 Go module path 不能包含端口号，本库使用 `.git` 后缀的 module path，并通过 Git URL rewrite 指向实际端口。

在需要使用此库的其他 Go 项目中执行：

```powershell
go env -w GOPRIVATE=gogs.xelaly.com
git config --global url."https://gogs.xelaly.com:3010/".insteadOf "https://gogs.xelaly.com/"
go get gogs.xelaly.com/lorry/go-clob-client.git@latest
```

如果只是本机临时开发，也可以在调用方项目的 `go.mod` 中使用本地替换：

```go
require gogs.xelaly.com/lorry/go-clob-client.git v0.0.0

replace gogs.xelaly.com/lorry/go-clob-client.git => E:/Codes/Golang/go-clob-client
```

## 基本用法

```go
package main

import (
	"context"
	"fmt"
	"log"

	clobclient "gogs.xelaly.com/lorry/go-clob-client.git"
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
