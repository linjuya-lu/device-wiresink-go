package config

import (
	"fmt"
	"log"
	"sync"
)

// EID映射
var (
	mu1                  sync.RWMutex
	SensorIDToDeviceName = map[string]string{}
)

// 添加EID
func AddMapping(sensorID, deviceName string) {
	mu1.Lock()
	defer mu1.Unlock()
	SensorIDToDeviceName[sensorID] = deviceName
	fmt.Printf("添加EID: %s -> %s\n", sensorID, deviceName)
}

func DeleteMapping(sensorID string) error {
	mu1.Lock()
	defer mu1.Unlock()
	if _, ok := SensorIDToDeviceName[sensorID]; !ok {
		return fmt.Errorf("无EID %s", sensorID)
	}
	delete(SensorIDToDeviceName, sensorID)
	fmt.Printf("删除EID: %s\n", sensorID)
	return nil
}

// 更新EID
func UpdateMapping(sensorID, newDeviceName string) error {
	mu1.Lock()
	defer mu1.Unlock()
	SensorIDToDeviceName[sensorID] = newDeviceName
	fmt.Printf("更新EID: %s -> %s\n", sensorID, newDeviceName)
	return nil
}

func LookupDeviceName(sensorID string) (deviceName string, ok bool) {
	mu1.RLock()
	defer mu1.RUnlock()
	log.Printf("[DEBUG] SensorIDToDeviceName: %#v", SensorIDToDeviceName)
	deviceName, ok = SensorIDToDeviceName[sensorID]
	return
}

// EID映射初始化
func UpdateSensorMapping() {
	mu1.Lock()
	defer mu1.Unlock()

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

// 删除映射
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
