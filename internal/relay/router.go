package relay

import (
	"fmt"
	"strings"
	"sync"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

// SendTopoQuery 向全局通道投递一个 AT+TOP 拓扑查询命令。
//
//	startIndex：起始序号（从 0 开始）
//	numOfQuery：本次要查询的节点数量（一次最多 10 个）
//
// 输出命令格式：\rAT+TOP=<startIndex>,<numOfQuery>?\r\n
func SendTopoQuery(startIndex, numOfQuery int) {
	body := fmt.Sprintf("AT+TOP=%d,%d?", startIndex, numOfQuery)
	cmd := "\r" + body + "\r\n"
	fmt.Printf(">> Sending Topology Query: %s\n", body)
	config.WriteChan <- []byte(cmd)
}

// topoList 存储最新一批解析出的 NodeTopology 列表
var (
	TopoList []config.NodeTopology
	topoMu   sync.RWMutex
)

// GetTopoList 返回当前缓存
func GetTopoList() []config.NodeTopology {
	topoMu.RLock()
	defer topoMu.RUnlock()
	cloned := make([]config.NodeTopology, len(TopoList))
	copy(cloned, TopoList)
	return cloned
}

// parseBuffer 把 "E1,t1,s1,p1,E2,t2,s2,p2,..." 拆成 []NodeTopology
func parseBuffer(buf string) ([]config.NodeTopology, error) {
	buf = strings.Trim(buf, ",")
	fields := strings.Split(buf, ",")
	if len(fields)%4 != 0 {
		return nil, fmt.Errorf("字段数 %d 不是 4 的倍数", len(fields))
	}
	count := len(fields) / 4
	list := make([]config.NodeTopology, 0, count)
	for i := 0; i < count; i++ {
		j := i * 4
		list = append(list, config.NodeTopology{
			EID:    fields[j],
			Type:   fields[j+1],
			State:  fields[j+2],
			Parent: fields[j+3],
		})
	}
	return list, nil
}
