package relay

import (
	"encoding/hex"
	"strings"

	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

func SendFrame(dstAddr string, payload []byte) {
	eidStr := "238A0841D828"

	hexStr1 := strings.ToUpper(hex.EncodeToString(payload))

	mqttclient.PublishSinkCommand(mqttclient.MqttClient, "edgex/server/response/device_wiresink/down", eidStr, hexStr1)
}
