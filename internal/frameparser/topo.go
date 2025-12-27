package frameparser

import (
	"encoding/binary"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

const (
	packetType     = 0x04
	fragInd        = 0 // 未分片
	requestSetFlag = 0 // 查询
)

// 传感器监测数据查询控制报文。
func BuildMonitoringDataQueryFrame1(sensorID [6]byte) ([]byte, error) {
	const (
		ctrlType   = 0x02
		dataParams = 0x0F
	)
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	head := byte((dataParams&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	ctrlByte := byte((dataParams&0x7F)<<1) |
		byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	crc := config.CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}

// 拓扑查询
func BuildGeneralParamQueryFrame(sensorID [6]byte, paramType14 uint16) ([]byte, error) {
	const (
		ctrlTypeMonitor = 0x02 // 7bit
		dataCount       = 0x01 // 4bit
	)
	buf := make([]byte, 0, 6+1+1+2+2)
	buf = append(buf, sensorID[:]...)
	head := byte((dataCount&0x0F)<<4) | byte((fragInd&0x01)<<3) | byte(packetType&0x07)
	buf = append(buf, head)
	ctrlByte := byte((ctrlTypeMonitor&0x7F)<<1) | byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// 参数列表
	buf = append(buf, 0x20, 0x00)
	crc := config.CRC16(buf)
	var crcB [2]byte
	binary.BigEndian.PutUint16(crcB[:], crc)
	buf = append(buf, crcB[:]...)
	return buf, nil
}
