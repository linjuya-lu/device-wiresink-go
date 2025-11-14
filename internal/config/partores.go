package config

import (
	"fmt"
	"sync"
)

type ParamKeyDevice struct {
	Key        ParamKey
	DeviceName string
}

var (
	ParamEidMu  sync.RWMutex
	ParamEidMap = make(map[ParamKeyDevice]string)
)

// 新增
func ParamEidAdd(par ParamKey, deviceName, resourceName string) {
	ParamEidMu.Lock()
	ParamEidMap[ParamKeyDevice{Key: par, DeviceName: deviceName}] = resourceName
	ParamEidMu.Unlock()
}

// 删除
func ParamEidDelete(par ParamKey, deviceName string) error {
	ParamEidMu.Lock()
	defer ParamEidMu.Unlock()
	k := ParamKeyDevice{Key: par, DeviceName: deviceName}
	if _, ok := ParamEidMap[k]; !ok {
		return fmt.Errorf("无绑定: %v@%s", par, deviceName)
	}
	delete(ParamEidMap, k)
	return nil
}

// 更新
func ParamEidUpdate(par ParamKey, deviceName, resourceName string) error {
	ParamEidMu.Lock()
	defer ParamEidMu.Unlock()
	k := ParamKeyDevice{Key: par, DeviceName: deviceName}
	if _, ok := ParamEidMap[k]; !ok {
		return fmt.Errorf("无绑定: %v@%s", par, deviceName)
	}
	ParamEidMap[k] = resourceName
	return nil
}

// 查询
func ParamEidGet(par ParamKey, deviceName string) (string, bool) {
	ParamEidMu.RLock()
	defer ParamEidMu.RUnlock()
	v, ok := ParamEidMap[ParamKeyDevice{Key: par, DeviceName: deviceName}]
	return v, ok
}

// 快照导出
func ParamEidSnapshot() map[ParamKeyDevice]string {
	ParamEidMu.RLock()
	defer ParamEidMu.RUnlock()
	cp := make(map[ParamKeyDevice]string, len(ParamEidMap))
	for k, v := range ParamEidMap {
		cp[k] = v
	}
	return cp
}

// 清空全部绑定
func ParamEidClear() {
	ParamEidMu.Lock()
	ParamEidMap = make(map[ParamKeyDevice]string)
	ParamEidMu.Unlock()
}
