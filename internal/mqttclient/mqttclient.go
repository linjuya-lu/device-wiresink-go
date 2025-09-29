package mqttclient

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	MqttClient       mqtt.Client
	SinkRawDataCh    = make(chan []byte, 128) // 网关自身参数
	UpgradeRawDataCh = make(chan []byte, 128) // 升级报文
)

// MQTT初始化
func NewClient(brokerURL, clientID string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		// 设置自动重连，心跳，超时
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

// 外层
type EdgexMessage struct {
	ApiVersion    string      `json:"apiVersion"`
	ReceivedTopic string      `json:"receivedTopic"`
	CorrelationID string      `json:"correlationID"`
	RequestID     string      `json:"requestID"`
	ErrorCode     int         `json:"errorCode"`
	Payload       interface{} `json:"payload"`
	ContentType   string      `json:"contentType"`
}

// 内层
type SinkPayload struct {
	Type      string `json:"Type"`      // sink: 网关自身；sensor: 传感器；update: 升级数据；
	Eid       string `json:"Eid"`       // 模块 EID
	Timestamp uint64 `json:"Timestamp"` // 世纪秒时间戳
	Datalen   int    `json:"Datalen"`   // 原始数据长度
	Data      string `json:"Data"`      // 原始数据
}

func SubscribeData(cli mqtt.Client, topic string, qos byte) error {
	log.Printf("订阅数据: %s", topic)
	tok := cli.Subscribe(topic, qos, MsgHandler)
	tok.Wait()
	return tok.Error()
}

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

// ---- HEX解码预处理：去掉空白、分隔符、0x 前缀 ----
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

func MsgHandler(_ mqtt.Client, msg mqtt.Message) {
	// 基本元信息
	log.Printf("[MQTT] topic=%q qos=%d retained=%v dup=%v payloadLen=%d",
		msg.Topic(), msg.Qos(), msg.Retained(), msg.Duplicate(), len(msg.Payload()))

	// 解外层
	var env EdgexMessage
	if err := json.Unmarshal(msg.Payload(), &env); err != nil {
		log.Printf("解析 EdgexMessage 失败: %v; payload=%s", err, string(msg.Payload()))
		return
	}
	log.Printf("[OUTER] EdgexMessage: %+v", env)

	pb, err := payloadBytes(env.Payload)
	if err != nil || len(pb) == 0 {
		log.Printf("读取内层 payload 失败: %v (len=%d)", err, len(pb))
		return
	}
	log.Printf("[OUTER] inner payload bytes len=%d preview=%q", len(pb), string(pb))

	// 解内层
	var sp SinkPayload
	if err := json.Unmarshal(pb, &sp); err != nil {
		log.Printf("解析 SinkPayload 失败: %v; payload=%s", err, string(pb))
		return
	}
	log.Printf("[INNER] SinkPayload: %+v", sp)

	if sp.Data == "" {
		log.Printf("数据帧值为空")
		return
	}

	// 十六进制→字节序列
	raw, err := decodeHexFlexible(sp.Data)
	if err != nil {
		log.Printf("HEX 解码失败: %v; Data=%q", err, sp.Data)
		return
	}
	if sp.Datalen >= 0 && sp.Datalen != len(raw) {
		log.Printf("数据长度(%d) ≠ 实际字节数(%d)", sp.Datalen, len(raw))
	}
	log.Printf("[RAW] bytes len=%d hex=%s", len(raw), strings.ToUpper(hex.EncodeToString(raw)))

	// 按类型分流
	t := strings.ToLower(strings.TrimSpace(sp.Type))
	switch t {
	case "update": // 升级
		select {
		case UpgradeRawDataCh <- raw:
			log.Printf("[DISPATCH] → UpgradeRawDataCh len=%d", len(raw))
		default:
			log.Printf("UpgradeRawDataCh 已满，丢弃 len=%d", len(raw))
		}
	case "", "sink": // 网关自身参数
		fallthrough
	default:
		if t != "" && t != "sink" {
			log.Printf("未识别 Type=%q，按常规通道处理", sp.Type)
		}
		select {
		case SinkRawDataCh <- raw:
			log.Printf("[DISPATCH] → SinkRawDataCh len=%d", len(raw))
		default:
			log.Printf("SinkRawDataCh 已满，丢弃 len=%d", len(raw))
		}
	}
}

// 预处理：去空白与常见分隔符、去 0x 前缀；确保偶数长度；
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

// 默认 QoS=1，超时 10s
func PublishSinkCommand(client mqtt.Client, topic, eid, data string) error {
	//预处理
	normHex, raw, err := normalizeHex(data)
	if err != nil {
		return fmt.Errorf("invalid hex data: %w", err)
	}

	//内层
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

// 解析 data（可能是 "12345" 或 "39300000"/"0x39300000"）为数值 val 和 4B 原始字节 raw。
// 关键：十六进制路径按“原始字节转储”理解，不当作大端整数。
func parseDataToUint32(data string, order binary.ByteOrder) (uint32, []byte, error) {
	s := strings.TrimSpace(data)
	if s == "" {
		return 0, nil, fmt.Errorf("empty data")
	}

	// 纯十进制
	isDec := true
	for _, r := range s {
		if r < '0' || r > '9' {
			isDec = false
			break
		}
	}
	if isDec {
		u, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return 0, nil, fmt.Errorf("parse decimal: %w", err)
		}
		val := uint32(u)
		raw := make([]byte, 4)
		order.PutUint32(raw, val)
		return val, raw, nil
	}

	// 十六进制（原始字节）
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, nil, fmt.Errorf("decode hex: %w", err)
	}
	if len(b) > 4 {
		return 0, nil, fmt.Errorf("hex too long for uint32: %d bytes", len(b))
	}

	// 补齐到4字节：LE 补到右侧，BE 补到左侧
	if len(b) < 4 {
		if order == binary.LittleEndian {
			b = append(b, make([]byte, 4-len(b))...)
		} else {
			pad := make([]byte, 4-len(b))
			b = append(pad, b...)
		}
	}

	val := order.Uint32(b) // 直接按端序取值，不翻转
	return val, b, nil
}

