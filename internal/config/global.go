package config

import "sync"

var (
	UpgradeTCPPort uint32 = 12345 // TCP端口

	WriteChan = make(chan []byte, 100) // 写通道

	EidStr = "238A0841D828" // 模块EID

	BrokerURL = "tcp://192.168.75.137:1883" //MQTT代理

	//配置文件目录
	DevicesYAML = "../cmd/res/devices/devices.yaml"
	ProfilesDir = "../cmd/res/profiles"
)

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

// -----------------------------------------------升级处理-------------------------------------------------------------------
type FrameFlags struct {
	Acked          bool // 是否收到响应
	NeedComplement bool // 是否需要补包
}

// 升级ACK
var (
	AckReceived int
	Mu3         sync.Mutex
)

// 设置ACK
func SetAck(received bool) {
	Mu3.Lock()
	defer Mu3.Unlock()
	if received {
		AckReceived = 1
	} else {
		AckReceived = 0
	}
}

// 读取ACK
func GetAck() int {
	Mu3.Lock()
	defer Mu3.Unlock()
	return AckReceived
}
