package relay

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

func SendFrame(srcAddr string, payload []byte) {

	hexStr1 := strings.ToUpper(hex.EncodeToString(payload))

	mqttclient.PublishSinkCommand(mqttclient.MqttClient, "edgex/server/response/device-wiresink/down", srcAddr, hexStr1)
}

type EdgexMessage struct {
	ApiVersion    string      `json:"apiVersion"`
	ReceivedTopic string      `json:"receivedTopic,omitempty"`
	CorrelationID string      `json:"correlationID"`
	RequestID     string      `json:"requestID"`
	ErrorCode     int         `json:"errorCode"`
	Payload       interface{} `json:"payload"`
	ContentType   string      `json:"contentType"`
}

type SinkPayload struct {
	Type      string `json:"Type"`
	Eid       string `json:"Eid"`
	Timestamp uint64 `json:"Timestamp"`
	Datalen   int    `json:"Datalen"`
	Data      string `json:"Data"`
}

func SendPortDecWithQoS(
	client mqtt.Client,
	topic, typ, eid string,
	port uint32,
	qos byte,
	timeout time.Duration,
) error {
	sp := SinkPayload{
		Type:      typ,
		Eid:       eid,
		Timestamp: uint64(time.Now().Unix()),
		Datalen:   5,
		Data:      fmt.Sprintf("%d", port), //  十进制字符串
	}

	now := time.Now().UnixNano()
	env := EdgexMessage{
		ApiVersion:    "v3",
		CorrelationID: fmt.Sprintf("%s-%d", typ, now),
		RequestID:     fmt.Sprintf("req-%d", now),
		ErrorCode:     0,
		Payload:       sp,
		ContentType:   "application/json",
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal edgex message: %w", err)
	}

	token := client.Publish(topic, qos, false, body)
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("publish timeout: topic=%s qos=%d timeout=%s", topic, qos, timeout)
	}
	return token.Error()
}
