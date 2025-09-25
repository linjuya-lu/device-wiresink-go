package driver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// 升级服务器参数
var (
	upgradeListener   net.Listener
	upgradeListenerMu sync.Mutex
)

// TCP服务器是否启动
func ensureUpgradeTCPServer(cfgPort uint32) (uint32, error) {
	upgradeListenerMu.Lock()
	defer upgradeListenerMu.Unlock()

	// 已启动
	if upgradeListener != nil {
		if a, ok := upgradeListener.Addr().(*net.TCPAddr); ok {
			return uint32(a.Port), nil
		}
		return 0, fmt.Errorf("unexpected listener addr: %v", upgradeListener.Addr())
	}

	// 未启动
	addr := fmt.Sprintf(":%d", cfgPort) // 0 -> 自动分配
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listen tcp %s: %w", addr, err)
	}
	upgradeListener = ln

	// 实际端口
	a := ln.Addr().(*net.TCPAddr)
	actual := uint32(a.Port)

	// 接收循环
	go func(l net.Listener) {
		backoff := 100 * time.Millisecond
		for {
			conn, err := l.Accept()
			if err != nil {
				// 1) 监听器被关闭：优雅退出
				if errors.Is(err, net.ErrClosed) {
					return
				}

				// 2) 超时/截止：可继续重试（如果你给 listener 设过 deadline）
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					time.Sleep(backoff)
					continue
				}
				// 也可判断通用的截止错误
				if errors.Is(err, os.ErrDeadlineExceeded) {
					time.Sleep(backoff)
					continue
				}

				// 3) 系统调用被中断：可继续重试（类 Unix）
				if errors.Is(err, syscall.EINTR) {
					continue
				}

				// 4) 其它错误：退出
				return
			}

			go handleUpgradeConn(conn) // 交给你的会话处理
		}
	}(upgradeListener)

	return actual, nil
}

func handleUpgradeConn(c net.Conn) {
	setUpgradeConn(c) // 告诉发送端：连接已就绪

	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Minute)) // 防永久阻塞，按需调整
	br := bufio.NewReaderSize(c, 8<<10)

	// 可作为 deviceName 的 key：用 CMD_ID 解析出的字符串；
	// 若你已有 deviceName 参数，也可以直接用外部值。
	var lastDeviceName string

	var acc []byte
	for {
		chunk := make([]byte, 4096)
		n, err := br.Read(chunk)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return
			}
			// 超时可续读
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				_ = c.SetReadDeadline(time.Now().Add(5 * time.Minute))
				continue
			}
			// 其它错误：退出
			return
		}
		acc = append(acc, chunk[:n]...)

		// 尝试解帧（可能一包多帧/半包）
		for {
			fr, used, e := frameparser.TryDecodeOne(acc)
			if e == io.ErrUnexpectedEOF {
				// 数据不完整，等更多
				// 丢弃 used 之前的垃圾（通常为 sync 前字节）
				if used > 0 && used < len(acc) {
					acc = acc[used:]
				}
				break
			}
			if e == io.EOF {
				// 没有 sync，丢弃全部
				acc = acc[:0]
				break
			}
			if e != nil && fr == nil {
				// 非致命错误，丢弃本帧，继续
				if used > 0 && used <= len(acc) {
					acc = acc[used:]
					continue
				}
				acc = acc[:0]
				continue
			}

			// 成功解析出一帧
			if used > 0 && used <= len(acc) {
				acc = acc[used:]
			} else {
				acc = acc[:0]
			}

			// CMD_ID 作为 deviceName（去掉尾部 0）
			dev := string(bytes.TrimRight(fr.CmdID[:], "\x00"))
			if dev != "" {
				lastDeviceName = dev
			}

			// 分发处理
			switch fr.PacketType {
			case frameparser.PktUpgradeResp: // 0xB1
				frameparser.HandleUpgradeResp(fr)

			case frameparser.PktUpgradeState: // 0xD1
				frameparser.HandleUpgradeState(fr)

			case frameparser.PktComplementReq: // 0xB4
				key := lastDeviceName
				if key == "" {
					key = "unknown"
				}
				frameparser.HandleComplementReq(fr, key)

			default:
				// 其它类型按需扩展
			}
		}
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

		// 确保TCP服务器已启动
		actualPort, err := ensureUpgradeTCPServer(config.UpgradeTCPPort)
		if err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("start tcp server failed: %v", err)
			return
		}
		d.lc.Infof("upgrade TCP server listening on port=%d", actualPort)

		// 端口写入激活报文 Data（unsigned int 4B），按协议端序选择
		var pktReq [4]byte
		binary.BigEndian.PutUint32(pktReq[:], uint32(actualPort)) // 如果协议是小端就改 LittleEndian

		// 发送 MQTT 激活报文（QoS=1 + 发布超时）
		if err := relay.SendFrameWithQoS("update", config.EidStr, pktReq[:], 1, 10*time.Second); err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("publish upgrade activation failed: %v", err)
			return
		}
		d.lc.Infof("publish upgrade activation ok (QoS=1, port=%d) for %s", actualPort, deviceName)

		// TCP 升级流程
		if err := d.handleUpgradeQuery(ctx, deviceName); err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("upgrade %s failed: %v", deviceName, err)
			return
		}

		d.report(deviceName, "done", nil)
		d.lc.Infof("upgrade %s finished", deviceName)
	}()
	upgradeListenerMu.Lock()
	if upgradeListener != nil {
		_ = upgradeListener.Close()
		upgradeListener = nil
	}
	upgradeListenerMu.Unlock()

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

