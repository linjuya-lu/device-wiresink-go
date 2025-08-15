package driver

import (
	"log"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

// 监听某设备（控制命令）的变化。
// deviceName: 逻辑设备名；resourceName: 要监控的控制资源名；
// pollInterval: 轮询间隔；
// handler: 新值到来时执行的回调。
func StartControlListener(deviceName, resourceName string, pollInterval time.Duration, handler func(newVal interface{})) {
	go func() {
		var last interface{}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for range ticker.C {
			vals, ok := config.GetDeviceValues(deviceName)
			if !ok {
				// 还没初始化
				continue
			}
			cur, exists := vals[resourceName]
			if !exists {
				// 资源不存在
				continue
			}
			// 值发生变化，且与上次不同
			if cur != last {
				last = cur
				log.Printf("[ControlListener] %s.%s 变为 %v", deviceName, resourceName, cur)
				// 回调
				handler(cur)
				// 值“清回”0，等待下次再次写入触发
				config.SetDeviceValue(deviceName, resourceName, 0)
				log.Printf("[ControlListener] %s.%s 已重置为 0", deviceName, resourceName)
			}
		}
	}()
}
