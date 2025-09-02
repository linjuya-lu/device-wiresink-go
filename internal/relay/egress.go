package relay

import (
	"encoding/hex"
	"strings"

	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

func SendFrame(srcAddr string, payload []byte) {

	hexStr1 := strings.ToUpper(hex.EncodeToString(payload))

	mqttclient.PublishSinkCommand(mqttclient.MqttClient, "edgex/server/response/device_wiresink/down", srcAddr, hexStr1)
}
