package snoti

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// const Snoti cmd type.
const (
	CmdLoginReq           = "login_req"
	CmdEnterpriseLoginReq = "enterprise_login_req"
	CmdLoginRes           = "login_res"
	CmdEventPush          = "event_push"
	CmdEventAck           = "event_ack"
	CmdSubscribe          = "subscribe_req"
	CmdUnSubscribe        = "unsubscribe_req"
	CmdRemoteControl      = "remote_control_v2_req"
	CmdPing               = "ping"
	CmdPong               = "pong"
)

// Request Snoti请求协议.
type Request struct {
	Cmd           string      `json:"cmd"`
	PrefetchCount int         `json:"prefetch_count,omitempty"`
	Data          interface{} `json:"data,omitempty"`
	MsgID         string      `json:"msg_id,omitempty"`
	DeliveryID    int         `json:"delivery_id,omitempty"`
}

type Handler func([]byte)

// Message Snoti回复协议.
type Message struct {
	Cmd string `json:"cmd"`
	Did string `json:"did"`
}

// Config Snoti 连接配置.
type Config struct {
	// Snoti 连接地址，如:snoti.gizwits.com:2017.
	Endpoint string
	// 产品Key，在开发者中心获取.
	ProductKey string
	// 产品授权Id，在开发者中心Snoti模块获取，详情见：http://docs.gizwits.com/zh-cn/Cloud/NotificationAPI.html#%E5%85%B3%E9%94%AE%E6%9C%AF%E8%AF%AD
	AuthID string
	// 产品授权秘钥，在开发者中心Snoti模块获取，详情见：http://docs.gizwits.com/zh-cn/Cloud/NotificationAPI.html#%E5%85%B3%E9%94%AE%E6%9C%AF%E8%AF%AD
	AuthSecret string
	// 消息订阅标识，可设置任意值，一个AuthID限额5个，详情见：http://docs.gizwits.com/zh-cn/Cloud/NotificationAPI.html#%E5%85%B3%E9%94%AE%E6%9C%AF%E8%AF%AD
	SubKey string
	// 订阅消息类型，多个时用','隔开，详情见：http://docs.gizwits.com/zh-cn/Cloud/NotificationAPI.html#%E5%85%B3%E9%94%AE%E6%9C%AF%E8%AF%AD
	EventTypes string
	// 每次最多拉取消息条数，默认50.
	PrefetchCount int
	// 单消息最大容量(B)，默认1024B.
	PacketSize int
	// 企业ID，不为空时优先使用企业Id登录.
	EnterpriseID string
	// 企业秘钥.
	EnterpriseSecret string
}

// Client is a client manages communication with the Snoti API.
type Client struct {
	cfg     Config
	handler Handler

	conn      *tls.Conn
	stopCh    chan struct{}
	heartbeat chan string
	msgCh     []chan string
	msgChSize int
}

// NewClient returns a new Snoti API client.
func NewClient(conf Config, handler Handler) *Client {
	if conf.PrefetchCount == 0 || conf.PrefetchCount > 32767 {
		conf.PrefetchCount = 50
	}

	if conf.PacketSize == 0 {
		conf.PrefetchCount = 1024
	}

	return &Client{
		cfg:       conf,
		handler:   handler,
		heartbeat: make(chan string, 100),
		msgChSize: runtime.NumCPU(),
	}
}

// Start the snoti client.
func (c *Client) Start() {
	c.msgCh = make([]chan string, c.msgChSize)
	for i := 0; i < c.msgChSize; i++ {
		c.msgCh[i] = make(chan string, 64)
		go c.loop(c.msgCh[i])
	}

	c.run()
}

// Stop the snoti client.
func (c *Client) Stop() {
	if c.conn != nil {
		_ = c.conn.Close()
	}

	if c.stopCh != nil {
		close(c.stopCh)

		c.stopCh = nil
	}
}

// Ack snoti message.
func (c *Client) Ack(msgId string, deliveryId int) error {
	pkt := Request{
		Cmd:        CmdEventAck,
		MsgID:      msgId,
		DeliveryID: deliveryId,
	}
	data, _ := json.Marshal(pkt)
	return c.Send(data)
}

