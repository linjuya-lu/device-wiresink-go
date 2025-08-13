package frameparser

// 封装 7.7 节 传感器复位设置报文
import (
	"encoding/binary"
)

// BuildResetRequest 构造“复位”控制报文。
// sensorID: 6 字节传感器 ID；
// 返回值：整帧字节切片（含 CRC），或出错。
func BuildResetRequest(sensorID [6]byte) ([]byte, error) {
	const (
		// PacketType（3bit） = 100b = 4
		packetType = 0x04
		// CtrlType（7bit） = 6
		ctrlType = 0x06
		// 以下字段取默认
		dataLen        = 0 // 4bit
		fragInd        = 0 // 1bit
		requestSetFlag = 0 // 1bit
	)
	// 拼 6 字节 SensorID
	buf := make([]byte, 0, 6+1+1+2)
	buf = append(buf, sensorID[:]...)
	// 拼 head 一字节：DataLen(4)|FragInd(1)|PacketType(3)
	head := byte((dataLen&0x0F)<<4) |
		byte((fragInd&0x01)<<3) |
		byte(packetType&0x07)
	buf = append(buf, head)
	// 拼 CtrlType(7bit) + RequestSetFlag(1bit) 共 1 字节
	ctrlByte := byte((ctrlType&0x7F)<<1) | byte(requestSetFlag&0x01)
	buf = append(buf, ctrlByte)
	// 计算 CRC16（针对前面所有字节），并以大端序追加 2 字节
	crc := CRC16(buf)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf, nil
}
