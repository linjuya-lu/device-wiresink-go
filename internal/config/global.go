package config

import (
	"sync"
	"time"
)

var (
	UpgradeTCPPort uint32 = 12345                                      // TCP端口
	WriteChan             = make(chan []byte, 100)                     // 写通道
	EidStr                = "238A0841D828"                             // 模块EID
	BrokerURL             = "tcp://172.16.19.91:1883"                  //MQTT代理
	MqttTopicUp           = "edgex/service/request/device-wiresink/up" // 上行
	MqttTopicDown         = "edgex/device/<svc>/uplink/event"          // 上行

	DevicesYAML = "../cmd/res/devices/devices.yaml"
	ProfilesDir = "../cmd/res/profiles"

	mu            sync.RWMutex
	LastDataTsMap = make(map[string]int64) // 设备数据时间戳
)

// 路由表
var (
	TopoList    []NodeTopology
	topoIndex   = map[string]int{} // EID -> index
	topoMu      sync.RWMutex
	topoLastAt  time.Time           // 最近合并时间
	topoIdleTTL = 600 * time.Second // 超过这个空闲视为新一轮
)

// 清空
func ClearTopo() (prev int) {
	topoMu.Lock()
	prev = len(TopoList)
	TopoList = TopoList[:0]
	topoIndex = make(map[string]int)
	topoLastAt = time.Time{} // 清掉时间戳
	topoMu.Unlock()
	return
}

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

// -----------------------------------------------心跳处理-------------------------------------------------------------------
// 更新时间戳
func UpdateLastDataTs(dev string, ts int64) {
	mu.Lock()
	LastDataTsMap[dev] = ts
	mu.Unlock()
}

// 获取时间戳
func GetLastDataTs(dev string) (int64, bool) {
	mu.RLock()
	ts, ok := LastDataTsMap[dev]
	mu.RUnlock()
	return ts, ok
}
