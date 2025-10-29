package frameparser

import (
	"encoding/binary"
	"fmt"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/relay"
)

const packetTypeControl = 0x04
const ctrlTypeTimeParam = 0x04

// 时间参数查询/设置原始报文
//
//	requestSetFlag  0=查询，1=设置
//	timestamp       世纪秒
func BuildTimeParamFrame(sensorID [6]byte, requestSetFlag byte, timestamp uint32) ([]byte, error) {
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return nil, fmt.Errorf("无效参数 %d", requestSetFlag)
	}
	buf := make([]byte, 0, 6+1+1+4+2)
	// EID
	buf = append(buf, sensorID[:]...)
	//头
	head := byte(0<<4) | byte(0<<3) | byte(packetTypeControl&0x07)
	buf = append(buf, head)
	//控制类型
	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)
	// 时间戳
	// 查询时为0；设置为世纪秒
	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp)
	buf = append(buf, tsBytes...)
	//校验
	crc := CRC16(buf)
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
	// EID
	buf = append(buf, sensorID[:]...)
	// 头
	head := byte(0<<4) | byte(0<<3) | byte(packetTypeControl&0x07)
	buf = append(buf, head)
	// 控制类型
	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)
	// Timestamp(4字节)
	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp)
	buf = append(buf, tsBytes...)
	// 校验
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	relay.SendFrame(eidStr, buf)

	return nil
}
