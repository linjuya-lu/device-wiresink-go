package frameparser

// 封装 7.3 节 监测参数查询/设置报文

import (
	"encoding/binary"
)

// BuildMonitoringDataQueryFrame 构造 “传感器监测数据查询” 控制报文。
//
//	sensorID: 原始 6 字节传感器 ID。
//
// 返回值：含 CRC16 的完整报文字节 slice，或出错。
func BuildMonitoringDataQueryFrame1(sensorID [6]byte) ([]byte, error) {
	const (
		packetType       = 0x04 // 3bit = 100b
		ctrlTypeMonitor  = 0x02 // 7bit，协议中“请求监测数据”对应的 CtrlType
		dataLenAllParams = 0x0F // 4bit = 1111b, 表示请求所有可采集参数
		fragInd          = 0    // 1bit，未分片
		requestSetFlag   = 0    // 1bit，查询
	)
	// 拼前 6 字节 SensorID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	// 拼 head：DataLen(4) | FragInd(1) | PacketType(3)
	head := byte((dataLenAllParams&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	// 拼 ctrlByte：CtrlType(7) | RequestSetFlag(1)
	ctrlByte := byte((ctrlTypeMonitor&0x7F)<<1) |
		byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// （不带 TypeList，因为请求所有参数）
	// 计算 CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}

// BuildGeneralParamQueryFrame 构造“传感器通用参数查询/设置”里的“查询 1 个参数”的控制报文。
// - sensorID: 6 字节传感器ID
// - paramType14: 参数类型（14bit，见附录D的编码），高于 14bit 的位会被丢弃
// 返回：完整报文（含 CRC16）
// func BuildGeneralParamQueryFrame(sensorID [6]byte, paramType14 uint16) ([]byte, error) {
// 	const (
// 		packetType      = 0x04 // 3bit = 100b, 控制报文
// 		ctrlTypeMonitor = 0x02 // 7bit，上面代码一致
// 		dataCount       = 0x01 // 4bit，参量个数=1
// 		fragInd         = 0    // 1bit，未分片
// 		requestSetFlag  = 0    // 1bit，查询
// 	)

// 	buf := make([]byte, 0, 6+1+1+2+2+4+2)
// 	buf = append(buf, sensorID[:]...)

// 	// head：DataLen(4) | FragInd(1) | PacketType(3)
// 	head := byte((dataCount&0x0F)<<4) |
// 		byte((fragInd&0x01)<<3) |
// 		byte(packetType&0x07)
// 	buf = append(buf, head)

// 	// ctrlByte：CtrlType(7) | RequestSetFlag(1)
// 	ctrlByte := byte((ctrlTypeMonitor&0x7F)<<1) |
// 		byte(requestSetFlag&0x01)
// 	buf = append(buf, ctrlByte)

// 	// 参数列表：这里只装 1 个参数（查询时值默认 0）
// 	paramEntry := EncodeParamEntry(paramType14, 0 /*lenFlag=0*/, nil /*自动填4字节0*/)
// 	buf = append(buf, paramEntry...)

// 	// CRC16（与你现有 CRC16 实现保持一致；示例为 BigEndian 放入）
// 	crc := CRC16(buf)
// 	var crcB [2]byte
// 	binary.BigEndian.PutUint16(crcB[:], crc)
// 	buf = append(buf, crcB[:]...)

// 	return buf, nil
// }

// BuildGeneralParamQueryFrame 构造“查询 1 个参数”控制报文（参数列表固定为 0x20 0x00）。
func BuildGeneralParamQueryFrame(sensorID [6]byte, paramType14 uint16) ([]byte, error) {
	const (
		packetType      = 0x04 // 3bit = 100b, 控制报文
		ctrlTypeMonitor = 0x02 // 7bit
		dataCount       = 0x01 // 4bit，参量个数=1
		fragInd         = 0    // 1bit，未分片
		requestSetFlag  = 0    // 1bit，查询
	)

	// 预分配：6(SensorID)+1(head)+1(ctrl)+2(参数列表固定)+2(CRC)
	buf := make([]byte, 0, 6+1+1+2+2)
	buf = append(buf, sensorID[:]...)

	// head：DataLen(4) | FragInd(1) | PacketType(3)
	head := byte((dataCount&0x0F)<<4) | byte((fragInd&0x01)<<3) | byte(packetType&0x07)
	buf = append(buf, head)

	// ctrlByte：CtrlType(7) | RequestSetFlag(1)
	ctrlByte := byte((ctrlTypeMonitor&0x7F)<<1) | byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)

	// 参数列表：固定为 0x20 0x00（忽略 paramType14）
	buf = append(buf, 0x20, 0x00)

	// CRC16（与你的实现保持一致；示例为 BigEndian 放入）
	crc := CRC16(buf)
	var crcB [2]byte
	binary.BigEndian.PutUint16(crcB[:], crc)
	buf = append(buf, crcB[:]...)

	return buf, nil
}

// EncodeParamEntry 将单个参数项封装成 “SensorType(14b) + LengthFlag(2b) + [length] + Data”。
// 约定：
// - lenFlag=0：不带长度字段，Data 固定 4 字节；若 data==nil 或长度!=4，则自动使用 4 个 0x00。
// - lenFlag=1：后随 1 字节长度；
// - lenFlag=2：后随 2 字节长度（LittleEndian，与你的解析保持一致）；
// - lenFlag=3：后随 3 字节长度（按你的解析顺序写成高-中-低）。
func EncodeParamEntry(paramType14 uint16, lenFlag uint8, data []byte) []byte {
	paramType14 &= 0x3FFF // 只保留 14 位

	// header 16bit：paramType(14) | lenFlag(2)
	head16 := (paramType14 << 2) | uint16(lenFlag&0x03)

	out := make([]byte, 0, 2+3+len(data))
	// 你的解析用的是 binary.LittleEndian.Uint16 读 header，所以这里也按 LittleEndian 写
	var h [2]byte
	binary.LittleEndian.PutUint16(h[:], head16)
	out = append(out, h[:]...)

	switch lenFlag & 0x03 {
	case 0:
		// 固定 4 字节数据；查询时“参数值默认 0”
		if len(data) != 4 {
			out = append(out, 0x00, 0x00, 0x00, 0x00)
		} else {
			out = append(out, data[:]...)
		}
	case 1:
		// 1 字节长度
		l := byte(len(data) & 0xFF)
		out = append(out, l)
		out = append(out, data...)
	case 2:
		// 2 字节长度（LittleEndian，与解析一致）
		var lb [2]byte
		binary.LittleEndian.PutUint16(lb[:], uint16(len(data)))
		out = append(out, lb[:]...)
		out = append(out, data...)
	case 3:
		// 3 字节长度（按你的解析：先高字节，再中，再低）
		ln := len(data)
		out = append(out, byte((ln>>16)&0xFF), byte((ln>>8)&0xFF), byte(ln&0xFF))
		out = append(out, data...)
	}
	return out
}
