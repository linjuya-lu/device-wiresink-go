package relay

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

func WriteFrame(port io.ReadWriteCloser, frame []byte) error {
	// 转成字符串
	payload := string(frame)
	// 调试
	fmt.Printf(">> 发送字符串: %q\n", payload)
	// 发送
	n, err := port.Write([]byte(payload))
	if err != nil {
		return fmt.Errorf("写入串口失败：%w", err)
	}
	if n != len(payload) {
		return fmt.Errorf("写入字节数不完整：%d/%d", n, len(payload))
	}
	return nil
}

func SendFrame(dstAddr string, payload []byte) {
	eidStr := "238A0841D828"
	// 逐字节格式化
	var parts []string
	for _, b := range payload {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	hexStr := strings.Join(parts, "") // 合并字符串
	// AT 命令
	cmd := fmt.Sprintf("\rAT+DTX=%s,%s\r\n", dstAddr, hexStr)
	// 调试
	fmt.Printf(">> Sending AT command: %s", cmd)
	// 发送
	hexStr1 := strings.ToUpper(hex.EncodeToString(payload))

	mqttclient.PublishSinkCommand(mqttclient.MqttClient, "edgex/server/response/device_wiresink/down", eidStr, hexStr1)
}
