package mqttclient

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MqttClient mqtt.Client

// 根据broker URL 和 clientID 创建并连接 MQTT 客户端
func NewClient(brokerURL, clientID string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		// 设置自动重连，心跳，超时等
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(60 * time.Second).
		SetPingTimeout(10 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if ok := token.WaitTimeout(10 * time.Second); !ok {
		return nil, fmt.Errorf("MQTT 连接超时")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT 连接失败: %w", err)
	}
	return client, nil
}

// EdgeX MessageBus 的通用消息格式
type EdgexMessage struct {
	ApiVersion    string      `json:"apiVersion"`
	ReceivedTopic string      `json:"receivedTopic"`
	CorrelationID string      `json:"correlationID"`
	RequestID     string      `json:"requestID"`
	ErrorCode     int         `json:"errorCode"`
	Payload       interface{} `json:"payload"`
	ContentType   string      `json:"contentType"`
}

// 通用消息格式中的 payload 部分
type SinkPayload struct {
	Type      string `json:"Type"`      // sink: 网关自身参数，sensor: 传感器数据
	Eid       string `json:"Eid"`       // 模块 EID
	Timestamp uint64 `json:"Timestamp"` // 世纪秒时间戳
	Datalen   int    `json:"Datalen"`   // 原始数据长度
	Data      string `json:"Data"`      // 原始数据
}

// 订阅指定 topic，解析后把 Data 放入 SinkHexDataCh
func SubscribeSinkData(cli mqtt.Client, topic string, qos byte) error {
	log.Printf("🔔 订阅数据: %s", topic)
	tok := cli.Subscribe(topic, qos, sinkMsgHandler)
	tok.Wait()
	return tok.Error()
}

// 消费通道：原始字节
var SinkRawDataCh = make(chan []byte, 128)

// ---- 提取 payload 的原始 JSON 字节 ----
func payloadBytes(p interface{}) ([]byte, error) {
	switch v := p.(type) {
	case nil:
		return nil, errors.New("payload is nil")
	case json.RawMessage:
		return []byte(v), nil
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case map[string]interface{}:
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}

// ---- HEX 解码：去掉空白、分隔符、0x 前缀 ----
func decodeHexFlexible(s string) ([]byte, error) {
	r := strings.NewReplacer(
		" ", "", "\t", "", "\n", "", "\r", "",
		",", "", ";", "", ":", "", "-", "",
		"0x", "", "0X", "",
	)
	s = r.Replace(strings.TrimSpace(s))
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("HEX 长度不是偶数: %d", len(s))
	}
	return hex.DecodeString(s)
}

// ========== MQTT 回调 ==========
func sinkMsgHandler(_ mqtt.Client, msg mqtt.Message) {
	// 解外层
	var env EdgexMessage
	if err := json.Unmarshal(msg.Payload(), &env); err != nil {
		log.Printf("❌ 解析 EdgexMessage 失败: %v; payload=%s", err, string(msg.Payload()))
		return
	}
	pb, err := payloadBytes(env.Payload)
	if err != nil || len(pb) == 0 {
		log.Printf("❌ 读取内层 payload 失败: %v", err)
		return
	}

	// 2) 解内层 SinkPayload
	var sp SinkPayload
	if err := json.Unmarshal(pb, &sp); err != nil {
		log.Printf("❌ 解析 SinkPayload 失败: %v; payload=%s", err, string(pb))
		return
	}
	if sp.Data == "" {
		log.Printf("⚠ SinkPayload.Data 为空，忽略")
		return
	}
	// 仅处理 Type=="sink"
	if sp.Type != "" && sp.Type != "sink" {
		log.Printf("ℹ 跳过 Type=%q 的消息", sp.Type)
		// 若也要处理 sensor，可去掉这个判断
	}

	// HEX → 原始字节
	raw, err := decodeHexFlexible(sp.Data)
	if err != nil {
		log.Printf("❌ HEX 解码失败: %v; Data=%q", err, sp.Data)
		return
	}
	// 长度校验（若上游未填或为负，则不校验）
	if sp.Datalen >= 0 && sp.Datalen != len(raw) {
		log.Printf("⚠ Datalen(%d) ≠ 实际字节数(%d)", sp.Datalen, len(raw))
	}

	// 投递到通道
	select {
	case SinkRawDataCh <- raw:
	default:
		log.Printf("⚠ SinkRawDataCh 已满，丢弃 len=%d", len(raw))
	}
}

// 清洗/校验：去空白与常见分隔符、去 0x 前缀；确保偶数长度
func normalizeHex(s string) (string, []byte, error) {
	r := strings.NewReplacer(
		" ", "", "\t", "", "\n", "", "\r", "",
		",", "", ";", "", ":", "", "-", "",
		"0x", "", "0X", "",
	)
	s = r.Replace(strings.TrimSpace(s))
	if len(s) == 0 {
		return "", nil, errors.New("hex string is empty")
	}
	if len(s)%2 != 0 {
		return "", nil, fmt.Errorf("hex length is odd: %d", len(s))
	}
	b, err := hex.DecodeString(s)
	return s, b, err
}

// 发送至topic
// - eid:   模块 EID
// - data:  字符串
func PublishSinkCommand(client mqtt.Client, topic, eid, data string) error {
	//规整 & 校验
	normHex, raw, err := normalizeHex(data)
	if err != nil {
		return fmt.Errorf("invalid hex data: %w", err)
	}

	//组内层 payload
	sp := SinkPayload{
		Type:      "sink",
		Eid:       eid,
		Timestamp: uint64(time.Now().Unix()),
		Datalen:   len(raw), // 字节数
		Data:      strings.ToUpper(normHex),
	}

	//外层
	env := EdgexMessage{
		ApiVersion:    "v3",
		CorrelationID: fmt.Sprintf("sink-%d", time.Now().UnixNano()),
		RequestID:     fmt.Sprintf("req-%d", time.Now().UnixNano()),
		ErrorCode:     0,
		Payload:       sp,
		ContentType:   "application/json",
	}

	//序列化并发布
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal edgex message: %w", err)
	}

	token := client.Publish(topic, 0, false, body)
	token.Wait()
	return token.Error()
}

func Close(ms uint) {
	if MqttClient != nil && MqttClient.IsConnectionOpen() {
		MqttClient.Disconnect(ms)
	}
}
