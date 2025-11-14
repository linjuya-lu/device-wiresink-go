package frameparser

// 升级帧
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"unicode/utf8"

	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
)

var (
	endByte = 0x96
	CompReg = NewComplementRegistry()
)

// 升级请求报文；
func BuildUpgradeRequest(cmdID string, frameNo byte, filePath string) ([]byte, error) {

	frameType := 0x03
	packetType := 0xB1
	fileType := 0x02
	frameLen := 400

	//文件大小
	totalSize, err := calcTotalSize(filePath)
	if err != nil {
		return nil, fmt.Errorf("calc total size: %w", err)
	}
	//总包数
	totalPackets := (totalSize + frameLen - 1) / frameLen
	if totalPackets == 0 {
		totalPackets = 1
	}
	var buf bytes.Buffer
	// Sync (2)
	buf.WriteByte(byte(syncHi))
	buf.WriteByte(byte(syncLo))
	// Packet_Length (2) 先占位
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	// CMD_ID (17 ASCII, 0 padding)
	writeASCIIFixed(&buf, cmdID, 17)
	// Frame_Type (1)
	buf.WriteByte(byte(frameType))
	// Packet_Type (1)
	buf.WriteByte(byte(packetType))
	// Frame_No (1)
	buf.WriteByte(frameNo)
	// File_Name (32, ASCII, 以 '\0' 结尾，剩余补0)
	writeCStringFixed(&buf, "fireware.hex", 32)
	// File_Type (1) 固定 2
	buf.WriteByte(byte(fileType))
	// Total_Size (4, 大端)
	writeU32BE(&buf, uint32(totalSize))
	// Frame_Len (2, 大端) 固定 1024
	writeU16BE(&buf, uint16(frameLen))
	// Total_Packets (2, 大端)
	writeU16BE(&buf, uint16(totalPackets))
	// 4) CRC16：对 CMD_ID ~ Total_Packets 计算
	payloadForCRC := buf.Bytes()[plenPos+2:] // 跳过 Sync(2) + Packet_Length(2)
	crc := config.CRC16(payloadForCRC)
	writeU16BE(&buf, crc)
	// End (1)
	buf.WriteByte(byte(endByte))
	// 5) 回填 Packet_Length
	pkt := buf.Bytes()
	// 从 CMD_ID 到 End 的总长度（不含前面 4 字节）
	packetLen := uint16(len(pkt) - 4)
	pkt[plenPos] = byte(packetLen >> 8)
	pkt[plenPos+1] = byte(packetLen)

	return pkt, nil
}

func writeU16BE(b *bytes.Buffer, v uint16) {
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

func writeU32BE(b *bytes.Buffer, v uint32) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// ASCII，不足补 0，超过截断
func writeASCIIFixed(b *bytes.Buffer, s string, n int) {
	if len(s) > n {
		b.WriteString(s[:n])
		return
	}
	b.WriteString(s)
	if pad := n - len(s); pad > 0 {
		b.Write(bytes.Repeat([]byte{0x00}, pad))
	}
}

// 写以 '\0' 结尾的 ASCII 字符串，整体固定 n 字节
func writeCStringFixed(b *bytes.Buffer, s string, n int) {
	if n <= 0 {
		return
	}
	// 留一个 '\0'
	max := n - 1
	if len(s) > max {
		b.WriteString(s[:max])
	} else {
		b.WriteString(s)
		b.Write(bytes.Repeat([]byte{0x00}, max-len(s)))
	}
	b.WriteByte(0x00)
}

func calcTotalSize(path string) (int, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return int(fi.Size()), nil
}

// 升级数据报文 (Packet_Type = 0xB2，所有 2B/4B 字段小端序)
func BuildUpgradeDataPacket(cmdID string, frameNo byte, subNo uint16, data []byte) ([]byte, error) {
	const (
		syncHi           = 0x5A
		syncLo           = 0xA5
		frameTypeData    = 0x03
		packetTypeData   = 0xB2
		endByte          = 0x96
		maxPacketDataLen = 400
	)

	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}
	if len(data) > maxPacketDataLen {
		return nil, fmt.Errorf("data too large: %d > %d", len(data), maxPacketDataLen)
	}

	var buf bytes.Buffer

	// 1) Sync

	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)

	// 2) Packet_Length 占位（2B，小端）
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// 3) 固定头/文本字段（无端序）
	writeASCIIFixed(&buf, cmdID, 17)            // CMD_ID (17)
	buf.WriteByte(frameTypeData)                // Frame_Type (1)
	buf.WriteByte(packetTypeData)               // Packet_Type (1)
	buf.WriteByte(frameNo)                      // Frame_No (1)
	writeCStringFixed(&buf, "fireware.hex", 32) // File_Name (32, 若非 C-string 改为 writeASCIIFixed)

	// 4) 数值字段（全部小端）
	writeU16LE(&buf, subNo)             // Subpacket_No (2, LE)
	writeU16LE(&buf, uint16(len(data))) // Packet_Size (2, LE)
	buf.Write(data)                     // Data (N<=400)

	// 5) 回填 Packet_Length（不含 Sync/End，含 CRC16），小端回填
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (plenPos + 2)) + 2) // +2 预留 CRC16 自身
	body[plenPos] = byte(packetLen)                      // low
	body[plenPos+1] = byte(packetLen >> 8)               // high

	// 6) CRC16 覆盖范围：从 Packet_Length 开始到 CRC 前（含 Packet_Length，不含 CRC/End）
	crcRange := body[plenPos:buf.Len()]
	crc := config.CRC16(crcRange)

	// 7) CRC16 小端写入
	writeU16LE(&buf, crc)

	// 8) End
	buf.WriteByte(endByte)

	return buf.Bytes(), nil
}

