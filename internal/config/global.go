package config

import "sync"

// 全局写入通道，传入待发送的帧数据
var WriteChan = make(chan []byte, 100)

// topoList 存储最新一批解析出的 NodeTopology 列表
var (
	TopoList []NodeTopology
	topoMu   sync.RWMutex
)

// GetTopoList 返回当前缓存
func GetTopoList() []NodeTopology {
	topoMu.RLock()
	defer topoMu.RUnlock()
	cloned := make([]NodeTopology, len(TopoList))
	copy(cloned, TopoList)
	return cloned
}
