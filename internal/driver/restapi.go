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

	const (
		targetEID    = "238A08411011"
		expectType   = "4"
		expectState  = "1"
		expectParent = "FFFFFFFFFFFF"
	)

	found := false
	for i := range topo {
		if topo[i].EID == targetEID &&
			topo[i].Type == expectType &&
			topo[i].State == expectState &&
			strings.ToUpper(topo[i].Parent) == expectParent {
			// 找到了就把 parent 改成汇聚网关 EID
			topo[i].Parent = config.GatewayEID // "238A0841D828"
			found = true
			break
		}
	}

	if !found {
		d.lc.Warnf(
			"handleGetTopology: 未找到 eid=%s, type=%s, state=%s, parent=%s 的节点",
			targetEID, expectType, expectState, expectParent,
		)
	}

	d.lc.Infof("返回拓扑列表，数量=%d", len(topo))
	return c.JSON(http.StatusOK, topo)
}
