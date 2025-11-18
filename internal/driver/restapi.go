package driver

import (
	"fmt"
	"net/http"

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

// 初始化自定义路由
func (d *WireSinkDriver) addCustomRoutes() error {
	if err := d.sdk.AddCustomRoute(
		"/custom/firmware-upgrade",
		interfaces.Unauthenticated,
		d.handleFirmwareUpgrade,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register firmware upgrade route failed: %w", err)
	}

	// 已有的配置下发接口
	if err := d.sdk.AddCustomRoute(
		"/custom/load-param-map",
		interfaces.Unauthenticated,
		d.handleLoadParamMap,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register load-param-map route failed: %w", err)
	}

	// 新增：获取拓扑列表接口
	if err := d.sdk.AddCustomRoute(
		"/custom/topology",
		interfaces.Unauthenticated, // 不需要 EdgeX 鉴权的话就用 Unauthenticated
		d.handleGetTopology,        // 新写的 handler
		http.MethodGet,
	); err != nil {
		return fmt.Errorf("register topology route failed: %w", err)
	}

	return nil
}

// handleGetTopology 返回当前内存中的路由拓扑列表
func (d *WireSinkDriver) handleGetTopology(c echo.Context) error {
	// 从 config 中获取一份拷贝，内部已经加锁
	topo := config.GetTopoList()

	// 也可以加一条日志，方便排查
	d.lc.Infof("返回拓扑列表，数量=%d", len(topo))

	// 直接把 []NodeTopology 作为 JSON 返回给后台
	return c.JSON(http.StatusOK, topo)
}
