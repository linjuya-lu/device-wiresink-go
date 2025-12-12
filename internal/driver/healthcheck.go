package driver

import (
	"strconv"
	"strings"
	"time"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func (d *WireSinkDriver) startHealthCheckLoop() {
	go func() {
		const interval = time.Minute // 每分钟跑一轮
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			//路由参数查询（AT+TOP）
			if err := d.handleRouterParameterQuery(); err != nil {
				d.lc.Errorf("handleRouterParameterQuery failed: %v", err)
				continue
			}

			time.Sleep(30 * time.Second)

			//异步上报设备的 state
			d.reportDevicesStateFromTopo()
		}
	}()
}

// 遍历路由表，根据 Topology.State 字段给每个设备异步上报 state 资源
func (d *WireSinkDriver) reportDevicesStateFromTopo() {
	topo := config.GetTopoList()
	if len(topo) == 0 {
		d.lc.Debug("healthCheck: TopoList 为空，跳过 state 上报")
		return
	}

	for _, n := range topo {
		// EID -> deviceName
		deviceName, ok := config.LookupDeviceName(n.EID)
		if !ok {
			d.lc.Debugf("healthCheck: EID=%s 未绑定 EdgeX 设备，跳过", n.EID)
			continue
		}

		// 解析 online 状态：1=在线，0=离线
		s := strings.TrimSpace(n.State)
		if s == "" {
			d.lc.Warnf("healthCheck: 设备 %s (EID=%s) state 为空，跳过", deviceName, n.EID)
			continue
		}

		u, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			d.lc.Warnf("healthCheck: 设备 %s (EID=%s) state=%q 解析失败: %v",
				deviceName, n.EID, n.State, err)
			continue
		}
		stateVal := uint8(u) // 0 或 1，对应 profile 里的 Uint8

		// 组装异步上报数据，只上报一个资源：state
		values := map[string]any{
			"state": stateVal,
		}

		d.AsyncReporting(deviceName, "state", values)
	}
}
