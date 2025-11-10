package config

import "sync"

type EidInfo struct {
	eid string
}

var ParamEidMap = map[ParamKey]EidInfo{
	{0b000, 0b00000000001}: {"238A0830CC20"},
}
var mu2 sync.RWMutex

// EID -> 资源
func SetParamEidMap(par ParamKey, eid EidInfo) {
	mu2.Lock()
	defer mu2.Unlock()
	if _, ok := ParamEidMap[par]; !ok {
		ParamEidMap[par] = EidInfo{}
	}
	ParamEidMap[par] = eid
}
