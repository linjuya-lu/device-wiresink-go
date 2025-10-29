package frameparser

// 升级帧
import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 升级请求报文；
// cmdID: 设备ID（ASCII，不足补0，超过截断）；
// frameNo: 帧序（无符号 1B）；
// filePath: 固件文件路径；
//
// 返回：完整报文字节流
func BuildUpgradeRequest(cmdID string, frameNo byte, filePath string) ([]byte, error) {

	syncHi := 0x5A
	syncLo := 0xA5
	frameType := 0x03
	packetType := 0xB1
	fileType := 0x02
	frameLen := 1024 // 0x0400
	endByte := 0x96

	// 1) 计算文件大小
	totalSize, err := calcTotalSize(filePath)
	if err != nil {
		return nil, fmt.Errorf("calc total size: %w", err)
	}
	// 2) 计算总包数
	totalPackets := (totalSize + frameLen - 1) / frameLen
	if totalPackets == 0 {
		totalPackets = 1
	}
	// 3) 开始拼装
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

// 计算大小：.hex 文本按十六进制内容换算为字节数；其他按文件真实字节数；
func calcTotalSize(path string) (int, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".hex" {
		return int(fi.Size()), nil
	}

	// .hex 文本：去空白/分隔符，按 HEX 解析长度；
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return 0, err
	}
	clean := normalizeHexForSize(string(content))
	if len(clean) == 0 {
		return 0, errors.New("empty hex file")
	}
	if len(clean)%2 != 0 {
		// 宽容：左补 0
		clean = "0" + clean
	}
	// 验证是否合法 HEX
	if _, err := hex.DecodeString(clean[:min(64, len(clean))]); err != nil {
		// 如果文件是 Intel HEX 格式（:开头的行），此处就不是纯 HEX。
		// 这种情况你应先把 Intel HEX 转 BIN 再发。这里直接退回“按文本字节数”。
		return int(fi.Size()), nil
	}
	return len(clean) / 2, nil
}

func normalizeHexForSize(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			b.WriteRune(r)
		}
		// 其他字符（空白/逗号/冒号/连字符等）忽略
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BuildUpgradeDataPacket 生成“升级数据报文”(Packet_Type 0xB2).
// cmdID:      17字节ASCII（超长截断，不足补0）
// frameNo:    帧号(1B)
// subNo:      子包号(2B, 递增, 大端)
// data:       本子包数据(<=400B)
func BuildUpgradeDataPacket(cmdID string, frameNo byte, subNo uint16, data []byte) ([]byte, error) {
	syncHi := 0x5A
	syncLo := 0xA5
	frameTypeData := 0x03
	packetTypeData := 0xB2
	endByte := 0x96
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

//升级结束报文
