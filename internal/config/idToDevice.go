package config

import (
	"fmt"
	"sync"
)

// sensorIDToDeviceName 是传感器 6 字节 ID（大写十六进制）到本地逻辑设备名的映射
var (
	mu1                  sync.RWMutex
	SensorIDToDeviceName = map[string]string{
		"238A08262319": "Data-Demo",
	}
)

// AddMapping 添加一条 SensorID -> DeviceName 映射，若存在则覆盖
func AddMapping(sensorID, deviceName string) {
	mu1.Lock()
	defer mu1.Unlock()
	SensorIDToDeviceName[sensorID] = deviceName
	fmt.Printf("Mapping added: %s -> %s\n", sensorID, deviceName)
}

// DeleteMapping 删除指定 SensorID 的映射，若不存在返回错误
func DeleteMapping(sensorID string) error {
	mu1.Lock()
	defer mu1.Unlock()
	if _, ok := SensorIDToDeviceName[sensorID]; !ok {
		return fmt.Errorf("no mapping found for SensorID %s", sensorID)
	}
	delete(SensorIDToDeviceName, sensorID)
	fmt.Printf("Mapping deleted: %s\n", sensorID)
	return nil
}

// UpdateMapping 更新指定 SensorID 的 DeviceName，若不存在返回错误
func UpdateMapping(sensorID, newDeviceName string) error {
	mu1.Lock()
	defer mu1.Unlock()
	if _, ok := SensorIDToDeviceName[sensorID]; !ok {
		return fmt.Errorf("no mapping found for SensorID %s", sensorID)
	}
	SensorIDToDeviceName[sensorID] = newDeviceName
	fmt.Printf("Mapping updated: %s -> %s\n", sensorID, newDeviceName)
	return nil
}

// LookupDeviceName 根据大写十六进制的 SensorID 返回逻辑设备名
func LookupDeviceName(sensorID string) (deviceName string, ok bool) {
	mu1.RLock()
	defer mu1.RUnlock()
	deviceName, ok = SensorIDToDeviceName[sensorID]
	return
}

// UpdateSensorMapping 扫描 valuesMap，把资源名为 "EID" 的值映射到设备名
func UpdateSensorMapping() {
	mu1.Lock()
	defer mu1.Unlock()

	// 清空旧映射
	SensorIDToDeviceName = make(map[string]string)

	for deviceName, resourceMap := range ValuesMap {
		raw, exists := resourceMap["eid"]
		if !exists {
			continue
		}
		var eid string
		switch v := raw.(type) {
		case string:
			eid = v
		case []byte:
			eid = string(v)
		default:
			// 其他类型统一用 fmt.Sprint 转成字符串
			eid = fmt.Sprint(v)
		}
		if eid == "" {
			continue
		}
		SensorIDToDeviceName[eid] = deviceName
	}
}
