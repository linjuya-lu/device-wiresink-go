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

// -----------------------------------------------升级所需全局变量-------------------------------------------------------------------
type FrameFlags struct {
	Acked          bool // 是否收到响应
	NeedComplement bool // 是否需要补包
}

type FrameTable struct {
	mu sync.RWMutex
	m  map[uint16]FrameFlags // key: Subpacket_No
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
