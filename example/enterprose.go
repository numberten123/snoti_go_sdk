package main

import (
	"encoding/json"

	snoti "github.com/numberten123/snoti_go_sdk"
)

var client *snoti.Client

func main() {
	conf := snoti.Config{
		Endpoint:         "snoti.gizwits.com:2017",
		SubKey:           "SubKey_test",
		PrefetchCount:    50,
		PacketSize:       1024,
		EnterpriseID:     "Your EnterpriseID",
		EnterpriseSecret: "Your EnterpriseSecret",
	}
	client = snoti.NewClient(conf, Handler)
	client.Start()
}

type Response struct {
	Cmd        string `json:"cmd"`
	DeliveryId int    `json:"delivery_id"`
	MsgId      string `json:"msg_id"`
	EventType  string `json:"event_type"`
	Did        string `json:"did"`
	Mac        string `json:"mac"`
	ProductKey string `json:"product_key"`
}

func Handler(payload []byte) {
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		panic(err)
	}

	//回复ack，未回复则会重发该消息
	if resp.Cmd == "event_push" {
		if err := client.Ack(resp.MsgId, resp.DeliveryId); err != nil {
			panic(err)
		}
	}

	//发送指令给到设备端
	if resp.EventType == "device_online" {
		RemoteControlByAttr(resp)
		RemoteControlByRaw(resp)
	}
}

func RemoteControlByAttr(resp Response) error {
	data := []snoti.ControlData{
		{
			Cmd: snoti.ControlAttr,
			Data: snoti.ControlDataDetail{
				ProductKey: resp.ProductKey,
				Did:        resp.Did,
				Mac:        resp.Mac,
				Attrs:      map[string]interface{}{"test2": 2}, //数据点kv数据
			},
		},
	}
	return client.RemoteControl(data)
}

func RemoteControlByRaw(resp Response) error {
	data := []snoti.ControlData{
		{
			Cmd: snoti.ControlWrite,
			Data: snoti.ControlDataDetail{
				ProductKey:   resp.ProductKey,
				Did:          resp.Did,
				Mac:          resp.Mac,
				BinaryCoding: "hex",
				Raw:          "1203", //查询指令
			},
		},
	}
	return client.RemoteControl(data)
}
