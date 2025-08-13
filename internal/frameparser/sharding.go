package frameparser

// 实现第8章和附录H分片解析、确认及重传机制
import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

// SDUCache 用于缓存分片数据，并记录最后一次确认重传次数
type SDUCache struct {
	SSEQ       uint8
	expected   uint8
	finalPSEQ  uint8
	buffer     []byte
	outOfOrder map[uint8][]byte
	timer      *time.Timer
}

const (
	// 分片最大重传次数，由传感器端根据ACK逻辑重试
	// maxRetransmits = 3
	// 重组超时，超过此时间未完成拼接则丢弃并回ACK失败
	reassembleTimeout = 20 * time.Second
)

var (
	cacheMu sync.Mutex
	// 使用 string 作为键，即 frame.SensorID
	sduCaches = make(map[string]*SDUCache)
	// 重组后的完整 SDU 交给此通道
	SDUCh = make(chan config.Frame, 100)
)

// ProcessFrame 处理从上层收到的 config.Frame
// 包括提取分片头、缓存重组、ACK应答、超时丢弃
func ProcessFrame(frame config.Frame) {
	i := 0
	two := binary.BigEndian.Uint16(frame.Payload[i : i+2])
	i += 2
	SSEQ := uint8(two >> 10) // 6bit
	sensorKey := frame.SensorID
	// 如果未分片，直接应答ACK并输出
	if frame.FragInd == 0 {
		sendAck(sensorKey, SSEQ, true, 0)
		SDUCh <- frame
		return
	}
	// 解析PDU头，需至少4字节
	if len(frame.Payload) < 4 {
		fmt.Printf("PDU头太短: %d字节\n", len(frame.Payload))
		sendAck(sensorKey, SSEQ, false, 0)
		return
	}
	// 游标
	PSEQ := uint8((two >> 1) & 0x7F) // 7bit
	i += 2                           // Size 字段占2字节前移过度?
	size := binary.LittleEndian.Uint16(frame.Payload[i-2 : i])
	if len(frame.Payload) < i+int(size) {
		fmt.Printf("PDU 数据越界: 期望%d, 实际%d\n", size, len(frame.Payload)-i)
		sendAck(sensorKey, SSEQ, false, PSEQ)
		return
	}
	data := frame.Payload[i : i+int(size)]
	cacheMu.Lock()
	cache, exists := sduCaches[sensorKey]
	// 首片
	if !exists {
		if isStart(PSEQ) {
			cache = &SDUCache{SSEQ: SSEQ, expected: PSEQ + 1, outOfOrder: make(map[uint8][]byte)}
			cache.buffer = append(cache.buffer, data...)
			cache.timer = time.AfterFunc(reassembleTimeout, func() {
				cacheMu.Lock()
				delete(sduCaches, sensorKey)
				cacheMu.Unlock()
				// 超时丢弃后发失败ACK
				sendAck(sensorKey, SSEQ, false, PSEQ)
			})
			sduCaches[sensorKey] = cache
			// 对每片都应答ACK成功
			sendAck(sensorKey, SSEQ, true, PSEQ)
			if isEnd(PSEQ) {
				cache.finalPSEQ = PSEQ
				finalize(sensorKey, frame)
			}
		} else {
			// 收到非首片但无缓存，直接应答失败ACK
			sendAck(sensorKey, SSEQ, false, PSEQ)
		}
		cacheMu.Unlock()
		return
	}
	// 不同SSEQ的新首片，重启
	if cache.SSEQ != SSEQ {
		if isStart(PSEQ) {
			cache.timer.Stop()
			delete(sduCaches, sensorKey)
			cacheMu.Unlock()
			ProcessFrame(frame)
			return
		}
		sendAck(sensorKey, SSEQ, false, PSEQ)
		cacheMu.Unlock()
		return
	}
	// 同一SSEQ的分片处理
	switch {
	case PSEQ < cache.expected:
		// 重复片，无需拼接，但仍ACK成功
		sendAck(sensorKey, SSEQ, true, PSEQ)
	case PSEQ > cache.expected:
		// 超前片，乱序缓存
		cache.outOfOrder[PSEQ] = data
		sendAck(sensorKey, SSEQ, true, PSEQ)
		if isEnd(PSEQ) {
			cache.finalPSEQ = PSEQ
		}
	default:
		// 顺序片
		cache.buffer = append(cache.buffer, data...)
		cache.expected++
		sendAck(sensorKey, SSEQ, true, PSEQ)
		if isEnd(PSEQ) {
			cache.finalPSEQ = PSEQ
		}
		// 合并乱序片
		for {
			nxt := cache.expected
			d, ok2 := cache.outOfOrder[nxt]
			if !ok2 {
				break
			}
			delete(cache.outOfOrder, nxt)
			cache.buffer = append(cache.buffer, d...)
			cache.expected++
		}
		if cache.finalPSEQ != 0 && cache.expected > cache.finalPSEQ {
			finalize(sensorKey, frame)
		}
	}
	cacheMu.Unlock()
}

