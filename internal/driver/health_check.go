package driver

import (
	"time"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func (d *WireSinkDriver) startHealthCheckLoop() {
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

			for _, dev := range deviceNames {

				// 读取这个设备的运行期状态字典
				vals, ok := config.GetDeviceValues(dev)
				if !ok || vals == nil {
					continue
				}

				state := StateOnline                          // 判定在线状态
				config.SetDeviceValue(dev, "heatbeat", state) // 写回心跳状态

				//触发拓扑查询
				if err := d.handleRouterParameterQuery(dev); err != nil {
					return
				}
			}

		}
	}()
}
