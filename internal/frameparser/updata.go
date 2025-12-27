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

// 升级请求报文
func BuildUpgradeRequest(cmdID string, frameNo byte, filePath string) ([]byte, error) {
	frameType := 0x03
	packetType := 0xB1
	fileType := 0x02
	frameLen := 400
	// 文件大小
	totalSize, err := calcTotalSize(filePath)
	if err != nil {
		return nil, fmt.Errorf("calc total size: %w", err)
	}
	// 总包数
	totalPackets := (totalSize + frameLen - 1) / frameLen
	if totalPackets == 0 {
		totalPackets = 1
	}
	var buf bytes.Buffer
	buf.WriteByte(byte(syncHi))
	buf.WriteByte(byte(syncLo))
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	writeASCIIFixed(&buf, cmdID, 17)
	buf.WriteByte(byte(frameType))
	buf.WriteByte(byte(packetType))
	buf.WriteByte(frameNo)
	writeCStringFixed(&buf, "fireware.hex", 32)
	buf.WriteByte(byte(fileType))
	writeU32BE(&buf, uint32(totalSize))
	writeU16BE(&buf, uint16(frameLen))
	writeU16BE(&buf, uint16(totalPackets))
	payloadForCRC := buf.Bytes()[plenPos+2:] //跳过Sync+Packet_Length
	crc := config.CRC16(payloadForCRC)
	writeU16BE(&buf, crc)
	buf.WriteByte(byte(endByte))
	pkt := buf.Bytes()
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

func writeCStringFixed(b *bytes.Buffer, s string, n int) {
	if n <= 0 {
		return
	}
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
	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	writeASCIIFixed(&buf, cmdID, 17)
	buf.WriteByte(frameTypeData)
	buf.WriteByte(packetTypeData)
	buf.WriteByte(frameNo)
	writeCStringFixed(&buf, "fireware.hex", 32)
	// 数值字段
	writeU16LE(&buf, subNo)
	writeU16LE(&buf, uint16(len(data)))
	buf.Write(data)
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (plenPos + 2)) + 2)
	body[plenPos] = byte(packetLen)
	body[plenPos+1] = byte(packetLen >> 8)
	// CRC16覆盖范围：从Packet_Length开始到CRC前
	crcRange := body[plenPos:buf.Len()]
	crc := config.CRC16(crcRange)
	writeU16LE(&buf, crc)
	buf.WriteByte(endByte)
	return buf.Bytes(), nil
}

// 升级结束报文
func UpgradeCompleted(cmdID string, frameNo byte, subNo uint16, data []byte) ([]byte, error) {
	frameTypeData := 0x03
	packetTypeData := 0xB1
	maxPacketDataLen := 400
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}
	if len(data) > maxPacketDataLen {
		return nil, fmt.Errorf("data too large: %d > %d", len(data), maxPacketDataLen)
	}
	var buf bytes.Buffer
	buf.WriteByte(byte(syncHi))
	buf.WriteByte(byte(syncLo))
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	writeASCIIFixed(&buf, cmdID, 17)
	buf.WriteByte(byte(frameTypeData))
	buf.WriteByte(byte(packetTypeData))
	buf.WriteByte(frameNo)
	writeCStringFixed(&buf, "fireware.hex", 32)
	writeU16BE(&buf, subNo)
	writeU16BE(&buf, uint16(len(data)))
	buf.Write(data)
	p3to10 := buf.Bytes()[plenPos+2:]
	crc := config.CRC16(p3to10)
	writeU16BE(&buf, crc)
	buf.WriteByte(byte(endByte))
	packetLen := uint16(buf.Len() - 1 - (plenPos + 2))
	pkt := buf.Bytes()
	pkt[plenPos] = byte(packetLen >> 8)
	pkt[plenPos+1] = byte(packetLen)
	return pkt, nil
}

const (
	endMark          = 0x96
	FrameTypeControl = 0x03 //控制报文
	PacketTypeB1     = 0xB1 //升级请求
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
func writeU16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
}
func writeU32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

func truncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for _, r := range s {
		rl := utf8.RuneLen(r)
		if rl < 0 {
			rl = 1
		}
		if buf.Len()+rl > maxBytes {
			break
		}
		buf.WriteRune(r)
	}
	return buf.String()
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
	// 文件名
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
		meta.FrameType = FrameTypeControl //0x03
	}
	if meta.PacketType == 0 {
		meta.PacketType = PacketTypeB1 //0xB1
	}
	var buf bytes.Buffer
	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)
	lenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	writeFixedASCII(&buf, meta.EID, cmdIDLen)
	buf.WriteByte(meta.FrameType)
	buf.WriteByte(meta.PacketType)
	buf.WriteByte(meta.FrameNo)
	writeFixedASCII(&buf, fileName, fileNameSz)
	// 数值字段
	buf.WriteByte(meta.FileType)
	writeU32LE(&buf, meta.TotalSize)
	writeU16LE(&buf, uint16(meta.FrameLen))
	writeU16LE(&buf, uint16(meta.TotalPackets))
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (lenPos + 2)) + 2)
	body[lenPos] = byte(packetLen)
	body[lenPos+1] = byte(packetLen >> 8)
	crcRange := body[lenPos:buf.Len()]
	crc := config.CRC16(crcRange)
	writeU16LE(&buf, crc)
	buf.WriteByte(endMark)
	return buf.Bytes(), nil
}

func writeFixedASCII(buf *bytes.Buffer, s string, n int) {
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

type ComplementInfo struct {
	Sum   uint16
	Nos   []uint16
	Acked bool
}

type ComplementRegistry struct {
	mu sync.RWMutex
	m  map[string]ComplementInfo
}

func NewComplementRegistry() *ComplementRegistry {
	return &ComplementRegistry{m: make(map[string]ComplementInfo)}
}

func (r *ComplementRegistry) Set(deviceName string, sum uint16, nos []uint16, acked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = make(map[string]ComplementInfo)
	}
	n := normalizeNos(nos)
	buf := make([]uint16, len(n))
	copy(buf, n)
	r.m[deviceName] = ComplementInfo{
		Sum:   sum,
		Nos:   buf,
		Acked: acked,
	}
}

func (r *ComplementRegistry) Add(deviceName string, moreNos ...uint16) {
	r.mu.Lock()
	info := r.m[deviceName]
	info.Nos = normalizeNos(append(info.Nos, moreNos...))
	info.Sum = uint16(len(info.Nos))
	r.m[deviceName] = info
	r.mu.Unlock()
}

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

func (r *ComplementRegistry) Clear(deviceName string) {
	r.mu.Lock()
	delete(r.m, deviceName)
	r.mu.Unlock()
}

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
	writeU16LE := func(buf *bytes.Buffer, v uint16) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
	}
	var buf bytes.Buffer
	buf.WriteByte(syncLo)
	buf.WriteByte(syncHi)
	plenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})
	writeASCIIFixed(&buf, eid, 17)
	buf.WriteByte(frameTypeCtl)
	buf.WriteByte(packetTypeEnd)
	buf.WriteByte(frameNo)
	base := filepath.Base(fileName)
	writeCStringFixed(&buf, base, 32)
	body := buf.Bytes()
	packetLen := uint16((buf.Len() - (plenPos + 2)) + 2)
	body[plenPos] = byte(packetLen)
	body[plenPos+1] = byte(packetLen >> 8)
	crcRange := body[plenPos:buf.Len()]
	crc := config.CRC16(crcRange)
	writeU16LE(&buf, crc)
	buf.WriteByte(endByte)
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// 升级解析函数部分
type UpgradeResponse struct {
	Sync          uint16   //0x5AA5
	PacketLength  uint16   //报文长度
	CMD_ID        [17]byte //监测装置ID
	FrameType     byte
	PacketType    byte
	FrameNo       byte
	CommandStatus byte //0xFF成功/0x00失败
	CRC16         uint16
	End           byte //0x96
}

