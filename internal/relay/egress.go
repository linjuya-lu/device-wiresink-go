package relay

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

// 主题
const downTopic = "edgex/server/response/device_wiresink/down"

func SendFrame(typ, srcAddr string, payload []byte) error {
	hexStr := strings.ToUpper(hex.EncodeToString(payload))
	return mqttclient.PublishSinkCommand(
		mqttclient.MqttClient,
		downTopic,
		typ,
		srcAddr,
		hexStr,
	)
}

// 可选 QoS/超时
func SendFrameWithQoS(typ, srcAddr string, payload []byte, qos byte, timeout time.Duration) error {
	hexStr := strings.ToUpper(hex.EncodeToString(payload))
	return mqttclient.PublishSinkCommandWithQoS(
		mqttclient.MqttClient,
		downTopic,
		typ,
		srcAddr,
		hexStr,
		qos,
		timeout,
	)
}