// 升级结束报文
// UpgradeCompleted 生成“升级结束报文”(Packet_Type 0xB2).
// cmdID:      17字节ASCII（超长截断，不足补0）
// frameNo:    帧号(1B)
// subNo:      子包号(2B, 递增, 大端)
// data:       本子包数据(<=400B)
func UpgradeCompleted(cmdID string, frameNo byte, subNo uint16, data []byte) ([]byte, error) {
	frameTypeData := 0x03
	packetTypeData := 0xB1
	maxPacketDataLen := 400 // Data 最大 400 字节
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}
	if len(data) > maxPacketDataLen {
		return nil, fmt.Errorf("data too large: %d > %d", len(data), maxPacketDataLen)
	}

	var buf bytes.Buffer

	// 1) 固定头
	buf.WriteByte(byte(syncHi)) // Sync
	buf.WriteByte(byte(syncLo))

	// Packet_Length 占位
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// 2) 序号3~10：CMD_ID ~ Data
	writeASCIIFixed(&buf, cmdID, 17)            // CMD_ID (17)
	buf.WriteByte(byte(frameTypeData))          // Frame_Type (1)
	buf.WriteByte(byte(packetTypeData))         // Packet_Type (1)
	buf.WriteByte(frameNo)                      // Frame_No (1)
	writeCStringFixed(&buf, "fireware.hex", 32) // File_Name (32, '\0'结尾)
	writeU16BE(&buf, subNo)                     // Subpacket_No (2)
	writeU16BE(&buf, uint16(len(data)))         // Packet_Size (2)
	buf.Write(data)                             // Data (N<=400)

	// 3) CRC16: 覆盖 CMD_ID ~ Data
	p3to10 := buf.Bytes()[plenPos+2:] // 跳过 Sync+Packet_Length
	crc := config.CRC16(p3to10)
	writeU16BE(&buf, crc) // CRC16 (2)

	// 4) End
	buf.WriteByte(byte(endByte)) // End (1)

	// 5) 回填 Packet_Length = 序号3~11的长度 = (当前长度 - End(1) - 开头4字节)
	packetLen := uint16(buf.Len() - 1 - (plenPos + 2))
	pkt := buf.Bytes()
	pkt[plenPos] = byte(packetLen >> 8)
	pkt[plenPos+1] = byte(packetLen)

	return pkt, nil
}

const (
	endMark = 0x96

	FrameTypeControl = 0x03 // 控制报文
	PacketTypeB1     = 0xB1 // 升级请求
	cmdIDLen         = 17
	fileNameSz       = 32
)

type Endian = binary.ByteOrder

type UpgradeMeta struct {
	EID          string
	FrameNo      byte
	FrameType    byte
	PacketType   byte
	FileName     string
	FileType     byte
	TotalSize    uint32
	FrameLen     uint16
	TotalPackets uint16
	Endian       Endian
}

