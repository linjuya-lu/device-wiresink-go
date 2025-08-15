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

// 表示 devices.yaml 中的单个设备条目
// 包含设备逻辑名称和对应的 Profile 名称
type DeviceEntry struct {
	Name        string `yaml:"name"`
	ProfileName string `yaml:"profileName"`
}

// 对应 devices.yaml 的顶层结构
type devicesYAML struct {
	DeviceList []DeviceEntry `yaml:"deviceList"`
}

// 保存设备资源属性配置
// 包含值类型、权限、单位和默认值等
type ResourceProperty struct {
	ValueType    string `yaml:"valueType"`
	ReadWrite    string `yaml:"readWrite"`
	Units        string `yaml:"units"`
	DefaultValue string `yaml:"defaultValue"`
}

// 对应 Profile 文件中的单个资源条目
// 包含名称、隐藏标志、描述和属性字段
type DeviceResource struct {
	Name        string           `yaml:"name"`
	IsHidden    bool             `yaml:"isHidden"`
	Description string           `yaml:"description"`
	Properties  ResourceProperty `yaml:"properties"`
}

// 对应 Profile 文件顶层
type profileYAML struct {
	DeviceResources []DeviceResource `yaml:"deviceResources"`
}

var (
	Mu sync.RWMutex
	// 存储所有设备的静态资源定义，key 为设备逻辑名称
	resourcesMap = make(map[string][]DeviceResource)
	//存储所有设备的运行时资源值，key: 设备名称 → (资源名称 → value)
	ValuesMap = make(map[string]map[string]interface{})
)

// 根据 ValueType 将 DefaultValue 字符串转换为对应类型
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

// 初始化静态资源定义及默认运行时值：
// 1. 读取并解析 devices.yaml，获取所有设备条目
// 2. 遍历每个 entry，根据 ProfileName 加载 Profile 文件，解析 deviceResources
// 3. 填充全局 maps，并将 DefaultValue 作为初始值写入 valuesMap
func InitDeviceResources(devicesPath, profilesDir string) error {
	// 读取 devices.yaml
	raw, err := os.ReadFile(devicesPath)
	if err != nil {
		return fmt.Errorf("无法读取设备列表文件 %s：%w", devicesPath, err)
	}
	// 解析 YAML
	var devs devicesYAML
	if err := yaml.Unmarshal(raw, &devs); err != nil {
		return fmt.Errorf("解析 devices.yaml 失败：%w", err)
	}
	Mu.Lock()
	defer Mu.Unlock()
	// 加载并写入静态资源和默认值表
	for _, entry := range devs.DeviceList {
		profileFile := filepath.Join(profilesDir, entry.ProfileName+".yaml")
		rawProfile, err := os.ReadFile(profileFile)
		if err != nil {
			return fmt.Errorf("无法读取 Profile 文件 %s：%w", profileFile, err)
		}
		var prof profileYAML
		if err := yaml.Unmarshal(rawProfile, &prof); err != nil {
			return fmt.Errorf("解析 Profile 文件 %s 失败：%w", profileFile, err)
		}
		// 保存定义
		resourcesMap[entry.Name] = prof.DeviceResources
		// 初始化运行时值为 DefaultValue
		ValuesMap[entry.Name] = make(map[string]interface{}, len(prof.DeviceResources))
		for _, dr := range prof.DeviceResources {
			ValuesMap[entry.Name][dr.Name] = parseDefaultValue(dr.Properties.DefaultValue, dr.Properties.ValueType)
		}
	}
	return nil
}

// 获取指定设备的资源列表
// 返回值: []DeviceResource, bool(是否存在)
func GetDeviceResources(deviceName string) ([]DeviceResource, bool) {
	Mu.RLock()
	defer Mu.RUnlock()
	res, ok := resourcesMap[deviceName]
	return res, ok
}

// 写入解析后的单个资源值
func SetDeviceValue(deviceName, resourceName string, value interface{}) {
	Mu.Lock()
	defer Mu.Unlock()
	if _, ok := ValuesMap[deviceName]; !ok {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	ValuesMap[deviceName][resourceName] = value
}

// 获取指定设备的单个资源值
// 返回值: interface{}, bool(是否存在)
func GetDeviceValue(deviceName, resourceName string) (interface{}, bool) {
	Mu.RLock()
	defer Mu.RUnlock()
	//  设备是否存在
	deviceValues, ok := ValuesMap[deviceName]
	if !ok {
		return nil, false
	}
	// 资源是否存在并返回值
	value, exists := deviceValues[resourceName]
	return value, exists
}

// 获取指定设备的所有运行时资源值
// 返回值: map[resourceName]value, bool(是否存在)
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

// 初始化设备资源并设置正确类型的默认值
func DeviceInit(deviceName, resourceName, defaultValue, valueType string) error {
	Mu.Lock()
	defer Mu.Unlock()
	// 确保有对应映射
	if _, exists := ValuesMap[deviceName]; !exists {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	// 转换默认值
	parsedValue := parseDefaultValue(defaultValue, valueType)
	ValuesMap[deviceName][resourceName] = parsedValue
	return nil
}

// 删除指定设备的所有运行时值
func DeleteDeviceValues(deviceName string) error {
	Mu.Lock()
	defer Mu.Unlock()
	// 设备是否存在
	if _, exists := ValuesMap[deviceName]; !exists {
		return fmt.Errorf("设备 %s 不存在于运行时值表中", deviceName)
	}
	// 删除设备有运行值
	delete(ValuesMap, deviceName)
	return nil
}

// 删除指定设备的所有传感器ID映射
func DeleteSensorIDMappingsByDevice(deviceName string) error {
	// 遍历映射，删除所有指向该设备的条目
	toDelete := make([]string, 0)
	for sensorID, mappedDeviceName := range SensorIDToDeviceName {
		if mappedDeviceName == deviceName {
			toDelete = append(toDelete, sensorID)
		}
	}
	// 删除找到的映射
	for _, sensorID := range toDelete {
		delete(SensorIDToDeviceName, sensorID)
	}
	return nil
}
