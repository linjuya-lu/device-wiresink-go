package driver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	"github.com/labstack/echo/v4"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func (d *WireSinkDriver) handleLoadParamMap(c echo.Context) error {
	// Content-Type: multipart/form-data, 字段名为file
	fh, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "missing form file field 'file': " + err.Error(),
		})
	}
	src, err := fh.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "open uploaded file failed: " + err.Error(),
		})
	}
	defer src.Close()
	if err := config.LoadParamMapFromReader(src, fh.Filename); err != nil {
		d.lc.Errorf("LoadParamMapFromReader error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (d *WireSinkDriver) addCustomRoutes() error {
	if err := d.sdk.AddCustomRoute(
		"/custom/firmware-upgrade",
		interfaces.Unauthenticated,
		d.handleFirmwareUpgrade,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register firmware upgrade route failed: %w", err)
	}
	if err := d.sdk.AddCustomRoute(
		"/custom/load-param-map",
		interfaces.Unauthenticated,
		d.handleLoadParamMap,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register load-param-map route failed: %w", err)
	}
	if err := d.sdk.AddCustomRoute(
		"/custom/topology",
		interfaces.Unauthenticated,
		d.handleGetTopology,
		http.MethodGet,
	); err != nil {
		return fmt.Errorf("register topology route failed: %w", err)
	}
	return nil
}

func (d *WireSinkDriver) handleGetTopology(c echo.Context) error {
	topo := config.GetTopoList()
	fillDescIfPossible := func(node *config.NodeTopology) {
		devName, ok := config.LookupDeviceName(node.EID)
		if !ok {
			// 没映射：Desc 不填
			d.lc.Infof("handleGetTopology:eid=%s无EID映射，Desc不填充", node.EID)
			return
		}
		// 有映射
		dev, err := d.sdk.GetDeviceByName(devName)
		if err != nil {
			d.lc.Infof("handleGetTopology:根据设备名%s 获取设备失败:%v，Desc不填充", devName, err)
			return
		}
		// description为空
		desc := strings.TrimSpace(dev.Description)
		if desc == "" {
			d.lc.Infof("handleGetTopology:设备%s(eid=%s)未配置description，Desc不填充", devName, node.EID)
			return
		}
		node.Desc = desc
	}
	const (
		targetEID    = "238A08411011"
		expectType   = "4"
		expectState  = "1"
		expectParent = "FFFFFFFFFFFF"
	)

	found := false
	for i := range topo {
		fillDescIfPossible(&topo[i])
		// parent修正
		if topo[i].EID == targetEID &&
			topo[i].Type == expectType &&
			topo[i].State == expectState &&
			strings.ToUpper(topo[i].Parent) == expectParent {
			topo[i].Parent = config.GatewayEID
			found = true
			d.lc.Infof("handleGetTopology:修正根节点eid=%s的parent->%s", targetEID, config.GatewayEID)
		}
	}
	if !found {
		d.lc.Warnf(
			"handleGetTopology:未找到eid=%s,type=%s,state=%s,parent=%s的节点",
			targetEID, expectType, expectState, expectParent,
		)
	}
	d.lc.Infof("返回拓扑列表（已按规则填充 Desc），数量=%d", len(topo))
	return c.JSON(http.StatusOK, topo)
}
