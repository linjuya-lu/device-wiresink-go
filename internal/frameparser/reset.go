package frameparser

import (
	"encoding/binary"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

// 复位命令原始报文
func BuildResetRequest(sensorID [6]byte) ([]byte, error) {
	const (
		ctrlType = 0x06
		dataLen  = 0
	)

	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)

	head := byte((dataLen&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)

	ctrlByte := byte((ctrlType&0x7F)<<1) | byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)

	crc := config.CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
