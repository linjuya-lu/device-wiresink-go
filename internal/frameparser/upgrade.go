package frameparser

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

const (
	syncHi               byte = 0x5A
	syncLo               byte = 0xA5
	endTag               byte = 0x96
	PktUpgradeResp       byte = 0xB1 //网关响应边代升级请求
	PktUpgradeState      byte = 0xD1 //主动上传当前升级状态
	PktComplementReq     byte = 0xB4 //后台升级补包请求
	DevStateIdle              = 1
	DevStateUpgrading         = 2
	DevStateDone              = 3
	DevStateFailed            = 4
	CommandStatusSuccess      = 0xFF
	CommandStatusFailed       = 0x00
)

var (
	MuReady   sync.RWMutex
	ReadyFlag uint8 //0未就绪；1已就绪；2失败
)

func setReady(ok bool) {
	MuReady.Lock()
	if ok {
		ReadyFlag = 1
	} else {
		ReadyFlag = 0
	}
	MuReady.Unlock()
}

type frame struct {
	SyncHi, SyncLo byte
	PacketLen      uint16
	CmdID          [17]byte
	FrameType      byte
	PacketType     byte
	FrameNo        byte
	Payload        []byte
	CRC16          uint16
	End            byte
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
	// Packet_Length
	var plen uint16
	if be {
		plen = binary.BigEndian.Uint16(b[2:4])
	} else {
		plen = binary.LittleEndian.Uint16(b[2:4])
	}
	// 期望总长&实际载荷长度
	totalExpected := int(2 + 2 + plen + 2 + 1) // 4 + plen + 3
	if len(b) < totalExpected {
		return nil, fmt.Errorf("incomplete frame: have=%d expect=%d (plen=%d)", len(b), totalExpected, plen)
	}
	if b[len(b)-1] != endTag {
		return nil, fmt.Errorf("bad end tag: 0x%02X", b[len(b)-1])
	}
	if 4+17+1+1+1 > len(b)-3 {
		return nil, fmt.Errorf("frame too short for header fields")
	}
	var f frame
	f.SyncHi, f.SyncLo = b[0], b[1]
	f.PacketLen = plen
	copy(f.CmdID[:], b[4:4+17])
	f.FrameType = b[21]
	f.PacketType = b[22]
	f.FrameNo = b[23]
	if 24 <= len(b)-3 {
		f.Payload = b[24 : len(b)-3]
	}
	f.End = endTag
	return &f, nil
}

// 升级请求响应
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

// 升级状态
func HandleUpgradeState(f *frame) {
	if len(f.Payload) < 1 {
		return
	}
	deviceState := f.Payload[0]
	if deviceState == DevStateUpgrading {
		config.SetAck(true)
	} else {
		config.SetAck(false)
	}
}

// 补包请求
func HandleComplementReq(f *frame, deviceName string) {
	if len(f.Payload) < 34 {
		return
	}
	sum := binary.BigEndian.Uint16(f.Payload[32:34])
	raw := f.Payload[34:]
	n := len(raw) / 2
	nos := make([]uint16, 0, n)
	for i := 0; i+1 < len(raw); i += 2 {
		nos = append(nos, binary.BigEndian.Uint16(raw[i:i+2]))
	}
	CompReg.Set(deviceName, sum, nos, true)
}

func PrintFrameBrief(fr *frame) {
	id := strings.TrimRight(string(fr.CmdID[:]), "\x00")
	fmt.Printf("[FRAME] ID=%s FT=0x%02X PT=0x%02X NO=%d payload=%d CRC=0x%04X\n",
		id, fr.FrameType, fr.PacketType, fr.FrameNo, len(fr.Payload), fr.CRC16)
	switch fr.PacketType {
	case PktUpgradeResp:
		var st byte
		if len(fr.Payload) >= 1 {
			st = fr.Payload[len(fr.Payload)-1]
		}
		txt := map[byte]string{0xFF: "成功", 0x00: "失败"}[st]
		if txt == "" {
			txt = fmt.Sprintf("未知(0x%02X)", st)
		}
		fmt.Printf("[B1] Command_Status=%s (0x%02X)\n", txt, st)
	case PktUpgradeState:
		if len(fr.Payload) >= 1 {
			state := fr.Payload[0]
			txt := map[byte]string{
				1: "空闲", 2: "升级中", 3: "升级完成", 4: "升级失败",
			}[state]
			if txt == "" {
				txt = fmt.Sprintf("未知(%d)", state)
			}
			fmt.Printf("[D1] Device_State=%s (%d)\n", txt, state)
		}
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
