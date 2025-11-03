package driver

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

// 请求体支持两种方式：
// 1) JSON: {"path":"/Emd/conf/param_map.xlsx"}
// 2) multipart: form-data 里直接上传文件字段名 "file"
type loadParamReq struct {
	Path       string `json:"path"`
	Sheet      string `json:"sheet"`      // 可选：工作表名（你的解析函数若不需要可忽略）
	SkipHeader int    `json:"skipHeader"` // 可选：跳过表头行数（默认 1）
}

func (d *WireSinkDriver) handleLoadParamMap(c echo.Context) error {
	var req loadParamReq
	_ = c.Bind(&req)

	xlsxPath := req.Path
	if xlsxPath == "" {
		// 尝试从 multipart 读取文件
		fh, err := c.FormFile("file")
		if err == nil && fh != nil {
			src, err := fh.Open()
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			}
			defer src.Close()

			tmp := filepath.Join(os.TempDir(), "parammap-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".xlsx")
			dst, err := os.Create(tmp)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			}
			if _, err = io.Copy(dst, src); err != nil {
				_ = dst.Close()
				_ = os.Remove(tmp)
				return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			}
			_ = dst.Close()
			defer os.Remove(tmp)
			xlsxPath = tmp
		}
	}

	if xlsxPath == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"ok": false, "error": "missing excel path or multipart file",
		})
	}

	// 调用你现有的解析函数：internal/config.LoadParamMapFromExcel(excelPath string) error
	if err := config.LoadParamMapFromExcel(xlsxPath); err != nil {
		d.lc.Errorf("LoadParamMapFromExcel error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "path": xlsxPath})
	}

	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}
