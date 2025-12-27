package config

// 升级处理
type FrameFlags struct {
	Acked          bool
	NeedComplement bool
}

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
