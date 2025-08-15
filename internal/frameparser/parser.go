package frameparser

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

// deviceName: 设备名称
// sourceName: 上报的源名称
// resourceNames: 已解析的资源名列表
type CallbackFunc func(deviceName, sourceName string, values map[string]interface{})

// 依照《Q/GDW 12184—2021》附录 D 业务报文格式，实现以下功能：
// 1. 提取 SensorID、报文类型（仅处理业务数据：监测和告警）  控制报文与控制报文响应单独函数处理
// 2. 根据 DataLen（4bit）、FragInd（1bit）、PacketType（3bit）判断是否处理
// 3. 分片帧（FragInd=1）开协程处理
// 4. 按照参量个数逐个解析 ParamType(14bit)+LengthFlag(2bit) + 可选长度字段 + 数据
// 5. 将数值按表转换为 float32/float64/int8等基本类型
// 6. 针对 SensorID，调用 config.SetDeviceValue 存储解析结果
func StartParser(frameCh <-chan []byte, cb CallbackFunc) {
	// fmt.Printf("[StartParser] cb=%p\n", cb)

	go func() {
		for frame := range frameCh {
			fmt.Printf("Received frame (%d bytes): % X\n", len(frame), frame)
			// 最小长度校验：6字节ID +1字节头 +2字节CRC
			if len(frame) < 9 {
				log.Println("帧长度不足，跳过解析")
				continue
			}
			// CRC 校验：最后 2 字节为 CRC-16
			payload := frame[:len(frame)-2]
			recvCRC := binary.BigEndian.Uint16(frame[len(frame)-2:])
			// 读取6字节SensorID，使用Hex字符串表示
			sidBytes := frame[0:6]
			sensorID := strings.ToUpper(hex.EncodeToString(sidBytes))
			deviceName, hasDevice := config.LookupDeviceName(sensorID)
			if !hasDevice {
				log.Printf("EID映射表 key: %#v", config.SensorIDToDeviceName)

				log.Printf(">>[%s]<<", sensorID)

				log.Printf("未知 EID=%s，跳过本帧", sensorID)
				continue
			}
			//更新维护时间
			onDataReceived(deviceName)
			// 头部：4bit DataLen、1bit FragInd、3bit PacketType
			head := frame[6]
			dataCount := int(head >> 4)  // 参量个数
			fragInd := (head >> 3) & 0x1 // 分片指示
			packetType := head & 0x07    // 报文类型
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
						cb(deviceName, "AsyncReporting", config.Resources1)
						config.ResourcesFlag = false
					}
					continue
				default:
					// 其他不处理
					continue
				}
			} else {
				// 分片帧
				ProcessFrame(frame_ctl)
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
				// 计算真实数据长度
				var dataLen uint32
				switch lenFlag {
				case 0:
					dataLen = 4 // 默认4字节
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
						log.Printf("❌ 参数 %s.%s 解析失败: %v", deviceName, info.Name, err)
					} else {
						// 写入运行时值表
						if val != nil {
							config.SetDeviceValue(deviceName, info.Name, val)
							resourceValues[info.Name] = val
							log.Printf("✅ 写入值 %s.%s = %v %s", deviceName, info.Name, val, info.Unit)
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
				cb(deviceName, "AsyncReporting", resourceValues)
			}
			// 若未完全解析，跳过后续逻辑
			if parsed < dataCount {
				continue
			}
		}
	}()
}

// 构造并发送“监测数据响应”报文
// 协议格式: [SensorID(6)][Header(1)][Data_Status(1)][CRC16(2)]
//   - SensorID: 字符串
//   - Header:
//     高4位：DataLen (参数个数)
//     第3位：FragInd (0=未分片)
//     低3位：PacketType (0b001=监测数据响应)
//   - Data_Status: 上传状态 0xFF 成功，0x00 失败
//   - CRC16: 对整帧前 8 字节 CRC16 校验，高低字节附加
func SendDataStatus(sensorKey string, packetType byte, dataStatus byte, dataLen byte) error {
	var eidStr = "238A0841D828"
	// 解码 EID
	keyBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		return errors.New("invalid sensorKey hex: " + err.Error())
	}
	if len(keyBytes) != 6 {
		return errors.New("sensorKey hex must decode to 6 bytes")
	}
	// 构造 Header
	const fragInd = 0 // 未分片
	header := (dataLen<<4)&0xF0 | (fragInd<<3)&0x08 | (packetType & 0x07)
	// 拼接帧：SensorID + Header + Data_Status
	packet := make([]byte, 0, len(keyBytes)+1+1+2)
	packet = append(packet, keyBytes...)
	packet = append(packet, header)
	packet = append(packet, dataStatus)
	//计算 CRC16
	crc := CRC16(packet)
	packet = append(packet, byte(crc>>8), byte(crc&0xFF))
	//发送
	relay.SendFrame(sensorKey, packet)
	return nil
}

func onDataReceived(deviceName string) {
	// 写入时间戳（纳秒）
	ts := time.Now().UnixNano()
	config.SetDeviceValue(deviceName, "lastDataTimestamp", ts)
}
