package config

import (
	"fmt"
	"sync"
)

var (
	ParamEidMap = map[ParamKey]string{}
	mu2         sync.RWMutex
)

// 添加EID
func AddParamEidMap(par ParamKey, deviceName string) {
	mu2.Lock()
	defer mu2.Unlock()
	ParamEidMap[par] = deviceName
	fmt.Printf("添加EID: %v -> %s\n", par, deviceName)
}

// 删除EID
func DeleteParamEidMap(par ParamKey) error {
	mu2.Lock()
	defer mu2.Unlock()
	if _, ok := ParamEidMap[par]; !ok {
		return fmt.Errorf("无EID %v", par)
	}
	delete(ParamEidMap, par)
	fmt.Printf("删除EID: %v\n", par)
	return nil
}

// 更新EID
func UpdateParamEidMap(par ParamKey, newDeviceName string) error {
	mu2.Lock()
	defer mu2.Unlock()
	if _, ok := ParamEidMap[par]; !ok {
		return fmt.Errorf("无EID %v", par)
	}
	ParamEidMap[par] = newDeviceName
	fmt.Printf("更新EID: %v -> %s\n", par, newDeviceName)
	return nil
}
