package driver

import (
	"time"

	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

func (d *WireSinkDriver) AsyncReporting(deviceName string, sourceName string, values map[string]any) {
	if len(values) == 0 {
		d.lc.Debugf("异步上传没有要上报的值")
		return
	}

	var cvs []*dsModels.CommandValue
	origin := time.Now().UnixNano()
	for name, val := range values {
		d.lc.Infof("一次异步上传: 设备=%s 资源=%s  值=%v", deviceName, name, val)

		var cv *dsModels.CommandValue
		var err error
		switch v := val.(type) {
		case int16:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeInt16, v)
		case int32:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeInt32, v)
		case int64:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeInt64, v)
		case uint8:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeUint8, v)
		case uint16:
			cv, err = dsModels.NewCommandValue(name, common.ValueTypeUint16, v)
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
}