// 升级请求
// 小端写 16/32 位
func writeU16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))      // low
	buf.WriteByte(byte(v >> 8)) // high
}
func writeU32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))       // b0
	buf.WriteByte(byte(v >> 8))  // b1
	buf.WriteByte(byte(v >> 16)) // b2
	buf.WriteByte(byte(v >> 24)) // b3
}

// UTF-8 按“字节数”安全截断（不截断半个中文/emoji）
func truncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for _, r := range s {
		rl := utf8.RuneLen(r)
		if rl < 0 {
			rl = 1 // 不可识别 rune 按 1 字节写
		}
		if buf.Len()+rl > maxBytes {
			break
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

// 升级请求（所有 2B/4B 字段：低字节在前）
func BuildUpgradeRequestEx(meta UpgradeMeta) ([]byte, error) {
	if meta.EID == "" {
		return nil, errors.New("EID empty")
	}
	if utf8.RuneCountInString(meta.EID) > cmdIDLen {
		return nil, fmt.Errorf("EID too long (> %d)", cmdIDLen)
	}
	if meta.FileName == "" {
		return nil, errors.New("FileName empty")
	}

	// 文件名：取基名，超长则 UTF-8 安全截断（按字节）
	fileName := filepath.Base(meta.FileName)
	const reserveNUL = 1
	maxNameBytes := fileNameSz - reserveNUL
	if maxNameBytes < 0 {
		maxNameBytes = 0
	}
	fileName = truncateUTF8ByBytes(fileName, maxNameBytes)

	if meta.FrameLen == 0 {
		return nil, fmt.Errorf("FrameLen invalid (%d)", meta.FrameLen)
	}
	if meta.TotalPackets == 0 {
		return nil, errors.New("TotalPackets must > 0")
	}
	if meta.FrameType == 0 {
		meta.FrameType = FrameTypeControl // 0x03
	}
	if meta.PacketType == 0 {
		meta.PacketType = PacketTypeB1 // 0xB1
	}

	var buf bytes.Buffer

	// 1) Sync (固定 5A A5)
	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)

	// 2) Packet_Length 占位（2B，小端）
	lenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// 3) 文本/头字段（无端序）
	writeFixedASCII(&buf, meta.EID, cmdIDLen)   // CMD_ID (17)
	buf.WriteByte(meta.FrameType)               // Frame_Type (1)
	buf.WriteByte(meta.PacketType)              // Packet_Type (1)
	buf.WriteByte(meta.FrameNo)                 // Frame_No (1)
	writeFixedASCII(&buf, fileName, fileNameSz) // File_Name (32)，不足补 0

	// 4) 数值字段（全部小端）
	buf.WriteByte(meta.FileType)                // File_Type (1)
	writeU32LE(&buf, meta.TotalSize)            // Total_Size (4, LE)
	writeU16LE(&buf, uint16(meta.FrameLen))     // Frame_Len (2, LE)
	writeU16LE(&buf, uint16(meta.TotalPackets)) // Total_Packets (2, LE)

	// 5) 回填 Packet_Length（从 CMD_ID 到 CRC16，含 CRC，不含 End），小端回填
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (lenPos + 2)) + 2) // +2 预留 CRC 自身
	body[lenPos] = byte(packetLen)                      // low
	body[lenPos+1] = byte(packetLen >> 8)               // high

	// 6) CRC16 计算范围：从 Packet_Length 开始到 CRC 前（含 Packet_Length，不含 CRC/End）
	crcRange := body[lenPos:buf.Len()]
	crc := config.CRC16(crcRange)

	// 7) CRC16 小端写入（低字节在前）
	writeU16LE(&buf, crc)

	// 8) End
	buf.WriteByte(endMark) // 0x96

	return buf.Bytes(), nil
}

// 工具函数
func writeFixedASCII(buf *bytes.Buffer, s string, n int) {
	b := []byte(s)
	if len(b) >= n {
		buf.Write(b[:n])
		return
	}
	buf.Write(b)
	if pad := n - len(b); pad > 0 {
		buf.Write(make([]byte, pad)) // 0 填充
	}
}

type ComplementInfo struct {
	Sum   uint16   // ComplementPack_Sum：未收到的总包数
	Nos   []uint16 // ComplementPack_No：补包号序列（2×N 字节 -> N 个 uint16）
	Acked bool     // 是否已收到 ACK（默认 false）
}

