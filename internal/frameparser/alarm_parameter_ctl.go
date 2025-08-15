package frameparser

import "encoding/binary"

// 7.4 节 告警参数查询/设置报文
//
//	sensorID: 原始 6 字节传感器 ID。
//
// 返回值：含 CRC16 的完整报文字节，或出错。
func BuildAlarmParameterQueryFrame(sensorID [6]byte) ([]byte, error) {
	const (
		packetType     = 0x04 // 3bit = 100b
		ctrlAlarmQuery = 0x03 // 7bit，协议中“告警参数查询”对应的 CtrlType（示例值，按文档替换）
		dataLen        = 0x0F // 4bit = 1111b, 请求所有告警参数
		fragInd        = 0    // 1bit
		requestSetFlag = 0    // 1bit = 查询
	)
	// 拼SensorID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	//head：DataLen(4)|FragInd(1)|PacketType(3)
	head := byte((dataLen&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	//ctrlByte：CtrlType(7)|RequestSetFlag(1)
	ctrlByte := byte((ctrlAlarmQuery&0x7F)<<1) |
		byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// （不带 ParameterList，因为请求所有告警参数）
	// CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
