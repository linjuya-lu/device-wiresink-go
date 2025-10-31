package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
)

var (
	Mu sync.RWMutex
	//设备名称 → (资源名称 → 值)
	ValuesMap = make(map[string]map[string]interface{})
)

// 默认值转化
func ParseDefaultValue(valStr, vt string) interface{} {
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

// 写入资源
func SetDeviceValue(deviceName, resourceName string, value interface{}) {
	Mu.Lock()
	defer Mu.Unlock()
	if _, ok := ValuesMap[deviceName]; !ok {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	ValuesMap[deviceName][resourceName] = value
}

// 获取资源
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

// 获取所有资源
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

// 更新资源
func DeviceInit(deviceName, resourceName, defaultValue, valueType string) error {
	Mu.Lock()
	defer Mu.Unlock()
	if _, exists := ValuesMap[deviceName]; !exists {
		ValuesMap[deviceName] = make(map[string]interface{})
	}
	parsedValue := ParseDefaultValue(defaultValue, valueType)
	ValuesMap[deviceName][resourceName] = parsedValue
	return nil
}

// 删除资源
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