type ComplementRegistry struct {
	mu sync.RWMutex
	m  map[string]ComplementInfo // key: deviceName
}

func NewComplementRegistry() *ComplementRegistry {
	return &ComplementRegistry{m: make(map[string]ComplementInfo)}
}

// 覆盖设置：同时更新 Sum / Nos / Acked
func (r *ComplementRegistry) Set(deviceName string, sum uint16, nos []uint16, acked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 确保 map 已初始化
	if r.m == nil {
		r.m = make(map[string]ComplementInfo)
	}

	n := normalizeNos(nos)

	// 防止调用方后续修改 nos 影响内部
	buf := make([]uint16, len(n))
	copy(buf, n)

	r.m[deviceName] = ComplementInfo{
		Sum:   sum,
		Nos:   buf,
		Acked: acked,
	}
}

// 追加合并（设备可能多次上报缺包号，把新缺包并进来，去重排序）
func (r *ComplementRegistry) Add(deviceName string, moreNos ...uint16) {
	r.mu.Lock()
	info := r.m[deviceName]
	info.Nos = normalizeNos(append(info.Nos, moreNos...))
	info.Sum = uint16(len(info.Nos)) // 以实际缺包数为准
	r.m[deviceName] = info
	r.mu.Unlock()
}

// 读取补包快照：仅当已收到 ACK 时返回数据
func (r *ComplementRegistry) Snapshot(deviceName string) (sum uint16, nos []uint16, ok bool) {
	r.mu.RLock()
	info, exists := r.m[deviceName]
	r.mu.RUnlock()

	if !exists || !info.Acked {
		return 0, nil, false
	}

	out := make([]uint16, len(info.Nos))
	copy(out, info.Nos)
	return info.Sum, out, true
}

// 清空指定设备的补包信息
func (r *ComplementRegistry) Clear(deviceName string) {
	r.mu.Lock()
	delete(r.m, deviceName)
	r.mu.Unlock()
}