type SubscribeData struct {
	ProductKey string   `json:"product_key"`
	AuthId     string   `json:"auth_id"`
	AuthSecret string   `json:"auth_secret"`
	SubKey     string   `json:"subkey"`
	EventTypes []string `json:"events"`
}

// Subscribe message.
func (c *Client) Subscribe(eventTypes string) error {
	pkt := Request{
		Cmd: CmdSubscribe,
		Data: []SubscribeData{{
			ProductKey: c.cfg.ProductKey,
			AuthId:     c.cfg.AuthID,
			AuthSecret: c.cfg.AuthSecret,
			SubKey:     c.cfg.SubKey,
			EventTypes: strings.Split(eventTypes, ","),
		}},
	}

	buf, _ := json.Marshal(pkt)
	return c.Send(buf)
}

// UnSubscribe message.
func (c *Client) UnSubscribe(eventTypes string) error {
	pkt := Request{
		Cmd: CmdUnSubscribe,
		Data: []SubscribeData{{
			ProductKey: c.cfg.ProductKey,
			AuthId:     c.cfg.AuthID,
			AuthSecret: c.cfg.AuthSecret,
			SubKey:     c.cfg.SubKey,
			EventTypes: strings.Split(eventTypes, ","),
		}},
	}

	buf, _ := json.Marshal(pkt)
	return c.Send(buf)
}

// const Control cmd type.
const (
	ControlAttr  = "write_attrs"
	ControlWrite = "write"
)

type ControlData struct {
	Cmd  string            `json:"cmd"`
	Data ControlDataDetail `json:"data"`
}

type ControlDataDetail struct {
	Did          string                 `json:"did"`
	Mac          string                 `json:"mac"`
	ProductKey   string                 `json:"product_key"`
	BinaryCoding string                 `json:"binary_coding,omitempty"`
	Attrs        map[string]interface{} `json:"attrs,omitempty"`
	Raw          string                 `json:"raw,omitempty"`
}

// RemoteControl send message to device by mqtt topic `app2dev/did`.
func (c *Client) RemoteControl(data []ControlData) error {
	pkt := Request{
		Cmd:  CmdRemoteControl,
		Data: data,
	}

	buf, _ := json.Marshal(pkt)
	return c.Send(buf)
}

// Send message to snoti server.
func (c *Client) Send(data []byte) error {
	if c.conn == nil {
		return fmt.Errorf("conn not init")
	}
	_, err := c.conn.Write(append(data, '\n'))
	return err
}

func (c *Client) run() {
	var err error
	for {
		c.stopCh = make(chan struct{})

		if c.conn, err = tls.Dial("tcp", c.cfg.Endpoint, &tls.Config{InsecureSkipVerify: true}); err != nil {
			fmt.Printf("snoti client connect error:%s\n", err.Error())
		}

		if err = c.login(); err != nil {
			fmt.Printf("snoti client login error:%s\n", err.Error())
		}

		if err == nil {
			c.receive()
		}

		c.Stop()
		time.Sleep(5 * time.Second)
	}
}

func (c *Client) receive() {
	reader := bufio.NewReaderSize(c.conn, c.cfg.PacketSize)
	for i := 0; i >= 0; i++ {
		payload, err := reader.ReadSlice('\n')
		if err != nil {
			fmt.Printf("ReadSlice error:%s\n", err.Error())
			return
		}
		c.msgCh[i%c.msgChSize] <- string(payload)
	}
}

func (c *Client) loop(msgCh <-chan string) {
	for {
		select {
		case str := <-msgCh:
			var msg Message
			payload := []byte(str)
			if err := json.Unmarshal(payload, &msg); err != nil {
				fmt.Printf("Unmarshal data err:%s\n", err.Error())
				continue
			}

			switch msg.Cmd {
			case CmdLoginRes:
				c.loginRes(payload)
			case CmdPong:
				c.heartbeat <- "pong"
			default:
				c.handler(payload)
			}
		}
	}
}
