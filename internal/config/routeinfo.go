package config

// 节点拓扑
type NodeTopology struct {
	EID    string `json:"eid"`    // 节点地址
	Type   string `json:"type"`   // 节点类型：0=微功率，1=汇聚，2=低功耗，4=接入
	State  string `json:"state"`  // 在线状态  1=在线，0=离线
	Parent string `json:"parent"` // 父节点地址
}
