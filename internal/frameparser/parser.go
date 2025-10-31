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

// deviceName: 设备名
// sourceName: 资源名
// resourceNames: 数据值列表
type CallbackFunc func(deviceName, sourceName string, values map[string]interface{})

// LORA协议解析
func StartParser(frameCh <-chan []byte, cb CallbackFunc) {
	go func() {
		for frame := range frameCh {
			fmt.Printf("Received frame (%d bytes): % X\n", len(frame), frame)
			// 长度校验
			if len(frame) < 9 {
				log.Println("帧长度不足，跳过解析")
				continue
			}
			// CRC 校验
			payload := frame[:len(frame)-2]
			recvCRC := binary.BigEndian.Uint16(frame[len(frame)-2:])
			// 解析EID
			sidBytes := frame[0:6]
			sensorID := strings.ToUpper(hex.EncodeToString(sidBytes))
			deviceName, hasDevice := config.LookupDeviceName(sensorID)
			if !hasDevice {
				log.Printf("EID映射表 key: %#v", config.SensorIDToDeviceName)

				log.Printf(">>[%s]<<", sensorID)

				log.Printf("未知 EID=%s，跳过本帧", sensorID)
				continue
			}

			// 头部
			head := frame[6]
			dataCount := int(head >> 4)
			fragInd := (head >> 3) & 0x1
			packetType := head & 0x07
			body := make([]byte, len(frame)-2-7)
			copy(body, frame[7:len(frame)-2])
			if CRC16(payload) != recvCRC {
				if fragInd == 0 {
					switch packetType {
					case 0:
						SendDataStatus(sensorID, 0b001, 0x00, byte(dataCount))
						// 监测报文
					case 2:
						SendDataStatus(sensorID, 0b011, 0x00, byte(dataCount))
						// 告警报文
					default:
						continue
					}
				}
				log.Println("CRC 校验失败，跳过解析")
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
			if fragInd == 0 {
				// 非分片帧：只处理业务或控制报文
				switch packetType {
				case 0:
					SendDataStatus(sensorID, 0b001, 0xFF, byte(dataCount))
					// 监测报文
				case 2:
					SendDataStatus(sensorID, 0b011, 0xFF, byte(dataCount))
					// 告警报文
				case 4, 5:
					// 控制报文响应
					handleFrameCtl(frame_ctl)
					if config.ResourcesFlag {
						cb(deviceName, "AsyncData", config.Resources1)
						config.ResourcesFlag = false
					}
					continue
				default:
					continue
				}
			} else {
			}
			idx := 7
			parsed := 0
			resourceValues := make(map[string]interface{})
			for parsed < dataCount {
				// 参数头2字节
				if idx+2 > len(frame)-2 {
					log.Printf("参数头越界 SensorID=%s，跳过本帧", sensorID)
					break
				}
				head16 := binary.LittleEndian.Uint16(frame[idx : idx+2])
				idx += 2
				paramType := head16 >> 2       // 14bit类型码
				lenFlag := uint8(head16 & 0x3) // 2bit长度指示
				// 数据长度
				var dataLen uint32
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
				// 提取原始值字节
				log.Printf("lenFlag=%d dataLen=%d idx=%d frameLen=%d", lenFlag, dataLen, idx, len(frame))

				valBytes := frame[idx : idx+int(dataLen)]
				idx += int(dataLen)
				// 解析数据
				if info, ok := config.LookupParamInfo(paramType); ok {
					val, err := info.Parse(valBytes)
					if err != nil {
						log.Printf("参数 %s.%s 解析失败: %v", deviceName, "info.Name", err)
					} else {
						// 写入运行时值表
						if val != nil {
							config.SetDeviceValue(deviceName, "info.Name", val)
							resourceValues["info.Name"] = val
							log.Printf("写入值 %s.%s = %v %s", deviceName, "info.Name", val, "info.Name")
						}
					}
				} else {
					log.Printf("未找到参数类型信息 type=0x%X", paramType)
				}
				parsed++
			}
			log.Printf("[DEBUG] parsed=%d dataCount=%d len(resourceValues)=%d cb=%v",
				parsed, dataCount, len(resourceValues), cb != nil)

			// 解析完成，调用回调
			fmt.Printf("cb=%v, len(resourceValues)=%d\n", cb, len(resourceValues))

			if cb != nil && len(resourceValues) > 0 {
				cb(deviceName, "AsyncData", resourceValues)
			}
			if parsed < dataCount {
				continue
			}
		}
	}()
}

// 监测数据响应报文
func SendDataStatus(sensorKey string, packetType byte, dataStatus byte, dataLen byte) error {

	// EID
	keyBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		return errors.New("invalid sensorKey hex: " + err.Error())
	}
	if len(keyBytes) != 6 {
		return errors.New("sensorKey hex must decode to 6 bytes")
	}
	//头
	const fragInd = 0
	header := (dataLen<<4)&0xF0 | (fragInd<<3)&0x08 | (packetType & 0x07)
	//拼接
	packet := make([]byte, 0, len(keyBytes)+1+1+2)
	packet = append(packet, keyBytes...)
	packet = append(packet, header)
	packet = append(packet, dataStatus)
	//CRC16
	crc := CRC16(packet)
	packet = append(packet, byte(crc>>8), byte(crc&0xFF))
	//发送
	relay.SendFrame(sensorKey, packet)
	return nil
}
