package config

import (
	"fmt"
	"sync"
)

// 资源映射
var (
	mu1                  sync.RWMutex
	SensorIDToDeviceName = map[string]string{
		"238A08262319": "Data-Demo",
	}
)

// 添加一条映射
func AddMapping(sensorID, deviceName string) {
	mu1.Lock()
	defer mu1.Unlock()
	SensorIDToDeviceName[sensorID] = deviceName
	fmt.Printf("AddMapping Mapping added: %s -> %s\n", sensorID, deviceName)
}

// 删除指定映射
func DeleteMapping(sensorID string) error {
	mu1.Lock()
	defer mu1.Unlock()
	if _, ok := SensorIDToDeviceName[sensorID]; !ok {
		return fmt.Errorf("DeleteMapping no mapping found for SensorID %s", sensorID)
	}
	delete(SensorIDToDeviceName, sensorID)
	fmt.Printf("DeleteMapping Mapping deleted: %s\n", sensorID)
	return nil
}

// 更新指定映射
func UpdateMapping(sensorID, newDeviceName string) error {
	mu1.Lock()
	defer mu1.Unlock()
	if _, ok := SensorIDToDeviceName[sensorID]; !ok {
		return fmt.Errorf("UpdateMapping no mapping found for SensorID %s", sensorID)
	}
	SensorIDToDeviceName[sensorID] = newDeviceName
	fmt.Printf("UpdateMapping Mapping updated: %s -> %s\n", sensorID, newDeviceName)
	return nil
}

// 根据EID返回逻辑设备名
func LookupDeviceName(sensorID string) (deviceName string, ok bool) {
	mu1.RLock()
	defer mu1.RUnlock()
	deviceName, ok = SensorIDToDeviceName[sensorID]
	return
}

// 资源值映射
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
			eid = fmt.Sprint(v)
		}
		if eid == "" {
			continue
		}
		SensorIDToDeviceName[eid] = deviceName
	}
}
