package config

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"
)

type ParamKey struct {
	FeatureBits byte   // 参量特征
	CodeBits    uint16 // 类型编码
}

type ParamInfo struct {
	Parse func([]byte) (any, error)
}

// 查询
func GetParamInfo(key ParamKey) (ParamInfo, bool) {
	paramMu.RLock()
	defer paramMu.RUnlock()

	info, ok := paramMap[key]
	return info, ok
}

// 整表替换
func ReplaceParamMap(newMap map[ParamKey]ParamInfo) {
	paramMu.Lock()
	defer paramMu.Unlock()
	paramMap = newMap
}

func LookupParamInfo(paramType uint16) (ParamInfo, bool) {
	feature := byte((paramType >> 11) & 0x07)
	code := paramType & 0x7FF
	fmt.Printf(
		"TypeCode=0x%04X → Feature=%03b (0x%X), Code=%011b (0x%X)\n",
		paramType, feature, feature, code, code,
	)
	key := ParamKey{
		FeatureBits: feature,
		CodeBits:    code,
	}
	paramMu.RLock()
	info, ok := paramMap[key]
	paramMu.RUnlock()

	return info, ok
}

// ===================== 通用解析函数 =====================

func parseFloat32(data []byte) (any, error) {
	if len(data) != 4 {
		return nil, fmt.Errorf("期望4字节，实际%d", len(data))
	}
	bits := binary.LittleEndian.Uint32(data)
	val := math.Float32frombits(bits)
	return val, nil
}

func parseUint8(data []byte) (any, error) {
	if len(data) != 1 {
		return nil, fmt.Errorf("期望1字节，实际%d", len(data))
	}
	return data[0], nil
}

func parseUint16(data []byte) (any, error) {
	if len(data) != 2 {
		return nil, fmt.Errorf("期望2字节，实际%d", len(data))
	}
	return binary.LittleEndian.Uint16(data), nil
}

func parseUint32(data []byte) (any, error) {
	if len(data) != 4 {
		return nil, fmt.Errorf("期望4字节，实际%d", len(data))
	}
	return binary.LittleEndian.Uint32(data), nil
}

func parsefloat32Array(data []byte) (any, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("波形数据长度非4的倍数: %d", len(data))
	}
	n := len(data) / 4
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}

func parseUint16Array(data []byte) (any, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("uint16 数组数据长度非2的倍数: %d", len(data))
	}
	n := len(data) / 2
	values := make([]uint16, n)
	for i := 0; i < n; i++ {
		values[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return values, nil
}

func parseInt16(data []byte) (any, error) {
	if len(data) != 2 {
		return nil, fmt.Errorf("期望2字节，实际%d", len(data))
	}
	u := binary.LittleEndian.Uint16(data)
	val := int16(u)
	return val, nil
}

// 拓扑解析并合并
func parseTopo(data []byte) (any, error) {
	n := len(data)

	// 判断是否为节点
	looksLikeNode := func(i int) bool {
		if i+16 > n {
			return false
		}
		return data[i+6] == 0x2C && // ','
			data[i+8] == 0x2C &&
			data[i+10] == 0x2C
	}
	// 字节转字符串
	toHex12 := func(b []byte) string {
		const hexdigits = "0123456789ABCDEF"
		dst := make([]byte, 12)
		for j := 0; j < 6; j++ {
			v := b[j]
			dst[2*j] = hexdigits[v>>4]
			dst[2*j+1] = hexdigits[v&0x0F]
		}
		return string(dst)
	}
	// 找起点
	i := 0
	for i < n && !looksLikeNode(i) {
		i++
	}
	if i >= n {
		return nil, fmt.Errorf("未找到节点起点，数拓扑不符合约定")
	}
	// 解析所有节点
	var entries []NodeTopology
	for i < n {
		if !looksLikeNode(i) {
			break
		}
		// EID
		eid := data[i : i+6]
		i += 6
		// ',' state ','
		if i >= n || data[i] != 0x2C {
			return nil, fmt.Errorf("节点缺少逗号分隔(1)")
			//			TODO:EID STATE关系映射
		}
		i++
		if i >= n {
			return nil, fmt.Errorf("节点缺少state字节")
		}
		stateByte := data[i]
		i++
		if i >= n || data[i] != 0x2C {
			return nil, fmt.Errorf("节点缺少逗号分隔(2)")
		}
		i++
		// type ','
		if i >= n {
			return nil, fmt.Errorf("节点缺少type字节")
		}
		typeByte := data[i]
		i++
		if i >= n || data[i] != 0x2C {
			return nil, fmt.Errorf("节点缺少逗号分隔(3)")
		}
		i++
		// parent(6)
		if i+6 > n {
			return nil, fmt.Errorf("节点缺少父EID字节")
		}
		parent := data[i : i+6]
		i += 6

		entries = append(entries, NodeTopology{
			EID:    toHex12(eid),
			State:  strconv.Itoa(int(stateByte)),
			Type:   strconv.Itoa(int(typeByte)),
			Parent: toHex12(parent),
		})
		// 本帧内节点分隔符：'$'
		if i < n && data[i] == 0x24 { // '$'
			i++
			continue
		}
		// 紧跟下一个节点
		if i < n && looksLikeNode(i) {
			continue
		}
		break
	}
	// 合并
	now := time.Now()
	topoMu.Lock()
	// 超时：开始新一轮，自动清空
	if topoIdleTTL > 0 && !topoLastAt.IsZero() && now.Sub(topoLastAt) > topoIdleTTL {
		TopoList = TopoList[:0]
		topoIndex = make(map[string]int)
	}
	topoLastAt = now
	for _, e := range entries {
		if idx, ok := topoIndex[e.EID]; ok {
			TopoList[idx] = e
		} else {
			topoIndex[e.EID] = len(TopoList)
			TopoList = append(TopoList, e)
		}
	}
	snapshot := append([]NodeTopology(nil), TopoList...)
	topoMu.Unlock()

	return snapshot, nil
}
