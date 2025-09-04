package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// 单个设备条目
type DeviceEntry struct {
	Name        string `yaml:"name"`
	ProfileName string `yaml:"profileName"`
}

// 设备表
type devicesYAML struct {
	DeviceList []DeviceEntry `yaml:"deviceList"`
}

// 资源属性
type ResourceProperty struct {
	ValueType    string `yaml:"valueType"`
	ReadWrite    string `yaml:"readWrite"`
	Units        string `yaml:"units"`
	DefaultValue string `yaml:"defaultValue"`
}

// 设备资源
type DeviceResource struct {
	Name        string           `yaml:"name"`
	IsHidden    bool             `yaml:"isHidden"`
	Description string           `yaml:"description"`
	Properties  ResourceProperty `yaml:"properties"`
}

// 设备资源表
type profileYAML struct {
	DeviceResources []DeviceResource `yaml:"deviceResources"`
}

var (
	Mu sync.RWMutex
	// 键为设备名
	resourcesMap = make(map[string][]DeviceResource)
	//设备名称 → (资源名称 → 值)
	ValuesMap = make(map[string]map[string]interface{})
)

// 默认值转化
func parseDefaultValue(valStr, vt string) interface{} {
	switch vt {
	case "Float32":
		if f, err := strconv.ParseFloat(valStr, 32); err == nil {
			return float32(f)
		}
	case "Uint16":
		if u, err := strconv.ParseUint(valStr, 10, 16); err == nil {
			return uint16(u)
		}
	case "Uint8":
		if u, err := strconv.ParseUint(valStr, 10, 8); err == nil {
			return uint8(u)
		}
	case "Bool":
		if b, err := strconv.ParseBool(valStr); err == nil {
			return b
		}
	case "Float32Array":
		var arr []float32
		if err := json.Unmarshal([]byte(valStr), &arr); err == nil {
			return arr
		}
	case "Object":
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(valStr), &obj); err == nil {
			return obj
		}
	}
	return valStr
}

// 初始化静态资源
func InitDeviceResources(devicesPath, profilesDir string) error {

	raw, err := os.ReadFile(devicesPath)
	if err != nil {
		return fmt.Errorf("无法读取设备列表文件 %s：%w", devicesPath, err)
	}

	var devs devicesYAML
	if err := yaml.Unmarshal(raw, &devs); err != nil {
		return fmt.Errorf("解析 devices.yaml 失败：%w", err)
	}
	Mu.Lock()
	defer Mu.Unlock()
	// 加载并写入静态资源和默认值
	for _, entry := range devs.DeviceList {
		profileFile := filepath.Join(profilesDir, entry.ProfileName+".yaml")
		rawProfile, err := os.ReadFile(profileFile)
		if err != nil {
			return fmt.Errorf("InitDeviceResources 无法读取 Profile 文件 %s：%w", profileFile, err)
		}
		var prof profileYAML
		if err := yaml.Unmarshal(rawProfile, &prof); err != nil {
			return fmt.Errorf("InitDeviceResources 解析 Profile 文件 %s 失败：%w", profileFile, err)
		}
		resourcesMap[entry.Name] = prof.DeviceResources
		ValuesMap[entry.Name] = make(map[string]interface{}, len(prof.DeviceResources))
		for _, dr := range prof.DeviceResources {
			ValuesMap[entry.Name][dr.Name] = parseDefaultValue(dr.Properties.DefaultValue, dr.Properties.ValueType)
		}
	}
	return nil
}

// 获取设备资源
func GetDeviceResources(deviceName string) ([]DeviceResource, bool) {
	Mu.RLock()
	defer Mu.RUnlock()
	res, ok := resourcesMap[deviceName]
	return res, ok
}

// 写入单个资源值
func SetDeviceValue(deviceName, resourceName string, value interface{}) {
	Mu.Lock()
	defer Mu.Unlock()
	if _, ok := ValuesMap[deviceName]; !ok {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	ValuesMap[deviceName][resourceName] = value
}

// 获取设备单个资源值
func GetDeviceValue(deviceName, resourceName string) (interface{}, bool) {
	Mu.RLock()
	defer Mu.RUnlock()
	deviceValues, ok := ValuesMap[deviceName]
	if !ok {
		return nil, false
	}
	value, exists := deviceValues[resourceName]
	return value, exists
}

// 获取指定设备所有资源值
func GetDeviceValues(deviceName string) (map[string]interface{}, bool) {
	Mu.RLock()
	defer Mu.RUnlock()
	vals, ok := ValuesMap[deviceName]
	if !ok {
		return nil, false
	}

	copyMap := make(map[string]interface{}, len(vals))
	for k, v := range vals {
		copyMap[k] = v
	}
	return copyMap, true
}

// 初始化设备资源并设置默认值
func DeviceInit(deviceName, resourceName, defaultValue, valueType string) error {
	Mu.Lock()
	defer Mu.Unlock()
	if _, exists := ValuesMap[deviceName]; !exists {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	parsedValue := parseDefaultValue(defaultValue, valueType)
	ValuesMap[deviceName][resourceName] = parsedValue
	return nil
}

// 删除指定设备资源值
func DeleteDeviceValues(deviceName string) error {
	Mu.Lock()
	defer Mu.Unlock()
	if _, exists := ValuesMap[deviceName]; !exists {
		return fmt.Errorf("DeleteDeviceValues 设备 %s 不存在于运行时值表中", deviceName)
	}
	delete(ValuesMap, deviceName)
	return nil
}

// 删除设备ID映射
func DeleteSensorIDMappingsByDevice(deviceName string) error {
	toDelete := make([]string, 0)
	for sensorID, mappedDeviceName := range SensorIDToDeviceName {
		if mappedDeviceName == deviceName {
			toDelete = append(toDelete, sensorID)
		}
	}
	for _, sensorID := range toDelete {
		delete(SensorIDToDeviceName, sensorID)
	}
	return nil
}
