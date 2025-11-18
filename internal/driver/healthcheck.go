package driver

import (
	"time"
)

func (d *WireSinkDriver) startHealthCheckLoop() {
	go func() {
		const interval = time.Minute // 每分钟触发一次
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := d.handleRouterParameterQuery(); err != nil {
				d.lc.Errorf("handleRouterParameterQuery failed: %v", err)
			}
		}
	}()
}
