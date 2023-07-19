# snoti_go_sdk
机智云Snoti Golang客户端SDK

## Install

`go get -u github.com/numberten123/snoti_go_sdk`


## Usage

```go
package main

import (
	"encoding/json"
	"fmt"
	snoti "github.com/numberten123/snoti_go_sdk"
)

var client *snoti.Client

func main() {
	conf := snoti.Config{
		Endpoint:      "snoti.gizwits.com:2017",
		ProductKey:    "Your ProductKey",
		AuthID:        "Your AuthID",
		AuthSecret:    "Your AuthSecret",
		SubKey:        "Your SubKey",
		EventTypes:    "device.online,device.offline,device.status.kv",
		PrefetchCount: 50,
		PacketSize:    1024,
	}
	client = snoti.NewClient(conf, Handler)
	client.Start()
}
func Handler(payload []byte) {

}
```

所有的 API 在 [example](./example/) 目录下都有对应的使用示例。
