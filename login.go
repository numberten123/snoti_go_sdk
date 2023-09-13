package snoti

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LoginData struct {
	ProductKey       string   `json:"product_key"`
	AuthId           string   `json:"auth_id"`
	AuthSecret       string   `json:"auth_secret"`
	SubKey           string   `json:"subkey"`
	EventTypes       []string `json:"events"`
	EnterpriseID     string   `json:"enterprise_id"`
	EnterpriseSecret string   `json:"enterprise_secret"`
}

type LoginResponse struct {
	Cmd  string `json:"cmd"`
	Data struct {
		Result bool   `json:"result"`
		Msg    string `json:"msg"`
	} `json:"data"`
}

func (c *Client) login() error {
	if c.cfg.EnterpriseID != "" {
		return c.enterpriseLogin()
	}

	data := LoginData{
		ProductKey: c.cfg.ProductKey,
		AuthId:     c.cfg.AuthID,
		AuthSecret: c.cfg.AuthSecret,
		SubKey:     c.cfg.SubKey,
		EventTypes: strings.Split(c.cfg.EventTypes, ","),
	}
	pkt := Request{
		Cmd:           CmdLoginReq,
		PrefetchCount: c.cfg.PrefetchCount,
		Data:          []LoginData{data},
	}
	buf, err := json.Marshal(pkt)
	if err != nil {
		return err
	}
	return c.Send(buf)
}

func (c *Client) enterpriseLogin() error {
	data := LoginData{
		EnterpriseID:     c.cfg.EnterpriseID,
		EnterpriseSecret: c.cfg.EnterpriseSecret,
		SubKey:           c.cfg.SubKey,
	}
	pkt := Request{
		Cmd:           CmdEnterpriseLoginReq,
		PrefetchCount: c.cfg.PrefetchCount,
		Data:          []LoginData{data},
	}
	buf, err := json.Marshal(pkt)
	if err != nil {
		return err
	}
	return c.Send(buf)
}

func (c *Client) loginRes(payload []byte) {
	var resp LoginResponse
	if err := json.Unmarshal(payload, &resp); err == nil && resp.Data.Result {
		fmt.Println("snoti client login success")

		go c.ping()
		return
	}
	panic(fmt.Errorf("snoti client login fail:%s", resp.Data.Msg))
}

func (c *Client) ping() {
	data, _ := json.Marshal(Request{Cmd: CmdPing})
	pingTicker := time.NewTicker(1 * time.Minute)
	defer pingTicker.Stop()
	hbTicker := time.NewTicker(3 * time.Minute)
	defer hbTicker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-pingTicker.C:
			_ = c.Send(data)
		case <-c.heartbeat:
			hbTicker.Reset(3 * time.Minute)
		case <-hbTicker.C:
			c.Stop()
		}
	}
}
