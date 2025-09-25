package frameparser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
)

// ===== 协议常量 =====
const (
	syncHi byte = 0x5A
	syncLo byte = 0xA5
	endTag byte = 0x96
)

const (
	PktUpgradeResp   byte = 0xB1 // 表2：网关响应边代升级请求
	PktUpgradeState  byte = 0xD1 // 表7：主动上传当前升级状态
	PktComplementReq byte = 0xB4 // 表5：后台升级补包请求（从机发送）
)

// Device_State（表7）
const (
	DevStateIdle      = 1
	DevStateUpgrading = 2
	DevStateDone      = 3
	DevStateFailed    = 4
)

// Command_Status（表2）
const (
	CommandStatusSuccess = 0xFF
	CommandStatusFailed  = 0x00
)

// ===== 你的全局标志（示例）=====
var (
	muReady   sync.Mutex
	readyFlag uint8 // 0 未就绪；1 已就绪；2 失败（可选）
)

// 置就绪标志
func setReady(ok bool) {
	muReady.Lock()
	if ok {
		readyFlag = 1
	} else {
		readyFlag = 0 // 也可置 2 表示失败
	}
	muReady.Unlock()
}

// 你已有的 ACK 标志接口
// func SetAck(received bool) { ... }

// 统一的帧对象（按字段位序）
type frame struct {
	SyncHi, SyncLo byte     // 0..1
	PacketLen      uint16   // 2..3  总长度（含头尾）
	CmdID          [17]byte // 4..20  ASCII/GB2312，不足补0
	FrameType      byte     // 21
	PacketType     byte     // 22
	FrameNo        byte     // 23
	Payload        []byte   // 24..(len-1-2-1) 去掉 CRC16 与 End
	CRC16          uint16   // len-3..len-2
	End            byte     // len-1
}

// 计算 CRC16（占位；替换为你的具体实现，如 Modbus/IBM/X25）
func crc16(data []byte) uint16 {
	// TODO: 根据协议替换具体多项式
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
	}
	return crc
}

// 尝试从缓冲中解出一帧；返回帧/消耗字节数/错误
func TryDecodeOne(buf []byte) (fr *frame, used int, err error) {
	// 寻找同步头 5A A5
	i := bytes.Index(buf, []byte{syncHi, syncLo})
	if i < 0 {
		return nil, len(buf), io.EOF // 整段都无 sync，全部丢弃
	}
	if len(buf[i:]) < 6 { // 至少要有头+长度+基本字段
		return nil, i, io.ErrUnexpectedEOF // 只丢弃 sync 前垃圾
	}

	// 长度（大端）
	pktLen := binary.BigEndian.Uint16(buf[i+2 : i+4])
	if int(pktLen) > len(buf[i:]) {
		return nil, i, io.ErrUnexpectedEOF // 数据不完整，保留等待更多字节
	}

	raw := buf[i : i+int(pktLen)]
	// 末尾检查 End
	if raw[len(raw)-1] != endTag {
		// 不是一帧，跳过当前 sync 继续找
		return nil, i + 2, fmt.Errorf("bad end tag")
	}

	// CRC 校验：从 Sync 到 CRC 前一字节
	body := raw[:len(raw)-3]
	crc := binary.BigEndian.Uint16(raw[len(raw)-3 : len(raw)-1])
	if crc != crc16(body) {
		return nil, i + int(pktLen), fmt.Errorf("crc mismatch")
	}

	// 解析字段
	var f frame
	f.SyncHi, f.SyncLo = raw[0], raw[1]
	f.PacketLen = pktLen
	copy(f.CmdID[:], raw[4:4+17])
	f.FrameType = raw[21]
	f.PacketType = raw[22]
	f.FrameNo = raw[23]
	f.Payload = raw[24 : len(raw)-3]
	f.CRC16 = crc
	f.End = endTag

	return &f, i + int(pktLen), nil
}

// 表2：升级请求响应（Packet_Type=0xB1）
// Payload: 仅 1 字节 Command_Status
func HandleUpgradeResp(f *frame) {
	if len(f.Payload) < 1 {
		return
	}
	status := f.Payload[0]
	if status == CommandStatusSuccess {
		setReady(true) // 就绪
	} else {
		setReady(false) // 未就绪/失败
	}
}

// 表7：升级状态（Packet_Type=0xD1）
// Payload: Device_State(1) + Description_Size(1) + Description(N)
func HandleUpgradeState(f *frame) {
	if len(f.Payload) < 1 {
		return
	}
	deviceState := f.Payload[0]

	// 是否视为“收到了该包”的 ACK
	if deviceState == DevStateUpgrading {
		config.SetAck(true)
	} else {
		config.SetAck(false)
	}
	// 其它状态你可以按需扩展日志/上报
}

// 表5：补包请求（Packet_Type=0xB4）
// Payload: File_Name(32) + ComplementPack_Sum(2) + ComplementPack_No(2×N)
func HandleComplementReq(f *frame, deviceName string) {
	if len(f.Payload) < 34 { // 32+2 最小
		return
	}
	// fileName := string(bytes.TrimRight(f.Payload[:32], "\x00")) // 如需使用文件名

	sum := binary.BigEndian.Uint16(f.Payload[32:34])
	raw := f.Payload[34:]
	var nos []uint16
	for i := 0; i+1 < len(raw); i += 2 {
		n := binary.BigEndian.Uint16(raw[i : i+2])
		nos = append(nos, n)
	}
	CompReg.Set(deviceName, sum, nos)
}
