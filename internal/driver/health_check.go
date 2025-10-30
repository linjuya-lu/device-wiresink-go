package driver

import (
	"fmt"
	"time"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
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

			// 获取设备名
			config.Mu.RLock()
			deviceNames := make([]string, 0, len(config.ValuesMap))
			for dev := range config.ValuesMap {
				deviceNames = append(deviceNames, dev)
			}
			config.Mu.RUnlock()

			nowNs := time.Now().UnixNano() // 时间戳

			for _, dev := range deviceNames {

				// 读取这个设备的运行期状态字典
				vals, ok := config.GetDeviceValues(dev)
				if !ok || vals == nil {
					continue
				}

				// 读取上次时间戳
				lastTs, okTs := config.GetLastDataTs(dev)

				// period 还是从设备状态里拿
				rawPr, okPr := vals["period"]

				fmt.Printf("dev=%s lastTs(ns)=%v , period(raw)=%v\n",
					dev, lastTs, rawPr)

				if !okTs || !okPr {
					//无法判断在线/离线
					continue
				}

				period, okPeriod := rawPr.(uint16)
				if !okPeriod || period == 0 {
					fmt.Printf("dev=%s period无效或为0，无法判定心跳\n", dev)
					continue
				}

				// 时间差
				elapsed := time.Duration(nowNs - lastTs)
				// 离线门槛：2倍采集周期
				deadline := 2 * time.Duration(period) * time.Second

				// 打印用秒
				elapsedSec := int64(elapsed.Round(time.Second) / time.Second)
				deadlineSec := int64(deadline.Round(time.Second) / time.Second)

				fmt.Printf("dev=%s elapsed=%ds deadline=%ds\n",
					dev, elapsedSec, deadlineSec)

				// 判定在线状态
				state := StateOnline
				if elapsed >= deadline || elapsed < 0 {
					state = StateOffline
				}

				// 写回当前心跳状态到设备值表（让别的逻辑可以读到）
				config.SetDeviceValue(dev, "heatbeat", state)
			}
		}
	}()
}
