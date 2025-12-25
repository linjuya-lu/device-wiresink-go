package driver

import (
	"fmt"
	"strings"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

func InitDeviceValues(sdk interfaces.DeviceServiceSDK, lc logger.LoggingClient) error {
	allDevices := sdk.Devices()
	lc.Infof("InitDeviceValues: 共发现 %d 个设备", len(allDevices))
	for i, dev := range allDevices {
		devName := dev.Name
		lc.Infof("InitDeviceValues: [%d] DeviceName=%s Service=%s Profile=%s AutoEvents=%v Protocols=%v",
			i, dev.Name, dev.ServiceName, dev.ProfileName, dev.AutoEvents, dev.Protocols)
		//EID映射
		if eid, ok := extractEID(dev.Protocols); ok {
			config.AddMapping(eid, devName)
			lc.Infof("InitDeviceValues: 设备 %s EID=%s 已加入映射", devName, eid)
		} else {
			lc.Warnf("InitDeviceValues: 设备 %s 未提供 eid", devName)
		}
		//初始化默认值&LoRa映射
		profName := dev.ProfileName
		if profName == "" {
			lc.Warnf("InitDeviceValues: device %s 没有关联 profileName，跳过该设备初始化", devName)
			continue
		}
		prof, err := sdk.GetProfileByName(profName)
		if err != nil {
			lc.Errorf("InitDeviceValues: 获取 Profile %s 失败，跳过设备 %s: %v", profName, devName, err)
			continue
		}
		lc.Debugf("InitDeviceValues: device %s 使用 profile %s，资源数=%d",
			devName, profName, len(prof.DeviceResources))
		initOneDevice(devName, prof, lc)
		lc.Infof("InitDeviceValues: device %s 初始化完成", devName)
	}
	lc.Infof("InitDeviceValues: 所有已有设备初始化结束")
	return nil
}

func initOneDevice(deviceName string, prof models.DeviceProfile, lc logger.LoggingClient) {
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		//初始化默认值
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			lc.Infof("InitDeviceValues: 初始化设备 %s 资源 %s 失败：%v", deviceName, resName, err)
			continue
		}
		lc.Infof("InitDeviceValues: 已将设备 %s 的资源 %s 初始化: %s (类型: %s)",
			deviceName, resName, defaultValue, valueType)
		var featStr, typeStr string
		if dr.Attributes == nil {
			lc.Infof("InitDeviceValues: 资源 %s 未配置 attributes，跳过 LoRa 登记", resName)
			continue
		}
		rawLora, ok := dr.Attributes["lora"]
		if !ok || rawLora == nil {
			lc.Infof("InitDeviceValues: 资源 %s 未配置 attributes.lora，跳过 LoRa 登记", resName)
			continue
		}
		loraMap, ok := rawLora.(map[string]any)
		if !ok {
			lc.Infof("InitDeviceValues: 资源 %s attributes.lora 类型异常: %T，期望 map[string]any，跳过 LoRa 登记",
				resName, rawLora)
			continue
		}
		if v, ok := loraMap["paramFeatures"]; ok && v != nil {
			featStr = strings.TrimSpace(fmt.Sprint(v))
		}
		if v, ok := loraMap["paramType"]; ok && v != nil {
			typeStr = strings.TrimSpace(fmt.Sprint(v))
		}
		if featStr == "" || typeStr == "" {
			lc.Infof("InitDeviceValues: 资源 %s 的 lora.paramFeatures 或 lora.paramType 为空，跳过 LoRa 登记", resName)
			continue
		}
		featureBits, err1 := parseBin8(featStr)
		typeBits, err2 := parseBin16(typeStr)
		if err1 != nil || err2 != nil {
			lc.Warnf("InitDeviceValues: 资源 %s LoRa 二进制解析失败: feature=%q err=%v, type=%q err=%v，跳过 LoRa 登记",
				resName, featStr, err1, typeStr, err2)
			continue
		}
		key := config.ParamKey{
			FeatureBits: featureBits,
			CodeBits:    typeBits,
		}
		config.ParamEidAdd(key, deviceName, resName)
		lc.Infof("InitDeviceValues: ParamEidRegistry 登记: dev=%s res=%s -> Feature=%03b Code=%011b",
			deviceName, resName, featureBits, typeBits)
	}
}