// finalize 停止定时器并输出完整SDU
func finalize(sensorKey string, frame config.Frame) {
	cache := sduCaches[sensorKey]
	cache.timer.Stop()
	delete(sduCaches, sensorKey)
	copy(frame.Payload, cache.buffer)
	SDUCh <- frame
}

// sendAck 构造并发送 ACK 帧：ackOK=true 则 ACK=11，否则 ACK=00
func sendAck(sensorKey string, sseq uint8, ackOK bool, pseq uint8) {
	var ackBits uint8
	if ackOK {
		ackBits = 0x3 // 二进制11
	} else {
		ackBits = 0x0 // 二进制00
	}
	// 构造 ACK 控制字段：ACK(2bit)|SSEQ(6bit)|PSEQ(7bit)
	two := (uint16(ackBits&0x3) << 14) | (uint16(sseq&0x3F) << 8) | uint16(pseq&0x7F)
	ackData := make([]byte, 2)
	binary.BigEndian.PutUint16(ackData, two)
	// 构造 Frame 并发送
	ackFrame := config.Frame{
		SensorID:   sensorKey,
		DataLen:    1,     // 参数个数1
		FragInd:    0,     // 完整帧
		PacketType: 0b110, // 分片应答报文
		Payload:    ackData,
		Check:      CRC16(ackData),
	}
	data := ackFrame.Bytes()
	relay.SendFrame(sensorKey, data)
}

// isStart/PSEQ 首尾判断
func isStart(pseq uint8) bool { return (pseq>>7)&0x1 == 0 }
func isEnd(pseq uint8) bool   { return (pseq>>7)&0x1 == 1 }

func ShardingParser(frameCh <-chan config.Frame) error {
	for frame := range frameCh {
		idx := 0
		parsed := 0
		for parsed < int(frame.DataLen) {
			// 参数头2字节
			if idx+2 > len(frame.Payload)-2 {
				log.Printf("参数头越界 SensorID=%s，跳过本帧", frame.SensorID)
				break
			}
			head16 := binary.LittleEndian.Uint16(frame.Payload[idx : idx+2])
			idx += 2
			paramType := head16 >> 2       // 14bit类型码
			lenFlag := uint8(head16 & 0x3) // 2bit长度指示
			// 计算真实数据长度
			var dataLen uint32
			switch lenFlag {
			case 0:
				dataLen = 4 // 默认4字节
			case 1:
				dataLen = uint32(frame.Payload[idx])
				idx++
			case 2:
				dataLen = uint32(binary.BigEndian.Uint16(frame.Payload[idx : idx+2]))
				idx += 2
			case 3:
				dataLen = uint32(frame.Payload[idx])<<16 | uint32(frame.Payload[idx+1])<<8 | uint32(frame.Payload[idx+2])
				idx += 3
			}
			// 数据越界校验
			if idx+int(dataLen) > len(frame.Payload)-2 {
				log.Printf("参数数据越界 SensorID=%s，跳过本帧", frame.SensorID)
				break
			}
			// 提取原始值字节
			valBytes := frame.Payload[idx : idx+int(dataLen)]
			idx += int(dataLen)
			deviceName, hasDevice := config.LookupDeviceName(frame.SensorID)
			if !hasDevice {
				log.Printf("未知 SensorID=%s，跳过本帧", frame.SensorID)
				continue
			}
			// 解析数据
			if info, ok := config.LookupParamInfo(paramType); ok {
				val, err := info.Parse(valBytes)
				if err != nil {
					log.Printf("❌ 参数 %s.%s 解析失败: %v", deviceName, info.Name, err)
				} else {
					// 写入运行时值表
					config.SetDeviceValue(deviceName, info.Name, val)
					log.Printf("✅ 写入值 %s.%s = %v %s", deviceName, info.Name, val, info.Unit)
				}
			} else {
				log.Printf("未找到参数类型信息 type=0x%X", paramType)
			}

			parsed++
		}
	}
	return nil
}
