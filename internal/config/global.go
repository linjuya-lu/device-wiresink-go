package config

import (
	"sync"
	"time"
)

const (
	ServiceName    string = "device-wiresink"
	Version        string = "1.0.0"
	UpgradeTCPPort uint32 = 12345                                        // TCP端口
	EidStr                = "238A0841D828"                               // 模块EID
	GatewayEID            = "238A0841D828"                               // 汇聚网关EID
	BrokerURL             = "tcp://172.16.19.91:1883"                    //MQTT代理
	MqttTopicUp           = "edgex/service/request/device-wiresink/up"   // 上行
	MqttTopicDown         = "edgex/server/response/device-wiresink/down" // 下行
	DevicesYAML           = "../cmd/res/devices/devices.yaml"
	ProfilesDir           = "../cmd/res/profiles"
)

// 上传表
var (
	Mu        sync.RWMutex
	ValuesMap = make(map[string]map[string]any) //设备 → (资源 → 值)
)

// 解析表
var (
	paramMu  sync.RWMutex
	paramMap = map[ParamKey]ParamInfo{
		{0b000, 0b00000000001}: {parseFloat32},
		{0b000, 0b00000001000}: {parseTopo},
	}
)

var (
	WriteChan     = make(chan []byte, 100) // 写通道
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

// 升级ACK
var (
	AckReceived int
	Mu3         sync.Mutex
)