// 带 QoS 和 超时的发布：Data 以十进制字符串输出（"12345"）
// 带 QoS 和超时的发布：Data 以十进制字符串输出（"12345"）
func PublishSinkCommandWithQoS(
	client mqtt.Client,
	topic, typ, eid, data string,
	qos byte,
	timeout time.Duration,
) error {
	fmt.Printf("99999999999999999999999999999")
	// 0) 连接与入参快速校验
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt not connected")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return fmt.Errorf("empty topic")
	}
	t := strings.TrimSpace(typ)
	if t == "" {
		t = "sink"
	}
	fmt.Printf("8888888888888888888888888888")
	// 1) 解析 data（允许带空格/常见格式），并给出十进制字符串
	val, raw, err := parseDataToUint32(strings.TrimSpace(data), binary.LittleEndian)
	if err != nil {
		return fmt.Errorf("invalid data %q: %w", data, err)
	}
	dataStr := strconv.FormatUint(uint64(val), 10) // 统一十进制字符串
	fmt.Printf("7777777777777777777777777777777")
	// 2) 组装 Payload（Datalen 必须与 Data 一致）
	sp := SinkPayload{
		Type:      t,
		Eid:       eid,
		Timestamp: uint64(time.Now().Unix()),
		Datalen:   len(dataStr), // ✅ 与 Data 一致
		Data:      dataStr,
	}

	now := time.Now().UnixNano()
	env := EdgexMessage{
		ApiVersion:    "v3",
		CorrelationID: fmt.Sprintf("%s-%d", t, now),
		RequestID:     fmt.Sprintf("req-%d", now),
		ErrorCode:     0,
		Payload:       sp,
		ContentType:   "application/json",
	}
	fmt.Printf("66666666666666666666666666666666666666666")
	// 3) 编码并发布
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal edgex message: %w", err)
	}

	// 如需排查可把 retained 设为 true 方便 MQTTX 立刻看到：client.Publish(topic, qos, true, body)
	token := client.Publish(topic, qos, false, body)

	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("publish timeout: topic=%s qos=%d timeout=%s", topic, qos, timeout)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish error: %w", err)
	}
	fmt.Printf("5555555555555555555555555555555555")
	// 4) 成功日志（含十六进制原值，便于核对）
	log.Printf("[PUB] ok → topic=%q qos=%d payload=%dB dec=%d hex=%s",
		topic, qos, len(body), val, strings.ToUpper(hex.EncodeToString(raw)))

	// 你的调试打印
	fmt.Printf("1111111111")
	return nil
}
