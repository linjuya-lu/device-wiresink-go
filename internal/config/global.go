package config

import "sync"

// 写入通道
var WriteChan = make(chan []byte, 100)

// 模块EID
var EidStr = "238A0841D828"

// 路由表
var (
	TopoList []NodeTopology
	topoMu   sync.RWMutex
)

// 返回路由表
func GetTopoList() []NodeTopology {
	topoMu.RLock()
	defer topoMu.RUnlock()
	cloned := make([]NodeTopology, len(TopoList))
	copy(cloned, TopoList)
	return cloned
}
