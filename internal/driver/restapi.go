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

	// 只在「有 EID 映射 + 有 description」时填充 Desc
	fillDescIfPossible := func(node *config.NodeTopology) {
		devName, ok := config.LookupDeviceName(node.EID)
		if !ok {
			d.lc.Infof("handleGetTopology: eid=%s 无 EID 映射，Desc 不填充", node.EID)
			return
		}

		dev, err := d.sdk.GetDeviceByName(devName)
		if err != nil {
			d.lc.Infof("handleGetTopology: 根据设备名 %s 获取设备失败: %v，Desc 不填充", devName, err)
			return
		}

		desc := strings.TrimSpace(dev.Description)
		if desc == "" {
			d.lc.Debugf("handleGetTopology: 设备 %s (eid=%s) 未配置 description，Desc 不填充", devName, node.EID)
			return
		}

		node.Desc = desc
	}

	// 特殊根节点修正：LoRa 汇聚节点挂到汇聚网关下
	const (
		targetEID    = "238A08411011"
		expectType   = "4"
		expectState  = "1"
		expectParent = "FFFFFFFFFFFF"
	)

	// 过滤：只保留“已注册 EID”的节点 + 本模块根节点
	filterByMapping := func(nodes []config.NodeTopology) []config.NodeTopology {
		filtered := make([]config.NodeTopology, 0, len(nodes))
		foundRoot := false

		for _, n := range nodes {
			node := n // 拷贝一份，避免直接改原切片元素

			// parent 修正
			if node.EID == targetEID &&
				node.Type == expectType &&
				node.State == expectState &&
				strings.ToUpper(node.Parent) == expectParent {

				node.Parent = config.EidStr
				foundRoot = true
				d.lc.Infof("handleGetTopology: 修正根节点 eid=%s 的 parent -> %s", targetEID, config.EidStr)
			}

			// 1. 本模块自身根节点（EID == EidStr）强制保留
			if node.EID == config.EidStr {
				fillDescIfPossible(&node)
				filtered = append(filtered, node)
				continue
			}

			// 2. 其他节点：必须已经做过 EID 映射才保留
			if _, ok := config.LookupDeviceName(node.EID); !ok {
				d.lc.Debugf("handleGetTopology: 丢弃未注册节点 eid=%s（无 EID 映射）", node.EID)
				continue
			}

			// 有映射 → 保留，并按规则尝试填 Desc
			fillDescIfPossible(&node)
			filtered = append(filtered, node)
		}

		if !foundRoot {
			d.lc.Warnf(
				"handleGetTopology: 未找到 eid=%s, type=%s, state=%s, parent=%s 的节点",
				targetEID, expectType, expectState, expectParent,
			)
		}

		return filtered
	}

	filtered := filterByMapping(topo)
	d.lc.Infof("返回拓扑列表（仅已注册 EID，且按规则填充 Desc），原始=%d，过滤后=%d",
		len(topo), len(filtered))

	return c.JSON(http.StatusOK, filtered)
}
