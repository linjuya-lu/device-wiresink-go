package config

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"time"
)

type Frame struct {
	SensorID   string //EID
	DataLen    byte   //参量个数
	FragInd    byte   //分片指示
	PacketType byte   //报文类型
	Payload    []byte //报文内容
	Check      uint16 //校验位
}

func (f *Frame) Bytes() []byte {
	buf := make([]byte, 0, 6+1+1+1+len(f.Payload)+2)
	buf = append(buf, f.SensorID[:]...)
	buf = append(buf, f.DataLen)
	flags := (f.DataLen << 4) | byte(f.FragInd<<3) | byte(f.PacketType)
	buf = append(buf, flags)
	buf = append(buf, f.Payload...)
	crc := []byte{byte(f.Check), byte(f.Check >> 8)}
	buf = append(buf, crc...)
	return buf
}

type ResponseKey struct {
	CtrlType       uint8 //控制报文类型：低7位
	RequestSetFlag bool  //参数配置类型标识：1位
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

// 控制解析函数
var (
	ControlResources          = make(map[string]any)
	ControlResourcesFlag bool = false
)

// 通用参数查询/设置
func common_para_response(data []byte, frameCtl Frame) error {
	idx := 0
	parsed := 0
	ControlResources = make(map[string]any)
	ControlResourcesFlag = false
	for parsed < int(frameCtl.DataLen) {
		if idx+2 > len(data)-2 {
			log.Printf("参数头越界SensorID=%s，跳过本帧", frameCtl.SensorID)
			break
		}
		head16 := binary.LittleEndian.Uint16(data[idx : idx+2])
		idx += 2
		paramType := head16 >> 2       //类型码
		lenFlag := uint8(head16 & 0x3) //长度指示
		var dataLen uint32
		switch lenFlag {
		case 0:
			dataLen = 4
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
		if idx+int(dataLen) > len(data)-2 {
			log.Printf("参数数据越界SensorID=%s，跳过本帧", frameCtl.SensorID)
			break
		}
		valBytes := data[idx : idx+int(dataLen)]
		idx += int(dataLen)
		deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
		if !hasDevice {
			log.Printf("未知SensorID=%s，跳过本帧", frameCtl.SensorID)
			continue
		}
		if info, ok := LookupParamInfo(paramType); ok {
			val, err := info.Parse(valBytes)
			key := ParamKey{
				FeatureBits: uint8((paramType >> 11) & 0x07),
				CodeBits:    uint16(paramType & 0x07FF),
			}
			var resName string
			if rn, ok := ParamEidGet(key, deviceName); ok {
				resName = rn
				fmt.Println("命中资源名：", resName)
			} else {
				fmt.Println("未找到绑定")
				continue
			}
			SetDeviceValue(deviceName, resName, val)
			ControlResources[resName] = val
			if err != nil {
				log.Printf("参数%s.%s解析失败: %v", deviceName, resName, err)
			} else {
				SetDeviceValue(deviceName, resName, val)
				ControlResources[resName] = val
				log.Printf("写入值%s.%s=%v", deviceName, resName, val)
			}
		} else {
			log.Printf("未找到参数类型信息type=0x%X", paramType)
		}
		parsed++
	}
	ControlResourcesFlag = true
	return nil
}

// 时间参数查询/设置
func timestamp_response(data []byte, frameCtl Frame) error {
	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	log.Printf("时间=0x%02X", data[0])
	secs := binary.LittleEndian.Uint32(data[0:4])
	t := time.Unix(int64(secs), 0)
	SetDeviceValue(deviceName, "timestampStr", t.Format("2006-01-02 15:04:05"))
	return nil
}

// 复位设置
func reset_response(data []byte, frameCtl Frame) error {
	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	reset_ctl := "reset_ctl"
	strVal := strconv.Itoa(int(data[0]))
	SetDeviceValue(deviceName, reset_ctl, strVal)
	return nil
}

func resetCommands(data []byte, frameCtl Frame) error {
	deviceName, hasDevice := LookupDeviceName(frameCtl.SensorID)
	if !hasDevice {
		log.Printf("未知SensorID=%s，跳过本帧", frameCtl.SensorID)
	}
	eidValue, ok := GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备%s的EID未初始化", deviceName)
		return err
	}
	eidBytes, err := hex.DecodeString(EidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s]转十六进制失败: %w", EidStr, err)
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID长度不对,实际%d字节", len(eidBytes))
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建复位帧
	loc := time.FixedZone("CST", 8*3600)
	ts := uint32(time.Now().In(loc).Unix()) //当前时间转为世纪秒
	eidStr, _ := eidValue.(string)
	RestCommandBuildFrame(eidStr, sensorID, 1, ts)
	return nil
}
