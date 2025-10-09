package frameparser

import (
	"encoding/binary"
)

// 工况查询原始报文
func BuildMonitoringDataQueryFrame(sensorID [6]byte) ([]byte, error) {
	const (
		packetType       = 0x04 // 3bit = 100b
		ctrlTypeMonitor  = 0x02 // 7bit  CtrlType
		dataLenAllParams = 0x0F // 4bit = 1111b, 表示请求所有可采集参数
		fragInd          = 0    // 1bit，未分片
		requestSetFlag   = 0    // 1bit，查询
	)
	//SensorID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	//head：DataLen(4) | FragInd(1) | PacketType(3)
	head := byte((dataLenAllParams&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	//ctrlByte：CtrlType(7) | RequestSetFlag(1)
	ctrlByte := byte((ctrlTypeMonitor&0x7F)<<1) |
		byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	//CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
