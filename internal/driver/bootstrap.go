package driver

import (
	"fmt"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

// 初始化默认值
func InitDeviceValues(sdk interfaces.DeviceServiceSDK) error {
	allDevices := sdk.Devices()

	config.Mu.Lock()
	defer config.Mu.Unlock()

	for _, dev := range allDevices {
		devName := dev.Name
		profName := dev.ProfileName
		if profName == "" {
			return fmt.Errorf("device %s 没有关联 profileName", devName)
		}

		//获取Profile
		prof, err := sdk.GetProfileByName(profName)
		if err != nil {
			return fmt.Errorf("获取 Profile %s 失败: %w", profName, err)
		}

		devValueMap := make(map[string]interface{}, len(prof.DeviceResources))

		for _, dr := range prof.DeviceResources {
			defVal := dr.Properties.DefaultValue
			valType := dr.Properties.ValueType
			devValueMap[dr.Name] = config.ParseDefaultValue(defVal, valType)
		}

		config.ValuesMap[devName] = devValueMap
	}

	return nil
}
