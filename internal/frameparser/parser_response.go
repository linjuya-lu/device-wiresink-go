package frameparser

import (
	"log"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

// handleFrameCtl 解析控制帧
func handleFrameCtl(frameCtl config.Frame) {
	raw := frameCtl.Payload // Payload 类型是 []byte
	if len(raw) < 1 {
		log.Printf("[CTL] payload 长度不足，跳过")
		return
	}
	// 解析第一个字节：高 7 位为 CtrlType，最低位为 RequestSetFlag
	head := raw[0]
	log.Printf("[CTL] head=0x%02X (%d)", head, head) // 16进制+十进制
	// 根据 head 查找解析函数
	if handle, ok := config.LookupResponseHandle(head); ok {
		if err := handle.Parse(raw[1:], frameCtl); err != nil {
			log.Printf("❌ 参数解析失败 head=0x%02X: %v", head, err)
		}
	} else {
		log.Printf("未找到解析函数 head=0x%02X", head)
	}
}
