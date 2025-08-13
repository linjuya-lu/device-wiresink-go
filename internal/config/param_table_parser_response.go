package config

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"time"
)

// Frame 代表“通用传感器报文”
type Frame struct {
	SensorID   string // 传感器 ID，6 字节
	DataLen    byte   // 参量个数，使用下位 4 位即可，或者直接用 uint32 存放 m
	FragInd    byte   // 分片指示，true=已分片, false=未分片
	PacketType byte   // 报文类型，3 字节，例：0x00,0x01,0x00 表示类型 100
	Payload    []byte // 报文内容
	Check      uint16 // 校验位，2 字节 CRC
}

// Bytes 把 Frame 转成 []byte
func (f *Frame) Bytes() []byte {
	buf := make([]byte, 0, 6+1+1+1+len(f.Payload)+2)
	buf = append(buf, f.SensorID[:]...)
	buf = append(buf, f.DataLen)
	// flags：高4位 DataLen，下一位 FragInd，低3位 PacketType
	flags := (f.DataLen << 4) | byte(f.FragInd<<3) | byte(f.PacketType)
	buf = append(buf, flags)
	buf = append(buf, f.Payload...)
	// CRC16 要先转成大/小端两字节，比如小端：
	crc := []byte{byte(f.Check), byte(f.Check >> 8)}
	buf = append(buf, crc...)
	return buf
}

type ResponseKey struct {
	// 控制报文类型：只用低 7 位
	CtrlType uint8
	// 参数配置类型标识：1 bit，0/1
	RequestSetFlag bool
}

type ResponseHandle struct {
	Parse func(data []byte, frameCtl Frame) error
}

var ResponseMap = map[ResponseKey]ResponseHandle{
	{CtrlType: 0x02, RequestSetFlag: false}: {common_para_response},
	{CtrlType: 0x02, RequestSetFlag: true}:  {common_para_response},
	{CtrlType: 0x04, RequestSetFlag: true}:  {timestamp_response},
	{CtrlType: 0x03, RequestSetFlag: true}:  {timestamp_response},
	{CtrlType: 0x06, RequestSetFlag: false}: {reset_response},
	{CtrlType: 0x06, RequestSetFlag: true}:  {reset_response},
	{CtrlType: 0x04, RequestSetFlag: false}: {timestamp_response},
	{CtrlType: 0x07, RequestSetFlag: false}: {resetCommands},
	{CtrlType: 0x07, RequestSetFlag: false}: {resetCommands},
}

func LookupResponseHandle(head uint8) (ResponseHandle, bool) {
	ctrlType := head >> 1
	requestSet := (head & 0x1) == 1
	key := ResponseKey{ctrlType, requestSet}
	handle, ok := ResponseMap[key]
	return handle, ok
}

// ===================== 通用解析函数 =====================
var Resources1 = make(map[string]interface{})
var ResourcesFlag bool = false

// 通用参数查询/设置
func common_para_response(data []byte, frameCtl Frame) error {
	idx := 0
	parsed := 0
	Resources1 = make(map[string]interface{})

	ResourcesFlag = false
	for parsed < int(frameCtl.DataLen) {
		// 参数头2字节
		if idx+2 > len(data)-2 {
			log.Printf("参数头越界 SensorID=%s，跳过本帧", frameCtl.SensorID)
			break
		}
		head16 := binary.LittleEndian.Uint16(data[idx : idx+2])
		idx += 2
		paramType := head16 >> 2       // 14bit类型码
		lenFlag := uint8(head16 & 0x3) // 2bit长度指示

		// 计算真实数据长度
		var dataLen uint32
		switch lenFlag {
		case 0:
			dataLen = 4 // 默认4字节
		case 1:
			dataLen = uint32(data[idx])
			idx++
		case 2:
			dataLen = uint32(binary.BigEndian.Uint16(data[idx : idx+2]))
			idx += 2
		case 3:
			dataLen = uint32(data[idx])<<16 | uint32(data[idx+1])<<8 | uint32(data[idx+2])
			idx += 3
		}

		// 数据越界校验
		if idx+int(dataLen) > len(data)-2 {
			log.Printf("参数数据越界 SensorID=%s，跳过本帧", frameCtl.SensorID)
			break
		}

		// 提取原始值字节
		valBytes := data[idx : idx+int(dataLen)]
		idx += int(dataLen)

		deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
		if !hasDevice {
			log.Printf("未知 SensorID=%s，跳过本帧", frameCtl.SensorID)
			continue
		}
		// 解析数据
		if info, ok := LookupParamInfo(paramType); ok {
			val, err := info.Parse(valBytes)
			if err != nil {
				log.Printf("❌ 参数 %s.%s 解析失败: %v", deviceName, info.Name, err)
			} else {
				// 写入运行时值表
				SetDeviceValue(deviceName, info.Name, val)
				Resources1[info.Name] = val

				log.Printf("✅ 写入值 %s.%s = %v %s", deviceName, info.Name, val, info.Unit)
			}
		} else {
			log.Printf("未找到参数类型信息 type=0x%X", paramType)
		}

		parsed++
	}
	ResourcesFlag = true
	return nil
}

// 时间参数查询/设置
func timestamp_response(data []byte, frameCtl Frame) error {

	// secs := binary.LittleEndian.Uint32(data)
	// 转换为本地时区时间
	// t := time.Unix(int64(secs), 0)
	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知 SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	timestamp_ctl := "timestamp"
	log.Printf("data[0] = 0x%02X", data[0]) // %02X 表示两位十六进制，大写
	secs := binary.LittleEndian.Uint32(data[0:4])
	t := time.Unix(int64(secs), 0) // 秒 -> 时间
	log.Printf("世纪秒=%d 时间=%s", secs, t.Format("2006-01-02 15:04:05"))

	strVal := strconv.Itoa(int(data[0]))
	SetDeviceValue(deviceName, timestamp_ctl, strVal)
	return nil
}

// 复位设置
func reset_response(data []byte, frameCtl Frame) error {

	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知 SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	reset_ctl := "reset_ctl"
	strVal := strconv.Itoa(int(data[0]))
	SetDeviceValue(deviceName, reset_ctl, strVal)
	return nil
}

func resetCommands(data []byte, frameCtl Frame) error {

	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知 SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	eidValue, ok := GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		return err
	}
	eidStr, ok := eidValue.(string)
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 类型错误，期望 string，实际 %T", deviceName, eidValue)
		return err
	}
	eidStr = "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	// 构建复位帧
	loc := time.FixedZone("CST", 8*3600)    // 北京时区
	ts := uint32(time.Now().In(loc).Unix()) // 当前时间转为世纪秒

	// 发送命令
	eidStr, _ = eidValue.(string)
	RestCommandBuildFrame(eidStr, sensorID, 1, ts)

	return nil
}
