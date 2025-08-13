package frameparser

// 封装 7.5 节 传感器时间参数查询/设置报文

import (
	"encoding/binary"
	"fmt"

	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

// packetTypeControl 3bit = 100b = 4
const packetTypeControl = 0x04

// ctrlTypeTimeParam 7bit = 4 （协议“传感器时间查询/设置”类型码）
const ctrlTypeTimeParam = 0x04

// BuildTimeParamFrame 构造“时间参数查询/设置”控制报文：
//
//	sensorID        [6]byte — 传感器 ID
//	requestSetFlag  byte   — 0=查询，1=设置
//	timestamp       uint32 — 世纪秒（设置时有效；查询时请传 0）
//
// 返回：完整的二进制帧（已附加 CRC16），或错误。
func BuildTimeParamFrame(sensorID [6]byte, requestSetFlag byte, timestamp uint32) ([]byte, error) {
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return nil, fmt.Errorf("invalid requestSetFlag %d, must be 0 or 1", requestSetFlag)
	}
	// 预分配：6B SensorID + 1B head + 1B ctrl + 4B ts + 2B CRC
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
	//CRC16 校验位（大端序）
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}

//----------------------------------------------------------exmaple----------------------------------------------------------
// 查询当前时间
// reqFrame, _ := ctrlframe.BuildTimeParamFrame(sensorID, 0, 0)
// serialPort.Write(reqFrame)
// 或者设置时间为 2025-06-25 12:00:00 UTC
// ts := uint32(time.Date(2025,6,25,12,0,0,0,time.UTC).Unix())
// reqFrame, _ = ctrlframe.BuildTimeParamFrame(sensorID, 1, ts)
// serialPort.Write(reqFrame)

func RestCommandBuildFrame(eidStr string, sensorID [6]byte, requestSetFlag byte, timestamp uint32) error {
	if requestSetFlag != 0 && requestSetFlag != 1 {
		return fmt.Errorf("invalid requestSetFlag %d, must be 0 or 1", requestSetFlag)
	}

	// 预分配：6B SensorID + 1B head + 1B ctrl + 4B ts + 2B CRC
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

	// CRC16 校验位（大端序）
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)

	// 发送帧（这里调用 serial.SendFrame）
	relay.SendFrame(eidStr, buf)

	return nil
}
