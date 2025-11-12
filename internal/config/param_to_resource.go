package config

import (
	"fmt"
	"sync"
)

// ---- 键：ParamKey + DeviceName ----

type ParamKeyDevice struct {
	Key        ParamKey
	DeviceName string
}

// 全局变量：并发安全需要配合锁使用
var (
	ParamEidMu  sync.RWMutex
	ParamEidMap = make(map[ParamKeyDevice]string) // value = ResourceName
)

// 新增/覆盖：绑定 (ParamKey, deviceName) -> resourceName
func ParamEidAdd(par ParamKey, deviceName, resourceName string) {
	ParamEidMu.Lock()
	ParamEidMap[ParamKeyDevice{Key: par, DeviceName: deviceName}] = resourceName
	ParamEidMu.Unlock()
}

// 删除：按 (ParamKey, deviceName)
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

// 更新：按 (ParamKey, deviceName) 改 resourceName
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

// 查询：按 (ParamKey, deviceName) 取 resourceName
func ParamEidGet(par ParamKey, deviceName string) (string, bool) {
	ParamEidMu.RLock()
	defer ParamEidMu.RUnlock()
	v, ok := ParamEidMap[ParamKeyDevice{Key: par, DeviceName: deviceName}]
	return v, ok
}

// 快照导出（深拷贝一份，避免外部改内部 map）
func ParamEidSnapshot() map[ParamKeyDevice]string {
	ParamEidMu.RLock()
	defer ParamEidMu.RUnlock()
	cp := make(map[ParamKeyDevice]string, len(ParamEidMap))
	for k, v := range ParamEidMap {
		cp[k] = v
	}
	return cp
}

// 可选：清空全部绑定
func ParamEidClear() {
	ParamEidMu.Lock()
	ParamEidMap = make(map[ParamKeyDevice]string)
	ParamEidMu.Unlock()
}
