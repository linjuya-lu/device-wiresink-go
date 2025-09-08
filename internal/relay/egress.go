package relay

import (
	"encoding/hex"
	"strings"

	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

// 常量主题
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
