package driver

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/frameparser"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

func (d *WireSinkDriver) handleTimeParameterSet(deviceName string) error {
	d.lc.Infof("开始处理时间设置命令: %s", deviceName)
	// 获取 EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	// 构建复位帧
	loc := time.FixedZone("UTC-0", 0) // UTC
	ts := uint32(time.Now().In(loc).Unix())

	// 发送帧
	reqFrame, _ := frameparser.BuildTimeParamFrame(sensorID, 1, ts)

	// 发送命令
	eidStr, _ = eidValue.(string)
	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("已发送时间设置命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleResetCommand(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	// 构建复位帧
	reqFrame, _ := frameparser.BuildResetRequest(sensorID)
	// 发送命令
	eidStr, _ = eidValue.(string)

	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleTimeParameterQuery(deviceName string) error {
	d.lc.Infof("开始处理时间参数查询命令: %s", deviceName)

	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	// 构建复位帧
	reqFrame, _ := frameparser.BuildTimeParamFrame(sensorID, 0, 0)
	// 发送命令
	eidStr, _ = eidValue.(string)
	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("已发送时间参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdQuery(deviceName string) error {
	d.lc.Infof("开始处理EID查询命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建ID查询帧
	frame, err := frameparser.BuildSensorIDFrame(sensorID, 0, [6]byte{})
	if err != nil {
		return fmt.Errorf("构造传感器ID查询帧失败: %w", err)
	}
	//发送命令
	eidStr, _ = eidValue.(string)
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("已发送EID查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdMoniDataQuery(deviceName string) error {
	d.lc.Infof("开始处理检测数据查询命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建ID查询帧
	frame, err := frameparser.BuildMonitoringDataQueryFrame(sensorID)
	if err != nil {
		return fmt.Errorf("构造全部通用参数查询失败: %w", err)
	}
	eidStr, _ = eidValue.(string)
	//发送命令
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("已发送检测数据查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdAlarmParaQuery(deviceName string) error {
	d.lc.Infof("开始处理告警参数查询命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	//解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	// 构建ID查询帧
	frame, err := frameparser.BuildAlarmParameterQueryFrame(sensorID)
	if err != nil {
		return fmt.Errorf("构造q全部通用参数查询失败: %w", err)
	}
	// 发送命令
	eidStr, _ = eidValue.(string)
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("已发送告警参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleGeneParaQuery(deviceName string) error {
	d.lc.Infof("开始处理通用参数查询命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidStr := "238A0841D828"
	//解码成 6 字节
	eidBytes, err := hex.DecodeString(eidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", eidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建ID查询帧
	frame, err := frameparser.BuildParameterQueryFrame(sensorID)
	if err != nil {
		return fmt.Errorf("构造q全部通用参数查询失败: %w", err)
	}
	//发送命令
	eidStr, _ = eidValue.(string)
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("已发送通用参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleRouterParameterQuery(deviceName string) error {
	d.lc.Infof("开始处拓扑查询命令: %s", deviceName)
	// 获取EID
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	// 解码成 6 字节
	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建ID查询帧
	frame, err := frameparser.BuildGeneralParamQueryFrame(sensorID, 0x0800)
	if err != nil {
		return fmt.Errorf("构造拓扑查询失败: %w", err)
	}
	eidStr, _ := eidValue.(string)
	//发送命令
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("已发送拓扑查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

var (
	mu2       sync.RWMutex
	readyFlag int
)

func (d *WireSinkDriver) handleUpgradeQuery(deviceName string) error {
	mu2.Lock()
	readyFlag = 0
	mu2.Unlock()
	d.lc.Infof("开始处拓扑查询命令: %s", deviceName)

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//构建帧
	// 升级文件路径
	filePath := "./file/fireware.hex"
	// 帧号
	var frameNo byte = 1

	// 生成“升级请求报文”
	pkt, err := frameparser.BuildUpgradeRequest(config.EidStr, frameNo, filePath)
	if err != nil {
		fmt.Println("BuildUpgradeRequest error:", err)
		return err
	}
	//发送命令
	relay.SendFrame(config.EidStr, pkt)
	// 等待 readyFlag 变 1，然后退出循环，但不 return
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu2.RLock()
		ready := (readyFlag == 1)
		mu2.RUnlock()
		if ready {
			break // 只结束循环，继续往下执行
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待 readyFlag 超时")
		}
		time.Sleep(10 * time.Millisecond) // 防止空转占满CPU
	}

	// 准备就绪，开始升级
	d.lc.Infof("已就绪，开始升级...")
	// 3) 读取固件并切分为 <=400B 分片
	fw, err := readFirmwareBytes(filePath)
	if err != nil {
		return fmt.Errorf("读取固件失败: %w", err)
	}
	chunks := split400(fw)
	if len(chunks) == 0 {
		return fmt.Errorf("固件为空")
	}

	// 4) 循环封包并发送（Subpacket_No 从 1 递增）
	for i, chunk := range chunks {
		subNo := uint16(i + 1)

		pkt, err := frameparser.BuildUpgradeDataPacket(config.EidStr, frameNo, subNo, chunk)
		if err != nil {
			return fmt.Errorf("BuildUpgradeDataPacket sub=%d 失败: %w", subNo, err)
		}

		relay.SendFrame(config.EidStr, pkt)

		// ——可选：等待每片 ACK——
		// 如果你有 per-chunk 的 ACK 机制，这里等一下（带超时）
		// ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		// waitAck(subNo, ctx) // 自己实现：收到某处回调时标记 subNo 已确认
		// cancel()
	}

	d.lc.Infof("全部数据包已发送：总片数=%d", len(chunks))
	return nil
}

// 读取固件：如果是 .hex 文本（纯 HEX），会解码成字节；否则直接读二进制
func readFirmwareBytes(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".hex") {
		// 去空白/分隔符，去前缀0x，奇数长度左补0
		s := strings.TrimSpace(string(b))
		var sb strings.Builder
		sb.Grow(len(s))
		for _, r := range s {
			switch r {
			case ' ', '\t', '\n', '\r', ',', ';', ':', '-':
				// 跳过常见分隔
			default:
				sb.WriteRune(r)
			}
		}
		hexStr := sb.String()
		if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
			hexStr = hexStr[2:]
		}
		if len(hexStr)%2 != 0 {
			hexStr = "0" + hexStr
		}
		return hex.DecodeString(hexStr)
	}
	return b, nil
}

// 把字节切成 <=400B 的分片
func split400(b []byte) [][]byte {
	if len(b) == 0 {
		return nil
	}
	out := make([][]byte, 0, (len(b)+399)/400)
	for i := 0; i < len(b); i += 400 {
		j := i + 400
		if j > len(b) {
			j = len(b)
		}
		out = append(out, b[i:j])
	}
	return out
}
