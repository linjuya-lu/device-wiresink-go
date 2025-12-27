package driver

import (
	"strconv"
	"strings"
	"time"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func (d *WireSinkDriver) startHealthCheckLoop() {
	go func() {
		const interval = time.Minute
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			//路由参数查询（AT+TOP）
			if err := d.handleRouterParameterQuery(); err != nil {
				d.lc.Errorf("handleRouterParameterQuery failed: %v", err)
				continue
			}
			time.Sleep(30 * time.Second)
			d.reportDevicesStateFromTopo()
		}
	}()
}

// 心跳上传
func (d *WireSinkDriver) reportDevicesStateFromTopo() {
	topo := config.GetTopoList()
	if len(topo) == 0 {
		d.lc.Debug("healthCheck: TopoList 为空，跳过 state 上报")
		return
	}
	for _, n := range topo {
		deviceName, ok := config.LookupDeviceName(n.EID)
		if !ok {
			d.lc.Debugf("healthCheck: EID=%s 未绑定 EdgeX 设备，跳过", n.EID)
			continue
		}
		//解析状态：1=在线，0=离线
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
		stateVal := uint8(u)
		values := map[string]any{
			"state": stateVal,
		}
		d.AsyncReporting(deviceName, "state", values)
	}
}