// 升级请求响应解析
func ParseUpgradeResponse(data []byte) (*UpgradeResponse, error) {
	if len(data) < 28 {
		return nil, fmt.Errorf("报文长度不足，只有 %d 字节", len(data))
	}
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
	if len(data)-offset != 3 {
	}
	crcBE := binary.BigEndian.Uint16(data[len(data)-3 : len(data)-1])
	resp.CRC16 = crcBE
	resp.End = data[len(data)-1]
	if resp.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", resp.End)
	}
	return resp, nil
}

// 当前升级状态报文
type UpgradeStatus struct {
	Sync            uint16   //报文头
	PacketLength    uint16   //报文长度
	CMD_ID          [17]byte //监测装置ID
	FrameType       byte     //帧类型
	PacketType      byte     //报文类型 (0xD1)
	FrameNo         byte     //帧序号
	DeviceState     byte     //设备状态 (1-空闲 2-升级中 3-完成 4-失败)
	DescriptionSize byte     //描述信息长度
	Description     string   //描述信息 (GB2312 编码，示例中简单按字节转字符串)
	CRC16           uint16   //校验位
	End             byte     //报文尾 (0x96)
}

// 升级状态报文解析
func ParseUpgradeStatus(data []byte) (*UpgradeStatus, error) {
	if len(data) < 28 {
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
	if descEnd > len(data)-3 {
		return nil, fmt.Errorf("描述信息长度非法，超出报文范围")
	}
	status.Description = string(data[descStart:descEnd])
	status.CRC16 = binary.BigEndian.Uint16(data[len(data)-3 : len(data)-1])
	status.End = data[len(data)-1]
	calcCRC := config.CRC16(data[:len(data)-3])
	if calcCRC != status.CRC16 {
		return nil, fmt.Errorf("CRC 校验失败: 报文CRC=0x%X, 计算CRC=0x%X", status.CRC16, calcCRC)
	}
	if status.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", status.End)
	}
	return status, nil
}

// 补包请求
type ComplementPacket struct {
	Sync              uint16   //报文头
	PacketLength      uint16   //报文长度
	CMD_ID            [17]byte //设备ID
	FrameType         byte     //帧类型
	PacketType        byte     //报文类型 (0xB4)
	FrameNo           byte     //帧序号
	FileName          string   //文件名 (32字节，以\0结尾)
	ComplementPackSum uint16   //补包包数
	ComplementPackNo  []uint16 //补包包号序列
	CRC16             uint16   //校验
	End               byte     //报文尾
}

// 补包请求解析
func ParseComplementPacket(data []byte) (*ComplementPacket, error) {
	if len(data) < 60 {
		return nil, fmt.Errorf("报文长度不足，只有 %d 字节", len(data))
	}
	cp := &ComplementPacket{}
	cp.Sync = binary.BigEndian.Uint16(data[0:2])
	cp.PacketLength = binary.BigEndian.Uint16(data[2:4])
	copy(cp.CMD_ID[:], data[4:21])
	cp.FrameType = data[21]
	cp.PacketType = data[22]
	cp.FrameNo = data[23]
	// 文件名
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
	end := len(data) - 3
	for i := 0; i < int(cp.ComplementPackSum); i++ {
		idx := start + i*2
		if idx+2 > end {
			return nil, fmt.Errorf("补包包号序列超出报文范围")
		}
		seq := binary.BigEndian.Uint16(data[idx : idx+2])
		cp.ComplementPackNo = append(cp.ComplementPackNo, seq)
	}
	cp.CRC16 = binary.BigEndian.Uint16(data[end : end+2])
	cp.End = data[end+2]
	calcCRC := config.CRC16(data[0:end])
	if calcCRC != cp.CRC16 {
		return nil, fmt.Errorf("CRC 校验失败: 报文CRC=0x%X, 计算CRC=0x%X", cp.CRC16, calcCRC)
	}
	if cp.End != 0x96 {
		return nil, fmt.Errorf("报文尾错误: 期望0x96, 实际0x%X", cp.End)
	}
	return cp, nil
}
