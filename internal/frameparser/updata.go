package frameparser

// 升级帧
import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"unicode/utf8"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

var endByte = 0x96
var CompReg = NewComplementRegistry()

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
	//开始拼装
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
	crc := CRC16(payloadForCRC)
	writeU16BE(&buf, crc)
	// End (1)
	buf.WriteByte(byte(endByte))
	// 5) 回填 Packet_Length
	pkt := buf.Bytes()
	// 定义：从 CMD_ID 到 End 的总长度（不含前面 4 字节）
	packetLen := uint16(len(pkt) - 4)
	// 如果你们要“全包长度”，用下面这行：
	// packetLen := uint16(len(pkt))

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

// 升级数据报文
func BuildUpgradeDataPacket(cmdID string, frameNo byte, subNo uint16, data []byte) ([]byte, error) {
	syncHi := 0x5A
	syncLo := 0xA5
	frameTypeData := 0x03
	packetTypeData := 0xB2

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
	crc := CRC16(p3to10)
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
	crc := CRC16(p3to10)
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
	syncHi  = 0x5A
	syncLo  = 0xA5
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
	fileName := filepath.Base(meta.FileName)
	if utf8.RuneCountInString(fileName) > fileNameSz-1 {
		return nil, fmt.Errorf("FileName too long (> %d)", fileNameSz-1)
	}
	if meta.FrameLen == 0 || meta.FrameLen > 400 {
		return nil, fmt.Errorf("FrameLen invalid (%d)", meta.FrameLen)
	}
	if meta.TotalPackets == 0 {
		return nil, errors.New("TotalPackets must > 0")
	}
	if meta.Endian == nil {
		meta.Endian = binary.BigEndian
	}
	if meta.FrameType == 0 {
		meta.FrameType = FrameTypeControl
	}
	if meta.PacketType == 0 {
		meta.PacketType = PacketTypeB1
	}

	var buf bytes.Buffer
	buf.WriteByte(syncHi)
	buf.WriteByte(syncLo)

	lenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00}) // Packet_Length 占位

	writeFixedASCII(&buf, meta.EID, cmdIDLen) // CMD_ID(17)
	buf.WriteByte(meta.FrameType)             // Frame_Type
	buf.WriteByte(meta.PacketType)            // Packet_Type
	buf.WriteByte(meta.FrameNo)               // Frame_No
	writeFixedASCII(&buf, fileName, fileNameSz)
	buf.WriteByte(meta.FileType)
	writeU16(&buf, meta.Endian, meta.FrameLen)
	writeU16(&buf, meta.Endian, meta.TotalPackets)
	writeU32(&buf, meta.Endian, meta.TotalSize)

	pkt := buf.Bytes()

	crcVal := CRC16(pkt[lenPos:buf.Len()])
	writeU16(&buf, meta.Endian, crcVal)

	buf.WriteByte(endMark)

	final := buf.Bytes()
	meta.Endian.PutUint16(final[lenPos:lenPos+2], uint16(len(final)))
	return final, nil
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
func writeU16(buf *bytes.Buffer, bo binary.ByteOrder, v uint16) {
	var t [2]byte
	bo.PutUint16(t[:], v)
	buf.Write(t[:])
}
func writeU32(buf *bytes.Buffer, bo binary.ByteOrder, v uint32) {
	var t [4]byte
	bo.PutUint32(t[:], v)
	buf.Write(t[:])
}

type ComplementInfo struct {
	Sum uint16   // ComplementPack_Sum：未收到的总包数
	Nos []uint16 // ComplementPack_No：补包号序列（2×N 字节 -> N 个 uint16）
}

type ComplementRegistry struct {
	mu sync.RWMutex
	m  map[string]ComplementInfo // key: deviceName
}

func NewComplementRegistry() *ComplementRegistry {
	return &ComplementRegistry{m: make(map[string]ComplementInfo)}
}

// 覆盖设置（第一次收到 B4 或主动以设备端上报的 Sum/No 为准）
func (r *ComplementRegistry) Set(deviceName string, sum uint16, nos []uint16) {
	r.mu.Lock()
	r.m[deviceName] = ComplementInfo{
		Sum: sum,
		Nos: normalizeNos(nos),
	}
	r.mu.Unlock()
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

// 读取补包快照
func (r *ComplementRegistry) Snapshot(deviceName string) (sum uint16, nos []uint16, ok bool) {
	r.mu.RLock()
	info, ok := r.m[deviceName]
	r.mu.RUnlock()
	if !ok {
		return 0, nil, false
	}
	return info.Sum, append([]uint16(nil), info.Nos...), true
}

// 清空指定设备的补包信息（补包完成后调用）
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
func (r *ComplementRegistry) SendEndAndConfirm(ctx context.Context, fileName string, frameNo byte) error {
	const (
		syncHi        = 0x5A
		syncLo        = 0xA5
		endByte       = 0x96
		frameTypeCtl  = 0x03 // 见表：Frame_Type(0x03)
		packetTypeEnd = 0xB3 // 升级结束报文
	)

	// ——小工具：若你已有同名函数，可删掉这些内联实现——
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
			// 至少保留 '\0'
			if n > 0 {
				buf.Write(b[:n-1])
				buf.WriteByte(0x00)
			}
			return
		}
		buf.Write(b)
		buf.WriteByte(0x00) // C-string 结尾
		if pad := n - len(b) - 1; pad > 0 {
			buf.Write(make([]byte, pad))
		}
	}
	writeU16BE := func(buf *bytes.Buffer, v uint16) {
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v))
	}

	var buf bytes.Buffer

	// 1) Sync
	buf.WriteByte(byte(syncHi))
	buf.WriteByte(byte(syncLo))

	// 2) Packet_Length（占位）
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// 3) CMD_ID（17，ASCII，右侧 0 填充）
	writeASCIIFixed(&buf, config.EidStr, 17)

	// 4) Frame_Type（1）
	buf.WriteByte(byte(frameTypeCtl))

	// 5) Packet_Type（1）= 0xB3
	buf.WriteByte(byte(packetTypeEnd))

	// 6) Frame_No（1）
	buf.WriteByte(frameNo)

	// 7) File_Name（32，ASCII，以 '\0' 结尾，右侧补 0）
	base := filepath.Base(fileName) // 只发文件名
	writeCStringFixed(&buf, base, 32)

	// 8) CRC16（2）——覆盖范围：从 CMD_ID 开始到 File_Name 结束
	body := buf.Bytes()[plenPos+2:]
	crc := CRC16(body) // 直接复用你已有的查表 CRC16
	writeU16BE(&buf, crc)

	// 9) End（1）
	buf.WriteByte(byte(endByte))

	// 回填 Packet_Length = 序号3~8的长度（不含 Sync/End；含 CRC16）
	packetLen := uint16(buf.Len() - 1 - (plenPos + 2))
	pkt := buf.Bytes()
	pkt[plenPos] = byte(packetLen >> 8)
	pkt[plenPos+1] = byte(packetLen)

	// 发送
	relay.SendFrame(config.EidStr, pkt)
	return nil
}
