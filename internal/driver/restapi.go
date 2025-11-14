package driver

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func (d *WireSinkDriver) handleLoadParamMap(c echo.Context) error {
	// 支持 multipart: form-data, 字段名必须是 "file"
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
