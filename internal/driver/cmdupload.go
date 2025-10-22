package driver

import (
	"time"

	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

func (d *WireSinkDriver) AsyncReporting(deviceName string, sourceName string, values map[string]interface{}) {
	d.lc.Infof("异步上传值为%#v", values)

	if len(values) == 0 {
		d.lc.Debugf("异步上传没有要上报的值")
		return
	}

	var cvs []*dsModels.CommandValue
	origin := time.Now().UnixNano()

	for name, val := range values {
		d.lc.Infof("一次异步上传: 资源=%s 类型=%T 值=%v", name, val, val)

		var cv *dsModels.CommandValue
		var err error

		switch v := val.(type) {
		case int32:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeInt32, v)
		case int64:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeInt64, v)
		case float32:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeFloat32, v)
		case float64:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeFloat64, v)
		case string:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeString, v)
		default:
			d.lc.Infof("异步上传 不支持的类型: %T", v)
			continue
		}

		if err != nil {
			d.lc.Errorf("异步上传 值类型(%s) 错误: %v", name, err)
			continue
		}
		cv.Origin = origin
		cvs = append(cvs, cv)
	}

	if len(cvs) == 0 {
		d.lc.Warnf("异步上传: 没有有效值，跳过上报")
		return
	}

	asyncValues := &dsModels.AsyncValues{
		DeviceName:    deviceName,
		SourceName:    sourceName,
		CommandValues: cvs,
	}

	d.asyncCh <- asyncValues
	d.lc.Infof("异步值上传: 设备=%s 资源=%s 数量=%d",
		deviceName, sourceName, len(cvs))
}

// 心跳上传
func (d *WireSinkDriver) StartAsyncReporter() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			config.Mu.RLock()
			for deviceName, resMap := range config.ValuesMap {
				if resMap == nil {
					continue
				}

				stateVal, ok := resMap["state"]
				if !ok {
					continue
				}
				values := map[string]interface{}{
					"state": stateVal,
				}
				d.AsyncReporting(deviceName, "hBeat", values)
			}
			config.Mu.RUnlock()
		}
	}()
}
