package frameparser

import (
	"encoding/binary"
	"fmt"

	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
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
	// SensorID
	buf = append(buf, sensorID[:]...)
	//head：DataLen(4b=0) | FragInd(1b=0)<<3 | PacketType(3b)
	head := byte(0<<4) | byte(0<<3) | byte(packetTypeControl&0x07)
	buf = append(buf, head)
	//CtrlType+RequestSetFlag：7b ctrlType<<1 | 1b flag
	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)
	// Timestamp(4字节)
	// 查询时 timestamp=0；设置时请传入需要下发的世纪秒
	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp)
	buf = append(buf, tsBytes...)
	//CRC16
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

	// 分配：6B SensorID + 1B head + 1B ctrl + 4B ts + 2B CRC
	buf := make([]byte, 0, 6+1+1+4+2)
	// SensorID
	buf = append(buf, sensorID[:]...)
	// head：DataLen(4b=0) | FragInd(1b=0)<<3 | PacketType(3b)
	head := byte(0<<4) | byte(0<<3) | byte(packetTypeControl&0x07)
	buf = append(buf, head)
	// CtrlType+RequestSetFlag：7b ctrlType<<1 | 1b flag
	ctrlByte := byte((ctrlTypeTimeParam&0x7F)<<1) | (requestSetFlag & 0x01)
	buf = append(buf, ctrlByte)
	// Timestamp(4字节)
	tsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tsBytes, timestamp) // 如果协议是小端
	buf = append(buf, tsBytes...)
	// CRC16
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	// 发送帧
	relay.SendFrame(eidStr, buf)

	return nil
}
