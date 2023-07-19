package main

import (
	"encoding/json"
	snoti "github.com/numberten123/snoti_go_sdk"
)

var client *snoti.Client

func main() {
	conf := snoti.Config{
		Endpoint:      "snoti.gizwits.com:2017",
		ProductKey:    "Your ProductKey",
		AuthID:        "Your AuthID",
		AuthSecret:    "Your AuthSecret",
		SubKey:        "SubKey_test",
		EventTypes:    "device.online,device.offline,device.status.kv",
		PrefetchCount: 50,
		PacketSize:    1024,
	}
	client = snoti.NewClient(conf, Handler)
	client.Start()
}

type Response struct {
	Cmd        string `json:"cmd"`
	DeliveryId int    `json:"delivery_id"`
	MsgId      string `json:"msg_id"`
	EventType  string `json:"event_type"`
}

func Handler(payload []byte) {
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		panic(err)
	}

	if err := client.Ack(resp.MsgId, resp.DeliveryId); err != nil {
		panic(err)
	}
}
