package frameparser

import (
	"encoding/binary"
)

//	7.3 节 监测参数查询/设置报文
//

//	sensorID: 传感器 ID。
//
// 返回值：含 CRC16 的完整报文字节，或出错。
func BuildMonitoringDataQueryFrame(sensorID [6]byte) ([]byte, error) {
	const (
		packetType       = 0x04 // 3bit = 100b
		ctrlTypeMonitor  = 0x02 // 7bit  CtrlType
		dataLenAllParams = 0x0F // 4bit = 1111b, 表示请求所有可采集参数
		fragInd          = 0    // 1bit，未分片
		requestSetFlag   = 0    // 1bit，查询
	)
	// 拼 SensorID
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
