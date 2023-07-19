package snoti

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/zeromicro/go-zero/core/hash"
	"runtime"
	"time"
)

// const Snoti cmd type.
const (
	CmdLoginReq  = "login_req"
	CmdLoginRes  = "login_res"
	CmdEventPush = "event_push"
	CmdEventAck  = "event_ack"
	CmdPing      = "ping"
	CmdPong      = "pong"
)

// Request Snoti请求协议.
type Request struct {
	Cmd           string      `json:"cmd"`
	PrefetchCount int         `json:"prefetch_count,omitempty"`
	Data          interface{} `json:"data,omitempty"`
	MsgID         string      `json:"msg_id,omitempty"`
	DeliveryID    int         `json:"delivery_id,omitempty"`
}

type Handler func(*Message)

// Message Snoti回复协议.
type Message struct {
	Cmd     string `json:"cmd"`
	Did     string `json:"did"`
	Payload []byte
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
	//订阅消息类型，多个时用','隔开，详情见：http://docs.gizwits.com/zh-cn/Cloud/NotificationAPI.html#%E5%85%B3%E9%94%AE%E6%9C%AF%E8%AF%AD
	EventTypes string
	//每次最多拉取消息条数，默认50.
	PrefetchCount int
	//单消息最大容量(B)，默认1024B.
	PacketSize int
}

// Client is a client manages communication with the Snoti API.
type Client struct {
	cfg     Config
	handler Handler

	conn      *tls.Conn
	stopCh    chan struct{}
	heartbeat chan string
	msgCh     []chan *Message
	msgChSize uint64
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
	}
}

// Start the snoti client.
func (c *Client) Start() error {
	c.msgChSize = uint64(runtime.NumCPU())
	c.msgCh = make([]chan *Message, c.msgChSize)
	for i := uint64(0); i < c.msgChSize; i++ {
		c.msgCh[i] = make(chan *Message, 64)
		go c.loop(c.msgCh[i])
	}

	go c.run()
	return nil
}

// Stop the snoti client.
func (c *Client) Stop() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	close(c.stopCh)
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
	for {
		msg := &Message{}
		data, err := reader.ReadSlice('\n')
		if err != nil {
			fmt.Printf("ReadSlice error:%s\n", err.Error())
			return
		}
		if err = json.Unmarshal(data, msg); err != nil {
			fmt.Printf("Unmarshal data err:%s\n", err.Error())
			continue
		}

		msg.Payload = data
		c.msgCh[hash.Hash([]byte(msg.Did))%c.msgChSize] <- msg
	}
}

func (c *Client) loop(msgCh <-chan *Message) {
	for {
		select {
		case msg := <-msgCh:
			switch msg.Cmd {
			case CmdLoginRes:
				c.loginRes(msg)
			case CmdEventPush:
				c.handler(msg)
			case CmdPong:
				c.heartbeat <- "pong"
			default:
				fmt.Printf("unknown cmd:%s\n", msg.Cmd)
			}
		}
	}
}
