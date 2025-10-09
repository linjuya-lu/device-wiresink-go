package frameparser

import (
	"encoding/binary"
)

// 复位命令原始报文
func BuildResetRequest(sensorID [6]byte) ([]byte, error) {
	const (
		packetType     = 0x04
		ctrlType       = 0x06
		dataLen        = 0
		fragInd        = 0
		requestSetFlag = 0
	)
	// SensorID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	// head ：DataLen(4)|FragInd(1)|PacketType(3)
	head := byte((dataLen&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	// CtrlType(7bit) + RequestSetFlag(1bit) 共 1 字节
	ctrlByte := byte((ctrlType&0x7F)<<1) | byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