// 去重 + 升序
func normalizeNos(nos []uint16) []uint16 {
	if len(nos) == 0 {
		return nil
	}
	m := make(map[uint16]struct{}, len(nos))
	out := make([]uint16, 0, len(nos))
	for _, n := range nos {
		if _, ok := m[n]; !ok {
			m[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// 发送升级结束报文
// 升级结束报文（0xB3），2B 字段一律小端序
func (r *ComplementRegistry) BuildUpgradeEndPacket(eid, fileName string, frameNo byte) ([]byte, error) {
	const (
		syncHi        = 0x5A
		syncLo        = 0xA5
		endByte       = 0x96
		frameTypeCtl  = 0x03
		packetTypeEnd = 0xB3
	)

	writeASCIIFixed := func(buf *bytes.Buffer, s string, n int) {
		b := []byte(s)
		if len(b) >= n {
			buf.Write(b[:n])
			return
		}
		buf.Write(b)
		if pad := n - len(b); pad > 0 {
			buf.Write(make([]byte, pad))
		}
	}
	writeCStringFixed := func(buf *bytes.Buffer, s string, n int) {
		b := []byte(s)
		if len(b) >= n {
			if n > 0 {
				buf.Write(b[:n-1])
				buf.WriteByte(0x00)
			}
			return
		}
		buf.Write(b)
		buf.WriteByte(0x00)
		if pad := n - len(b) - 1; pad > 0 {
			buf.Write(make([]byte, pad))
		}
	}
	// 小端写 16 位
	writeU16LE := func(buf *bytes.Buffer, v uint16) {
		buf.WriteByte(byte(v))      // low
		buf.WriteByte(byte(v >> 8)) // high
	}

	var buf bytes.Buffer

	// 1) Sync
	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)

	// 2) Packet_Length 占位（2B，小端）
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// 3) CMD_ID..File_Name
	writeASCIIFixed(&buf, eid, 17) // CMD_ID (17)
	buf.WriteByte(frameTypeCtl)    // Frame_Type (1)
	buf.WriteByte(packetTypeEnd)   // Packet_Type (1)
	buf.WriteByte(frameNo)         // Frame_No (1)
	base := filepath.Base(fileName)
	writeCStringFixed(&buf, base, 32) // File_Name (32)

	// 4) 先回填 Packet_Length（不含 Sync/End，含 CRC16），小端回填
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (plenPos + 2)) + 2) // +2 预留 CRC 自身
	body[plenPos] = byte(packetLen)                      // low
	body[plenPos+1] = byte(packetLen >> 8)               // high

	// 5) CRC16：从 Packet_Length 起到 CRC 前（含 Packet_Length，不含 CRC/End），小端写入
	crcRange := body[plenPos:buf.Len()]
	crc := config.CRC16(crcRange)
	writeU16LE(&buf, crc)

	// 6) End
	buf.WriteByte(endByte)

	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

//---------------------------------------------------升级解析函数部分---------------------------------------------------

// 升级请求响应报文
type UpgradeResponse struct {
	Sync          uint16   // 0x5AA5
	PacketLength  uint16   // 报文长度（见计算规则）
	CMD_ID        [17]byte // 监测装置 ID
	FrameType     byte
	PacketType    byte // 设备可能用 0xB1/0xD1
	FrameNo       byte
	CommandStatus byte // 0xFF 成功 / 0x00 失败
	CRC16         uint16
	End           byte // 0x96
}

// 升级请求响应解析
func ParseUpgradeResponse(data []byte) (*UpgradeResponse, error) {
	// 表2最短：2+2+17+1+1+1+1+2+1 = 28
	if len(data) < 28 {
		return nil, fmt.Errorf("报文长度不足，只有 %d 字节", len(data))
	}

	// 1) 帧头和长度字节序。设备常见两种：5A A5（大端显示）/ A5 5A（小端显示）
	var lenOrder binary.ByteOrder
	switch {
	case data[0] == 0x5A && data[1] == 0xA5:
		lenOrder = binary.BigEndian
	case data[0] == 0xA5 && data[1] == 0x5A:
		lenOrder = binary.LittleEndian
	default:
		return nil, fmt.Errorf("非法帧头: %02X %02X (期望 5A A5 或 A5 5A)", data[0], data[1])
	}

	resp := &UpgradeResponse{}
	resp.Sync = 0x5AA5
	resp.PacketLength = lenOrder.Uint16(data[2:4])

	offset := 4
	copy(resp.CMD_ID[:], data[offset:offset+17])
	offset += 17
	resp.FrameType = data[offset]
	offset++
	resp.PacketType = data[offset]
	offset++
	resp.FrameNo = data[offset]
	offset++
	resp.CommandStatus = data[offset]
	offset++

	// 末尾 3B：CRC16(2) + End(1)
	if len(data)-offset != 3 {
		// 如需更严格，可根据 PacketLength 再校验一次 layout
	}

	// CRC 一般按大端存；如果失败再尝试小端
	crcBE := binary.BigEndian.Uint16(data[len(data)-3 : len(data)-1])
	// crcLE := binary.LittleEndian.Uint16(data[len(data)-3 : len(data)-1])
	resp.CRC16 = crcBE

	resp.End = data[len(data)-1]
	if resp.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", resp.End)
	}

	// 2) CRC 校验
	// A) 从 Packet_Length 开始（不含 Sync/CRC/End）
	// calcA := CRC16(data[2 : len(data)-3])
	// // B) 从帧头开始（含 Sync，不含 CRC/End）——个别实现会这样
	// calcB := CRC16(data[0 : len(data)-3])

	// if calcA != crcBE && calcB != crcBE && calcA != crcLE && calcB != crcLE {
	// 	return nil, fmt.Errorf("CRC 校验失败: 报文CRC(BE)=0x%04X, (LE)=0x%04X, 计算A=0x%04X, 计算B=0x%04X",
	// 		crcBE, crcLE, calcA, calcB)
	// }

	return resp, nil
}

// 当前升级状态报文
type UpgradeStatus struct {
	Sync            uint16   // 报文头
	PacketLength    uint16   // 报文长度
	CMD_ID          [17]byte // 监测装置ID
	FrameType       byte     // 帧类型
	PacketType      byte     // 报文类型 (0xD1)
	FrameNo         byte     // 帧序号
	DeviceState     byte     // 设备状态 (1-空闲 2-升级中 3-完成 4-失败)
	DescriptionSize byte     // 描述信息长度
	Description     string   // 描述信息 (GB2312 编码，示例中简单按字节转字符串)
	CRC16           uint16   // 校验位
	End             byte     // 报文尾 (0x96)
}

// 升级状态报文解析
func ParseUpgradeStatus(data []byte) (*UpgradeStatus, error) {
	if len(data) < 28 { // 最小长度
		return nil, fmt.Errorf("报文长度不足，只有 %d 字节", len(data))
	}

	status := &UpgradeStatus{}
	status.Sync = binary.BigEndian.Uint16(data[0:2])
	status.PacketLength = binary.BigEndian.Uint16(data[2:4])
	copy(status.CMD_ID[:], data[4:21])
	status.FrameType = data[21]
	status.PacketType = data[22]
	status.FrameNo = data[23]
	status.DeviceState = data[24]
	status.DescriptionSize = data[25]

	// 描述信息解析
	descStart := 26
	descEnd := descStart + int(status.DescriptionSize)
	if descEnd > len(data)-3 { // 预留 CRC16 + End 3字节
		return nil, fmt.Errorf("描述信息长度非法，超出报文范围")
	}
	status.Description = string(data[descStart:descEnd]) // 简单转换（GB2312 -> UTF8 可额外处理）

	// CRC16 和 End
	status.CRC16 = binary.BigEndian.Uint16(data[len(data)-3 : len(data)-1])
	status.End = data[len(data)-1]

	// CRC 校验
	calcCRC := config.CRC16(data[:len(data)-3])
	if calcCRC != status.CRC16 {
		return nil, fmt.Errorf("CRC 校验失败: 报文CRC=0x%X, 计算CRC=0x%X", status.CRC16, calcCRC)
	}

	// End 校验
	if status.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", status.End)
	}

	return status, nil
}

