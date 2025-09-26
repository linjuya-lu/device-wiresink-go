package frameparser

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
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
	MuReady   sync.RWMutex
	ReadyFlag uint8 // 0 未就绪；1 已就绪；2 失败（可选）
)

// 置就绪标志
func setReady(ok bool) {
	MuReady.Lock()
	if ok {
		ReadyFlag = 1
	} else {
		ReadyFlag = 0 // 也可置 2 表示失败
	}
	MuReady.Unlock()
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

func ParseFrameBytes(b []byte) (*frame, error) {
	const endTag = 0x96
	if len(b) < 7 {
		return nil, fmt.Errorf("short frame: %d", len(b))
	}

	// 头+端序
	be := true
	switch {
	case b[0] == 0x5A && b[1] == 0xA5:
		be = true
	case b[0] == 0xA5 && b[1] == 0x5A:
		be = false
	default:
		return nil, fmt.Errorf("bad sync: %02X %02X", b[0], b[1])
	}

	// 协议中的 Packet_Length（=载荷长度，从 CMD_ID 起算）
	var plen uint16
	if be {
		plen = binary.BigEndian.Uint16(b[2:4])
	} else {
		plen = binary.LittleEndian.Uint16(b[2:4])
	}

	// 计算期望总长 & 实际载荷长度
	totalExpected := int(2 + 2 + plen + 2 + 1) // 4 + plen + 3
	if len(b) < totalExpected {
		return nil, fmt.Errorf("incomplete frame: have=%d expect=%d (plen=%d)", len(b), totalExpected, plen)
	}
	if b[len(b)-1] != endTag {
		return nil, fmt.Errorf("bad end tag: 0x%02X", b[len(b)-1])
	}

	// // CRC（字段本身通常 BE 存放）
	// crcGot := binary.BigEndian.Uint16(b[len(b)-3 : len(b)-1])

	// // 尝试多种 CRC 范围（任一通过即可）
	// okCRC := false
	// // B) 从 CMD_ID 起，长度=报文声明的 plen
	// if 4+int(plen) <= len(b)-3 && crc16(b[4:4+int(plen)]) == crcGot {
	// 	okCRC = true
	// }
	// // A) 从 Packet_Length 字段起到 CRC 前
	// if !okCRC && crc16(b[2:len(b)-3]) == crcGot {
	// 	okCRC = true
	// }
	// // C) 兼容“报文声明长度错误”的情况：用“实际载荷长度”验
	// actualPlen := len(b) - 7 // = len - (Sync2+Len2) - (CRC2+End1)
	// if !okCRC && actualPlen >= 0 && 4+actualPlen <= len(b)-3 {
	// 	if crc16(b[4:4+actualPlen]) == crcGot {
	// 		okCRC = true
	// 	}
	// }
	// if !okCRC {
	// 	return nil, fmt.Errorf("crc mismatch: got=0x%04X", crcGot)
	// }

	// 固定头边界
	if 4+17+1+1+1 > len(b)-3 {
		return nil, fmt.Errorf("frame too short for header fields")
	}

	var f frame
	f.SyncHi, f.SyncLo = b[0], b[1]
	f.PacketLen = plen // ← 这里实际上是“载荷长度(=Packet_Length)”，建议把注释改掉
	copy(f.CmdID[:], b[4:4+17])
	f.FrameType = b[21]
	f.PacketType = b[22]
	f.FrameNo = b[23]

	// payload：从 24 起直到 CRC 前
	if 24 <= len(b)-3 {
		f.Payload = b[24 : len(b)-3]
	}
	// f.CRC16 = crcGot
	f.End = endTag

	// 如需调试“报文声明长度 vs 实际长度”的不一致，可在此处打印：
	// if len(b) != totalExpected {
	//     fmt.Printf("[WARN] length mismatch: len(b)=%d expect=%d (plen=%d actualPlen=%d)\n",
	//         len(b), totalExpected, plen, actualPlen)
	// }

	return &f, nil
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

// 仅打印关键信息：头部 + B1/D1 的关键字段
func PrintFrameBrief(fr *frame) {
	id := strings.TrimRight(string(fr.CmdID[:]), "\x00")
	fmt.Printf("[FRAME] ID=%s FT=0x%02X PT=0x%02X NO=%d payload=%d CRC=0x%04X\n",
		id, fr.FrameType, fr.PacketType, fr.FrameNo, len(fr.Payload), fr.CRC16)

	switch fr.PacketType {
	case PktUpgradeResp: // 0xB1
		var st byte
		if len(fr.Payload) >= 1 {
			st = fr.Payload[len(fr.Payload)-1] // B1 的 payload 就 1 字节：Command_Status
		}
		txt := map[byte]string{0xFF: "成功", 0x00: "失败"}[st]
		if txt == "" {
			txt = fmt.Sprintf("未知(0x%02X)", st)
		}
		fmt.Printf("[B1] Command_Status=%s (0x%02X)\n", txt, st)

	case PktUpgradeState: // 0xD1
		if len(fr.Payload) >= 1 {
			state := fr.Payload[0] // Device_State
			txt := map[byte]string{
				1: "空闲", 2: "升级中", 3: "升级完成", 4: "升级失败",
			}[state]
			if txt == "" {
				txt = fmt.Sprintf("未知(%d)", state)
			}
			fmt.Printf("[D1] Device_State=%s (%d)\n", txt, state)
		}
		// 可选：再打印描述长度/十六进制预览
		if len(fr.Payload) >= 2 {
			sz := int(fr.Payload[1])
			end := 2 + sz
			if end > len(fr.Payload) {
				end = len(fr.Payload)
			}
			desc := fr.Payload[2:end]
			fmt.Printf("[D1] DescSize=%d DescHex=%s\n", sz, strings.ToUpper(hex.EncodeToString(desc)))
		}
	}
}
