package config

import "sync"

// TCP端口
var UpgradeTCPPort uint32 = 12345

// 写通道
var WriteChan = make(chan []byte, 100)

// 模块EID
var EidStr = "238A0841D828"

// 路由表
var (
	TopoList []NodeTopology
	topoMu   sync.RWMutex
)

// 获取路由表
func GetTopoList() []NodeTopology {
	topoMu.RLock()
	defer topoMu.RUnlock()
	cloned := make([]NodeTopology, len(TopoList))
	copy(cloned, TopoList)
	return cloned
}

// -----------------------------------------------升级变量-------------------------------------------------------------------
type FrameFlags struct {
	Acked          bool // 是否收到响应
	NeedComplement bool // 是否需要补包
}

// 定义全局变量，默认 0 表示未收到 ACK
var AckReceived int
var Mu3 sync.Mutex // 防止并发读写问题

// 设置 ACK 状态
func SetAck(received bool) {
	Mu3.Lock()
	defer Mu3.Unlock()
	if received {
		AckReceived = 1
	} else {
		AckReceived = 0
	}
}

// 读取 ACK 状态
func GetAck() int {
	Mu3.Lock()
	defer Mu3.Unlock()
	return AckReceived
}