// 补包请求
type ComplementPacket struct {
	Sync              uint16   // 报文头
	PacketLength      uint16   // 报文长度
	CMD_ID            [17]byte // 设备ID
	FrameType         byte     // 帧类型
	PacketType        byte     // 报文类型 (0xB4)
	FrameNo           byte     // 帧序号
	FileName          string   // 文件名 (32字节，以\0结尾)
	ComplementPackSum uint16   // 补包包数
	ComplementPackNo  []uint16 // 补包包号序列
	CRC16             uint16   // 校验
	End               byte     // 报文尾
}

// 补包请求解析
func ParseComplementPacket(data []byte) (*ComplementPacket, error) {
	if len(data) < 60 { // 最小长度(含32字节文件名)
		return nil, fmt.Errorf("报文长度不足，只有 %d 字节", len(data))
	}

	cp := &ComplementPacket{}
	cp.Sync = binary.BigEndian.Uint16(data[0:2])
	cp.PacketLength = binary.BigEndian.Uint16(data[2:4])
	copy(cp.CMD_ID[:], data[4:21])
	cp.FrameType = data[21]
	cp.PacketType = data[22]
	cp.FrameNo = data[23]

	// 文件名 (32字节，以\0结尾)
	fileBytes := data[24:56]
	n := 0
	for ; n < len(fileBytes); n++ {
		if fileBytes[n] == 0 {
			break
		}
	}
	cp.FileName = string(fileBytes[:n])

	// 补包包数
	cp.ComplementPackSum = binary.BigEndian.Uint16(data[56:58])

	// 补包包号序列
	start := 58
	end := len(data) - 3 // 留 CRC16(2) + End(1)
	for i := 0; i < int(cp.ComplementPackSum); i++ {
		idx := start + i*2
		if idx+2 > end {
			return nil, fmt.Errorf("补包包号序列超出报文范围")
		}
		seq := binary.BigEndian.Uint16(data[idx : idx+2])
		cp.ComplementPackNo = append(cp.ComplementPackNo, seq)
	}

	// 报文中的 CRC 和 End
	cp.CRC16 = binary.BigEndian.Uint16(data[end : end+2])
	cp.End = data[end+2]

	// CRC 校验
	calcCRC := config.CRC16(data[0:end]) // 只算到 CRC16 前
	if calcCRC != cp.CRC16 {
		return nil, fmt.Errorf("CRC 校验失败: 报文CRC=0x%X, 计算CRC=0x%X", cp.CRC16, calcCRC)
	}

	// End 校验
	if cp.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", cp.End)
	}

	return cp, nil
}
