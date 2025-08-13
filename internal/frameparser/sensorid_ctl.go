package frameparser

// 封装 7.6 节 传感器ID查询/设置报文

import (
	"encoding/binary"
	"fmt"
)

// 控制报文类型：传感器 ID 查询/设置（7bit），按协议附录B 定义
const ctrlTypeSensorID = 0x05 // 假设值为 7，如有具体值请替换

// BuildSensorIDFrame 构造 “传感器ID 查询/设置” 控制报文。
// sensorID: 原始 6 字节传感器 ID。
// requestSetFlag: 0=查询；1=设置。
// newID: 当 requestSetFlag=1 时，填入新的 6 字节 ID；否则可传空零值 [6]byte{}。
func BuildSensorIDFrame(sensorID [6]byte, requestSetFlag byte, newID [6]byte) ([]byte, error) {
	// 校验标志位
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return nil, fmt.Errorf("invalid requestSetFlag %d, must be 0 or 1", requestSetFlag)
	}
	// 头部缓存：6B SensorID + 1B head + 1B CtrlType+Flag
	buf := make([]byte, 0, 6+1+1+6+2)
	// SensorID
	buf = append(buf, sensorID[:]...)
	// head = DataLen(4b=0) | FragInd(1b=0)<<3 | PacketType(3b)
	head := byte(0<<4) | byte(0<<3) | byte(packetTypeControl&0x07)
	buf = append(buf, head)
	// Control 字段 = CtrlType(7b)<<1 | RequestSetFlag(1b)
	ctrlByte := byte((ctrlTypeSensorID & 0x7F) << 1)
	if requestSetFlag == 1 {
		ctrlByte |= 0x01
	}
	buf = append(buf, ctrlByte)
	// 报文内容：NewSensorID (6 字节)
	buf = append(buf, newID[:]...)
	// 校验位：CRC16 前面所有字节，大端序追加 2 字节
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
