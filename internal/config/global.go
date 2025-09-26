package config

import "sync"

// TCP端口
var UpgradeTCPPort uint32 = 12345 // TODO: 启动时从配置/环境变量覆盖
// var UpgradeTCPPort uint32 = 8080 // TODO: 启动时从配置/环境变量覆盖

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

type FrameTable struct {
	mu sync.RWMutex
	m  map[uint16]FrameFlags // key: 子包号
}

var Frames = NewFrameTable()

func NewFrameTable() *FrameTable {
	return &FrameTable{m: make(map[uint16]FrameFlags)}
}

func (t *FrameTable) Get(no uint16) (FrameFlags, bool) {
	t.mu.RLock()
	v, ok := t.m[no]
	t.mu.RUnlock()
	return v, ok
}
func (t *FrameTable) SetAcked(no uint16, v bool) {
	t.mu.Lock()
	f := t.m[no]
	f.Acked = v
	t.m[no] = f
	t.mu.Unlock()
}
func (t *FrameTable) SetNeedComplement(no uint16, v bool) {
	t.mu.Lock()
	f := t.m[no]
	f.NeedComplement = v
	t.m[no] = f
	t.mu.Unlock()
}
func (t *FrameTable) Delete(no uint16) {
	t.mu.Lock()
	delete(t.m, no)
	t.mu.Unlock()
}
func (t *FrameTable) Clear() {
	t.mu.Lock()
	t.m = make(map[uint16]FrameFlags)
	t.mu.Unlock()
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
