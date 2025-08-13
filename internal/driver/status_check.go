package driver

import (
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

func startHealthCheckLoop() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now().UnixNano() // 当前系统时间，纳秒

			// 1. 读取所有设备名称（并发安全）
			config.Mu.RLock()
			deviceNames := make([]string, 0, len(config.ValuesMap))
			for dev := range config.ValuesMap {
				deviceNames = append(deviceNames, dev)
			}
			config.Mu.RUnlock()

			// 2. 逐个检查每台设备
			for _, dev := range deviceNames {
				// 并发安全地读取 lastDataTimestamp 和 period
				rawTs, okTs := config.GetDeviceValue(dev, "lastDataTimestamp")
				rawPr, okPr := config.GetDeviceValue(dev, "period")
				if !okTs || !okPr {
					continue
				}

				// 类型断言
				lastTs, ok1 := rawTs.(int64)
				period, ok2 := rawPr.(uint32)
				if !ok1 || !ok2 {
					continue
				}

				// 判断是否超时： now - lastTs > 2 * period 秒
				// period 单位是秒，要换算成纳秒
				deadline := int64(period) * 2 * int64(time.Second)
				newState := uint8(0)
				if now-lastTs > deadline {
					newState = 1
				}

				// 并发安全地写回 state
				config.SetDeviceValue(dev, "state", newState)
			}
		}
	}()
}
