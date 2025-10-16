package driver

import (
	"fmt"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

func startHealthCheckLoop() {
	go func() {
		const (
			StateOffline uint8 = 0
			StateOnline  uint8 = 1
		)
		const interval = 10 * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			config.Mu.RLock()
			deviceNames := make([]string, 0, len(config.ValuesMap))
			for dev := range config.ValuesMap {
				deviceNames = append(deviceNames, dev)
			}
			config.Mu.RUnlock()

			nowNs := time.Now().UnixNano() // 统一用纳秒
			for _, dev := range deviceNames {

				vals, ok := config.GetDeviceValues(dev)
				if !ok || vals == nil {
					continue
				}
				rawTs, okTs := vals["LastDataTs"]
				rawPr, okPr := vals["period"]

				fmt.Printf("dev=%s LastDataTs(raw)=%v , period(raw)=%v\n",
					dev, rawTs, rawPr)
				if !okTs || !okPr {
					continue
				}

				lastTs, ok1 := rawTs.(int64)
				period, ok2 := rawPr.(uint16)
				if !ok1 || !ok2 || period == 0 {
					fmt.Printf("未收到消息\n")
					continue
				}

				// 计算 elapsed/deadline（纳秒）
				elapsed := time.Duration(nowNs - lastTs)
				deadline := 2 * time.Duration(period) * time.Second

				// 四舍五入到秒用于打印
				elapsedSec := int64(elapsed.Round(time.Second) / time.Second)
				deadlineSec := int64(deadline.Round(time.Second) / time.Second)

				fmt.Printf("dev=%s elapsed=%ds) deadline=%ds\n)",
					dev, elapsedSec, deadlineSec)

				// 在线=1；超时/异常=0
				state := StateOnline
				if elapsed >= deadline || elapsed < 0 {
					state = StateOffline
				}
				// 写回状态
				config.SetDeviceValue(dev, "heatbeat", state)

			}
		}
	}()
}