var (
	upgConnMu    sync.RWMutex
	upgConn      net.Conn // 当前升级用的TCP连接
	upgConnReady = make(chan net.Conn, 1)
)

// 登记连接
func setUpgradeConn(c net.Conn) {
	upgConnMu.Lock()
	defer upgConnMu.Unlock()
	// 有旧连接
	if upgConn != nil {
		_ = upgConn.Close()
	}
	upgConn = c
	// 通知等待者
	select {
	case upgConnReady <- c:
	default:
	}
}

// 取连接
func waitUpgradeConn(ctx context.Context) (net.Conn, error) {
	upgConnMu.RLock()
	c := upgConn
	upgConnMu.RUnlock()
	if c != nil {
		return c, nil
	}
	select {
	case c = <-upgConnReady:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// 发送一帧（带写超时、处理短写）
func sendTCPFrame(ctx context.Context, data []byte) error {
	c, err := waitUpgradeConn(ctx)
	if err != nil {
		return fmt.Errorf("wait tcp conn: %w", err)
	}
	// 写超时（可按需调整）
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	for len(data) > 0 {
		n, err := c.Write(data)
		if err != nil {
			return fmt.Errorf("tcp write: %w", err)
		}
		data = data[n:]
	}
	return nil
}

// 升级流程
func (d *WireSinkDriver) handleUpgradeQuery(ctx context.Context, deviceName string) error {
	d.lc.Infof("开始处理%s的升级命令", deviceName)

	// 接入节点EID
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

	// 固件切片和元数据
	const frameLen = 400 // 单帧最大数据
	chunks := split400(fw)
	totalPackets := uint16((len(fw) + frameLen - 1) / frameLen)
	if totalPackets == 0 {
		totalPackets = 1
	}
	fileName := filepath.Base(filePath)
	var frameNo byte = 0 // 包序号

	// ===== 升级请求帧 =====
	meta := frameparser.UpgradeMeta{
		EID:          "HY_HJWG_202500002",
		FrameNo:      frameNo,
		FrameType:    frameparser.FrameTypeControl,
		PacketType:   frameparser.PacketTypeB1,
		FileName:     fileName,
		FileType:     1,
		TotalSize:    uint32(len(fw)),
		FrameLen:     uint16(frameLen),
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

	//TCP发送
	if err := sendTCPFrame(ctx, pktReq); err != nil {
		return fmt.Errorf("发送升级请求(TCP)失败: %w", err)
	}

	// 等设备就绪
	if err := waitReady(ctx, 1000*time.Second); err != nil {
		return err
	}
	d.lc.Infof("设备应答就绪，开始传输数据... (总包数=%d)", totalPackets)

	// ===== 逐包发送升级数据 =====
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
		// 直接 TCP 发送
		if err := sendTCPFrame(ctx, pktData); err != nil {
			return fmt.Errorf("发送数据帧(TCP)失败 sub=%d: %w", subNo, err)
		}

		// 轮询等待 ACK
		const ackWait = 200 * time.Second
		if err := waitAck(ctx, ackWait); err != nil {
			d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
			return err
		}
	}

	d.lc.Infof("初次发送完毕，进入补包处理/结束确认阶段")

	const (
		compCollectWindow = 2 * time.Second
		maxCompRounds     = 10
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

		sum, nos, _ := frameparser.CompReg.Snapshot(deviceName)
		if sum == 0 || len(nos) == 0 {
			d.lc.Infof("设备 %s 无补包请求，本轮结束（round=%d）", deviceName, round)
			break
		}

		d.lc.Infof("设备 %s 请求补包 %d 个：%v（round=%d）", deviceName, sum, nos, round)
		for _, subNo := range nos {
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
			// 直接TCP发送
			if err := sendTCPFrame(ctx, pkt); err != nil {
				return fmt.Errorf("补包发送失败(TCP) sub=%d: %w", subNo, err)
			}

			const ackWait = 2 * time.Second
			if err := waitAck(ctx, ackWait); err != nil {
				d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
				return err
			}
		}
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
