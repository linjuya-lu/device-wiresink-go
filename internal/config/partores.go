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
	ParamEidMap[k] = resourceName
	return nil
}

// 查询
func ParamEidGet(par ParamKey, deviceName string) (string, bool) {
	ParamEidMu.RLock()
	defer ParamEidMu.RUnlock()
	// 调试：打印整个映射表
	fmt.Println("===== ParamEidMap Dump BEGIN =====")
	for k, v := range ParamEidMap {
		// k 是 ParamKeyDevice 结构体，用 %+v 打出字段名更清晰
		fmt.Printf("  %+v -> %s\n", k, v)
	}
	fmt.Println("===== ParamEidMap Dump END =====")
	key := ParamKeyDevice{Key: par, DeviceName: deviceName}
	v, ok := ParamEidMap[key]
	// 调试：打印这次查询的结果
	if ok {
		fmt.Printf("[ParamEidGet] 命中: key=%+v -> %s\n", key, v)
	} else {
		fmt.Printf("[ParamEidGet] 未找到: key=%+v\n", key)
	}
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
