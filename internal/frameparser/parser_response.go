package frameparser

import (
	"log"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

// 控制帧解析
func handleFrameCtl(frameCtl config.Frame) {
	raw := frameCtl.Payload
	if len(raw) < 1 {
		log.Printf("控制帧长度不足，错误")
		return
	}
	// 解析
	head := raw[0]
	if handle, ok := config.LookupResponseHandle(head); ok {
		if err := handle.Parse(raw[1:], frameCtl); err != nil {
			log.Printf("参数解析失败 head=0x%02X: %v", head, err)
		}
	} else {
		log.Printf("未找到解析函数 head=0x%02X", head)
	}
}
