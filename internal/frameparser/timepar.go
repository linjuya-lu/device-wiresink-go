package frameparser

import (
	"encoding/binary"
	"fmt"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/relay"
)

const ctrlTypeTimeParam = 0x04

// 时间参数查询/设置
//
//	requestSetFlag  0=查询，1=设置
func BuildTimeParamFrame(sensorID [6]byte, requestSetFlag byte, timestamp uint32) ([]byte, error) {
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return nil, fmt.Errorf("无效参数 %d", requestSetFlag)
	}
	buf := make([]byte, 0, 6+1+1+4+2)
	buf = append(buf, sensorID[:]...)

	head := byte(0<<4) | byte(0<<3) | byte(packetType&0x07)
	buf = append(buf, head)

	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)

	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp)
	buf = append(buf, tsBytes...)

	crc := config.CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}

func RestCommandBuildFrame(eidStr string, sensorID [6]byte, requestSetFlag byte, timestamp uint32) error {
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return fmt.Errorf("invalid requestSetFlag %d, must be 0 or 1", requestSetFlag)
	}

	buf := make([]byte, 0, 6+1+1+4+2)

	buf = append(buf, sensorID[:]...)

	head := byte(0<<4) | byte(0<<3) | byte(packetType&0x07)
	buf = append(buf, head)

	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)

	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp)
	buf = append(buf, tsBytes...)

	crc := config.CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	relay.SendFrame(eidStr, buf)

	return nil
}
