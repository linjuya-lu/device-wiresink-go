package frameparser

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/relay"
)

type CallbackFunc func(deviceName, sourceName string, values map[string]any)

func StartParser(frameCh <-chan []byte, cb CallbackFunc) {
	go func() {
		for frame := range frameCh {
			fmt.Printf("Received frame (%d bytes): % X\n", len(frame), frame)
			if len(frame) < 9 {
				continue
			}
			payload := frame[:len(frame)-2]
			recvCRC := binary.BigEndian.Uint16(frame[len(frame)-2:])
			sidBytes := frame[0:6]
			sensorID := strings.ToUpper(hex.EncodeToString(sidBytes))
			deviceName, hasDevice := config.LookupDeviceName(sensorID)
			if !hasDevice {
				log.Printf("未知 EID=%s，跳过本帧", sensorID)
				continue
			}
			// 头部
			head := frame[6]
			dataCount := int(head >> 4)
			fragInd := (head >> 3) & 0x1
			packetType := head & 0x07
			// 负载
			body := make([]byte, len(frame)-9)
			copy(body, frame[7:len(frame)-2])
			// 错误响应
			if config.CRC16(payload) != recvCRC {
				if fragInd == 0 {
					switch packetType {
					case 0: //监测报文
						SendDataStatus(sensorID, 0b001, 0x00, byte(dataCount))
					case 2: //告警报文
						SendDataStatus(sensorID, 0b011, 0x00, byte(dataCount))
					default:
						continue
					}
				}
				log.Println("CRC 校验失败")
				continue
			}
			frame_ctl := config.Frame{
				SensorID:   sensorID,
				DataLen:    byte(dataCount),
				FragInd:    fragInd,
				PacketType: packetType,
				Payload:    body,
				Check:      recvCRC,
			}
			// 正常响应
			if fragInd == 0 {
				switch packetType {
				case 0: //监测报文
					SendDataStatus(sensorID, 0b001, 0xFF, byte(dataCount))
				case 2: //告警报文
					SendDataStatus(sensorID, 0b011, 0xFF, byte(dataCount))
				case 4, 5: //控制报文与处理
					handleFrameCtl(frame_ctl)
					if config.ControlResourcesFlag {
						cb(deviceName, "asyncData", config.ControlResources)
						config.ControlResourcesFlag = false
					}
					continue
				default:
					continue
				}
			} else {
			}
			// 业务数据
			idx := 7
			parsed := 0
			resourceValues := make(map[string]any)
			for parsed < dataCount {
				if idx+2 > len(frame)-2 {
					log.Printf("业务参数类型越界 SensorID=%s", sensorID)
					break
				}
				head16 := binary.LittleEndian.Uint16(frame[idx : idx+2])
				idx += 2
				paramType := head16 >> 2       //参量类型
				lenFlag := uint8(head16 & 0x3) //数据长度指示
				var dataLen uint32             //数据长度
				switch lenFlag {
				case 0:
					dataLen = 4
				case 1:
					dataLen = uint32(frame[idx])
					idx++
				case 2:
					dataLen = uint32(binary.LittleEndian.Uint16(frame[idx : idx+2]))
					idx += 2
				case 3:
					dataLen = uint32(frame[idx])<<16 | uint32(frame[idx+1])<<8 | uint32(frame[idx+2])
					idx += 3
				}
				log.Printf("lenFlag=%d dataLen=%d idx=%d frameLen=%d", lenFlag, dataLen, idx, len(frame))
				// 解析数据
				valBytes := frame[idx : idx+int(dataLen)]
				idx += int(dataLen)
				// 特殊处理
				if paramType == 0x0008 {
					if topo, err := config.ParseTopo(valBytes); err != nil {
						log.Printf("拓扑参数解析失败 SensorID=%s type=0x%X: %v", sensorID, paramType, err)
					} else {
						log.Printf("拓扑参数解析成功 SensorID=%s type=0x%X 结果=%v", sensorID, paramType, topo)
					}
					parsed++
					continue
				}
				if info, ok := config.LookupParamInfo(paramType); ok {
					val, err := info.Parse(valBytes)
					key := config.ParamKey{
						FeatureBits: uint8((paramType >> 11) & 0x07), //3位
						CodeBits:    uint16(paramType & 0x07FF),      //11位
					}
					var resName string
					if rn, ok := config.ParamEidGet(key, deviceName); ok {
						resName = rn
						fmt.Println("命中资源名：", resName)
					} else {
						fmt.Print("未找到绑定", deviceName)
						continue
					}
					config.SetDeviceValue(deviceName, resName, val)
					resourceValues[resName] = val
					if err != nil {
						log.Printf("参数 %s.%s 解析失败: %v", deviceName, resName, err)
					} else {
						//写入映射表
						if val != nil {
							config.SetDeviceValue(deviceName, resName, val)
							resourceValues[resName] = val
							log.Printf("写入值 %s.%s = %v %s", deviceName, resName, val, resName)
						}
					}
				} else {
					log.Printf("未找到参数类型信息 type=0x%X", paramType)
				}
				parsed++
			}
			if cb != nil && len(resourceValues) > 0 {
				cb(deviceName, "asyncData", resourceValues)
			}
			if parsed < dataCount {
				continue
			}
		}
	}()
}

// 监测数据响应报文
func SendDataStatus(sensorKey string, packetType byte, dataStatus byte, dataLen byte) error {
	//EID
	keyBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		return errors.New("invalid sensorKey hex: " + err.Error())
	}
	if len(keyBytes) != 6 {
		return errors.New("sensorKey hex must decode to 6 bytes")
	}
	// 头
	const fragInd = 0
	header := (dataLen<<4)&0xF0 | (fragInd<<3)&0x08 | (packetType & 0x07)
	// 拼接
	packet := make([]byte, 0, len(keyBytes)+1+1+2)
	packet = append(packet, keyBytes...)
	packet = append(packet, header)
	packet = append(packet, dataStatus)
	crc := config.CRC16(packet)
	packet = append(packet, byte(crc>>8), byte(crc&0xFF))
	relay.SendFrame(sensorKey, packet)
	return nil
}
