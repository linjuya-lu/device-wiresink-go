package driver

import (
	"context"
	"encoding/binary"
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

var (
	mu2       sync.RWMutex
	readyFlag int
)

type UpgradeProgress struct {
	Device string
	Stage  string
	Err    error
}

func (d *WireSinkDriver) handleTimeParameterSet(deviceName string) error {
	d.lc.Infof("时间设置: %s", deviceName)
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("时间设置 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("时间设置 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("时间设置 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	//时间
	loc := time.FixedZone("UTC-0", 0)
	ts := uint32(time.Now().In(loc).Unix())

	reqFrame, _ := frameparser.BuildTimeParamFrame(sensorID, 1, ts)

	eidStr, _ := eidValue.(string)
	relay.SendFrame("sink", eidStr, reqFrame)
	d.lc.Infof("时间设置 已发送时间设置命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleResetCommand(deviceName string) error {
	d.lc.Infof("复位命令: %s", deviceName)
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("复位命令 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("复位命令 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("复位命令 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	reqFrame, _ := frameparser.BuildResetRequest(sensorID)
	eidStr, _ := eidValue.(string)

	relay.SendFrame("sink", eidStr, reqFrame)
	d.lc.Infof("复位命令 已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleTimeParameterQuery(deviceName string) error {
	d.lc.Infof("时间参数查询: %s", deviceName)

	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("时间参数查询 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("时间参数查询 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("时间参数查询 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	reqFrame, _ := frameparser.BuildTimeParamFrame(sensorID, 0, 0)
	eidStr, _ := eidValue.(string)
	relay.SendFrame("sink", eidStr, reqFrame)
	d.lc.Infof("时间参数查询 已发送时间参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdQuery(deviceName string) error {
	d.lc.Infof("EID查询命令: %s", deviceName)

	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("EID查询命令 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("EID查询命令 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("EID查询命令 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	frame, err := frameparser.BuildSensorIDFrame(sensorID, 0, [6]byte{})
	if err != nil {
		return fmt.Errorf("EID查询命令 构造传感器ID查询帧失败: %w", err)
	}
	//发送命令
	eidStr, _ := eidValue.(string)
	relay.SendFrame("sink", eidStr, frame)
	d.lc.Infof("EID查询命令 已发送EID查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdMoniDataQuery(deviceName string) error {
	d.lc.Infof("检测数据查询: %s", deviceName)
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("检测数据查询 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("检测数据查询 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("检测数据查询 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)
	frame, err := frameparser.BuildMonitoringDataQueryFrame(sensorID)
	if err != nil {
		return fmt.Errorf("检测数据查询 构造全部通用参数查询失败: %w", err)
	}
	eidStr, _ := eidValue.(string)
	//发送命令
	relay.SendFrame("sink", eidStr, frame)
	d.lc.Infof("检测数据查询 已发送检测数据查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdAlarmParaQuery(deviceName string) error {
	d.lc.Infof("告警参数查询: %s", deviceName)
	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("告警参数查询 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("告警参数查询 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("告警参数查询 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)

	frame, err := frameparser.BuildAlarmParameterQueryFrame(sensorID)
	if err != nil {
		return fmt.Errorf("告警参数查询 构造q全部通用参数查询失败: %w", err)
	}

	eidStr, _ := eidValue.(string)
	relay.SendFrame("sink", eidStr, frame)
	d.lc.Infof("告警参数查询 已发送告警参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleRouterParameterQuery(deviceName string) error {
	d.lc.Infof("拓扑查询: %s", deviceName)

	eidValue, ok := config.GetDeviceValue(deviceName, "eid")
	if !ok {
		err := fmt.Errorf("拓扑查询 设备 %s 的 EID 未初始化", deviceName)
		d.lc.Error(err.Error())
		return err
	}

	eidBytes, err := hex.DecodeString(config.EidStr)
	if err != nil {
		err = fmt.Errorf("拓扑查询 EID[%s] 转十六进制失败: %w", config.EidStr, err)
		d.lc.Error(err.Error())
		return err
	}
	if len(eidBytes) != 6 {
		err = fmt.Errorf("拓扑查询 EID 长度不对，期望 6 字节，实际 %d 字节", len(eidBytes))
		d.lc.Error(err.Error())
		return err
	}
	var sensorID [6]byte
	copy(sensorID[:], eidBytes)

	frame, err := frameparser.BuildGeneralParamQueryFrame(sensorID, 0x0800)
	if err != nil {
		return fmt.Errorf("拓扑查询 构造拓扑查询失败: %w", err)
	}
	eidStr, _ := eidValue.(string)
	relay.SendFrame("sink", eidStr, frame)
	d.lc.Infof("已发送拓扑查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

// 读取固件
func readFirmwareBytes(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".hpk", ".bin":
		return b, nil
	default:
		return b, nil
	}
}

// 异步升级
func (d *WireSinkDriver) startUpgradeAsync(deviceName string) error {
	d.upgMu.Lock()
	if _, exists := d.upgrading[deviceName]; exists {
		d.upgMu.Unlock()
		d.lc.Warnf("已经开始升级 %s", deviceName)
		return nil
	}

	// 30分钟超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	d.upgrading[deviceName] = cancel
	d.upgMu.Unlock()

	go func() {
		defer func() {
			d.upgMu.Lock()
			delete(d.upgrading, deviceName)
			d.upgMu.Unlock()
		}()

		d.report(deviceName, "start", nil)
		if err := d.handleUpgradeQuery(ctx, deviceName); err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("upgrade %s failed: %v", deviceName, err)
			return
		}
		d.report(deviceName, "done", nil)
		d.lc.Infof("upgrade %s finished", deviceName)
	}()

	return nil
}

// 上报状态
func (d *WireSinkDriver) report(dev, stage string, err error) {
	select {
	case d.progCh <- UpgradeProgress{Device: dev, Stage: stage, Err: err}:
	default:
		// 丢弃或扩容通道
	}
}

// 轮询等待全局 ACK==1，支持 ctx 取消 & 超时
func waitAck(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(10 * time.Millisecond) // 轮询间隔
	timer := time.NewTimer(timeout)
	defer func() {
		ticker.Stop()
		timer.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("等待ACK超时(%s)", timeout)
		case <-ticker.C:
			if config.GetAck() == 1 {
				return nil
			}
		}
	}
}

// 升级流程
func (d *WireSinkDriver) handleUpgradeQuery(ctx context.Context, deviceName string) error {
	d.lc.Infof("开始处理升级命令: %s", deviceName)

	// EID转化
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

	// 固件读取
	filePath := "./file/HJWG-20250731.hpk"
	fw, err := readFirmwareBytes(filePath)
	if err != nil {
		return fmt.Errorf("读取固件失败: %w", err)
	}
	if len(fw) == 0 {
		return fmt.Errorf("固件为空")
	}

	// 切片（<=400B） & 元数据
	const frameLen = 400 // 单帧最大数据区
	chunks := split400(fw)
	totalPackets := uint16((len(fw) + frameLen - 1) / frameLen)
	if totalPackets == 0 {
		totalPackets = 1
	}
	fileName := filepath.Base(filePath)
	var frameNo byte = 0 // 包序列号

	//升级请求
	meta := frameparser.UpgradeMeta{
		// EID:          config.EidStr,
		EID:          "HY_HJWG_202500002",
		FrameNo:      frameNo,
		FrameType:    frameparser.FrameTypeControl, // 0x03
		PacketType:   frameparser.PacketTypeB1,     // 0xB1
		FileName:     fileName,                     // 会在构造里做 32 字节填充
		FileType:     1,                            // 文件类型
		TotalSize:    uint32(len(fw)),              // 字节
		FrameLen:     uint16(frameLen),             // ≤400
		TotalPackets: totalPackets,
		Endian:       binary.BigEndian,
	}
	pktReq, err := frameparser.BuildUpgradeRequestEx(meta)
	if err != nil {
		return fmt.Errorf("发送升级请求报文失败: %w", err)
	}
	mu2.Lock()
	readyFlag = 0 //未就绪
	mu2.Unlock()
	relay.SendFrame("update", config.EidStr, pktReq)

	// 等设备就绪
	if err := waitReady(ctx, 1000*time.Second); err != nil {
		return err
	}
	d.lc.Infof("设备应答就绪，开始传输数据... (总包数=%d)", totalPackets)

	// 逐包发送升级数据
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		subNo := uint16(i + 1)

		pktData, err := frameparser.BuildUpgradeDataPacket(config.EidStr, frameNo, subNo, chunk)
		if err != nil {
			return fmt.Errorf("BuildUpgradeDataPacket sub=%d 失败: %w", subNo, err)
		}
		config.SetAck(false)
		if err := relay.SendFrame("update", config.EidStr, pktData); err != nil {
			return fmt.Errorf("send frame sub=%d: %w", subNo, err)
		}

		// 轮询等待 ACK==1；若超时/取消则返回
		const ackWait = 200 * time.Second
		if err := waitAck(ctx, ackWait); err != nil {
			d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
			return err
		}
	}

	d.lc.Infof("初次发送完毕，进入补包处理/结束确认阶段")
	// 结束/补包阶段
	const (
		compCollectWindow = 2 * time.Second //固定时间读是否收到补包清单
		maxCompRounds     = 10              // 最多补包轮数
	)

	for round := 1; round <= maxCompRounds; round++ {
		// 发送结束报文
		if err := frameparser.CompReg.SendEndAndConfirm(ctx, fileName, frameNo); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(compCollectWindow):
		}

		// 取补包清单
		sum, nos, _ := frameparser.CompReg.Snapshot(deviceName)
		// 没有缺包，升级结束
		if sum == 0 || len(nos) == 0 {
			d.lc.Infof("设备 %s 无补包请求，本轮结束（round=%d）", deviceName, round)
			break
		}

		d.lc.Infof("设备 %s 请求补包 %d 个：%v（round=%d）", deviceName, sum, nos, round)
		// 补发缺包
		for _, subNo := range nos {
			// 上下文检查
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if subNo == 0 || int(subNo) > len(chunks) {
				d.lc.Warnf("忽略非法补包号 subNo=%d（总片数=%d）", subNo, len(chunks))
				continue
			}

			pkt, err := frameparser.BuildUpgradeDataPacket(config.EidStr, frameNo, subNo, chunks[int(subNo)-1])
			if err != nil {
				return fmt.Errorf("补包构造失败 sub=%d: %w", subNo, err)
			}

			if err := relay.SendFrame("updata", config.EidStr, pkt); err != nil {
				return fmt.Errorf("补包发送失败 sub=%d: %w", subNo, err)
			}

			// 轮询等待 ACK==1；若超时/取消则返回
			const ackWait = 2 * time.Second
			if err := waitAck(ctx, ackWait); err != nil {
				d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
				return err
			}
		}

		// 本轮补包完成，清空数据
		frameparser.CompReg.Clear(deviceName)
	}

	d.lc.Infof("升级流程完成：设备=%s，文件=%s", deviceName, fileName)
	return nil
}

// 等待设备就绪
func waitReady(ctx context.Context, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		mu2.RLock()
		ready := (readyFlag == 1)
		mu2.RUnlock()
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待设备就绪超时")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 400B 切片
func split400(b []byte) [][]byte {
	if len(b) == 0 {
		return nil
	}
	const n = 400
	out := make([][]byte, 0, (len(b)+n-1)/n)
	for i := 0; i < len(b); i += n {
		j := i + n
		if j > len(b) {
			j = len(b)
		}
		chunk := make([]byte, j-i)
		copy(chunk, b[i:j])
		out = append(out, chunk)
	}
	return out
}
