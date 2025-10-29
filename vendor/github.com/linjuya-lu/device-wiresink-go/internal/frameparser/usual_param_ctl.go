package frameparser

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

const (
	// CtrlType: 通用参数查询/设置 (7bit)，协议附录B 定义
	ctrlTypeGeneralParams = 0x03
	// 最大支持一次下发/查询的参数数量
	maxParams = 16
)

// 封装 7.2 节 传感器通用参数查询/设置报文
//
//	sensorID:        6 字节传感器 ID
//	requestSetFlag:  0 = 查询所有参数（此时 paramsMap 应传 nil 或 empty，DataLen=0xF 且无 ParameterList）
//	                 1 = 按 paramsOrder & paramsMap 中指定的参数组合 ParameterList
//	paramsOrder:     设 requestSetFlag=1 时，按此顺序列出要查询/设置的参数名
//	paramsMap:       map[参数名]→[]byte（对应参数的数据内容）
//
// 返回：完整帧字节切片
func BuildGeneralParamFrame(sensorID [6]byte, requestSetFlag byte, paramsOrder []string, paramsMap map[string][]byte) ([]byte, error) {
	// 确定 DataLen 和 ParameterList
	var dataLen byte
	var parameterList []byte
	if requestSetFlag == 0 {
		// 查询所有通用参数：DataLen=0b1111，不附带 ParameterList
		dataLen = 0x0F
	} else {
		m := len(paramsOrder)
		if m == 0 || m > maxParams {
			return nil, fmt.Errorf("参数个数必须 1~%d, got %d", maxParams, m)
		}
		dataLen = byte(m & 0x0F)
		// 构造 ParameterList: 每个参数名对应 head16(2B little-endian) + data
		buf := &bytes.Buffer{}
		for _, name := range paramsOrder {
			// 先拿到当前表中对应的 entry 副本
			entry, err := config.GetEntryCopy(name)
			if err != nil {
				return nil, err
			}
			// 再更新 entry.data 为调用者传来的值
			val, ok := paramsMap[name]
			if !ok {
				return nil, fmt.Errorf("缺少参数 %q 的值", name)
			}
			if len(val) != entry.Length {
				return nil, fmt.Errorf("参数 %q 长度错误: want %d, got %d", name, entry.Length, len(val))
			}
			entry.Data = make([]byte, entry.Length)
			copy(entry.Data, val)
			// 将 head16 写入
			le := make([]byte, 2)
			binary.LittleEndian.PutUint16(le, entry.Head16)
			buf.Write(le)
			// 将 data 写入
			buf.Write(entry.Data)
		}
		parameterList = buf.Bytes()
	}
	// 构建前导头：SensorID(6B) + head(1B)
	//    head = DataLen(4b)<<4 | FragInd(1b=0)<<3 | PacketType(3b)
	head := byte((dataLen&0x0F)<<4) | byte(packetTypeControl&0x07)
	// 构建 CtrlType+RequestSetFlag(1b)
	ctrlByte := byte((ctrlTypeGeneralParams&0x7F)<<1) | (requestSetFlag & 0x01)
	// 汇总所有字段
	buf := &bytes.Buffer{}
	buf.Write(sensorID[:])
	buf.WriteByte(head)
	buf.WriteByte(ctrlByte)
	if requestSetFlag == 1 {
		buf.Write(parameterList)
	}
	// 追加CRC16
	crc := CRC16(buf.Bytes())
	crcb := make([]byte, 2)
	binary.BigEndian.PutUint16(crcb, crc)
	buf.Write(crcb)
	return buf.Bytes(), nil
}

// BuildParameterQueryFrame 构造 “通用参数查询” 控制报文。
//
//	sensorID: 原始 6 字节传感器 ID。
//
// 返回值：完整报文字节，或出错。
func BuildParameterQueryFrame(sensorID [6]byte) ([]byte, error) {
	const (
		packetType     = 0x04 // 3 bit = 100b
		ctrlType       = 0x01 // 7 bit，协议中通用参数查询对应的 CtrlType
		dataLen        = 0x0F // 4 bit = 1111b，表示“请求所有通用参数”
		fragInd        = 0    // 1 bit
		requestSetFlag = 0    // 1 bit = 查询
	)
	// 拼EID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	// head 字节：DataLen(4) | FragInd(1) | PacketType(3)
	head := byte((dataLen&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	// ctrlByte 字节：CtrlType(7) | RequestSetFlag(1)
	ctrlByte := byte((ctrlType&0x7F)<<1) |
		byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// （不带 ParameterList，因为请求所有通用参数）
	// 追加 CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
