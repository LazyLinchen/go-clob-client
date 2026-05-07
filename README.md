# go-clob-client

`go-clob-client` 是 Polymarket CLOB API 的 Go 客户端库，包名为 `clobclient`。

## 基本用法

```go
package main

import (
	"context"
	"fmt"
	"log"

	clobclient "https://github.com/LazyLinchen/go-clob-client.git"
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
