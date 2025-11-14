package driver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
)

// 后台调用方式：
// POST /custom/firmware-upgrade
// Content-Type: multipart/form-data
// form fields:
//
//	deviceName: 目标设备名（必填）
//	file:       固件文件（二进制，必填）
//	version:    可选，固件版本号，仅用于记录
func (d *WireSinkDriver) handleFirmwareUpgrade(c echo.Context) error {
	deviceName := c.FormValue("deviceName")
	if deviceName == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "missing deviceName",
		})
	}
	// 读取固件文件
	fh, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "missing firmware file: " + err.Error(),
		})
	}
	src, err := fh.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "open firmware file failed: " + err.Error(),
		})
	}
	defer src.Close()

	// 固件保存目录：<可执行文件所在目录>/res/updata
	exePath, err := os.Executable()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "cannot get executable path: " + err.Error(),
		})
	}

	baseDir := filepath.Dir(exePath)
	dir := filepath.Join(baseDir, "res", "updata")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "create upgrade dir failed: " + err.Error(),
		})
	}

	// 保存为 deviceName+时间戳，避免冲突
	filename := fmt.Sprintf("%s-%d-%s", deviceName, time.Now().Unix(), filepath.Base(fh.Filename))
	savePath := filepath.Join(dir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "save firmware failed: " + err.Error(),
		})
	}
	if _, err = io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(savePath)
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "write firmware failed: " + err.Error(),
		})
	}
	_ = dst.Close()

	// 记录一下该设备使用哪个固件文件（供 startUpgradeAsync 使用）
	if d.upgradeFiles == nil {
		d.upgradeFiles = make(map[string]string)
	}
	d.upgradeFiles[deviceName] = savePath

	// 启动异步升级（内部从 d.upgradeFiles[deviceName] 读取固件，按你的协议分片发送）
	if err := d.startUpgradeAsync(deviceName); err != nil {
		d.lc.Errorf("startUpgradeAsync(%s) failed: %v", deviceName, err)
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "start upgrade failed: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":         true,
		"deviceName": deviceName,
		"file":       filename,
	})
}
