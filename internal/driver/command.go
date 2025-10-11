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
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/frameparser"
	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

type UpgradeProgress struct {
	Device string
	Stage  string
	Err    error
}

func (d *WireSinkDriver) handleTimeParameterSet(deviceName string) error {
	d.lc.Debug("时间设置: %s", deviceName)
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
	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("时间设置 已发送时间设置命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleResetCommand(deviceName string) error {
	d.lc.Debug("复位命令: %s", deviceName)
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
	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("复位命令 已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleTimeParameterQuery(deviceName string) error {
	d.lc.Debug("时间参数查询: %s", deviceName)

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
	relay.SendFrame(eidStr, reqFrame)
	d.lc.Infof("时间参数查询 已发送时间参数查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdMoniDataQuery(deviceName string) error {
	d.lc.Debug("检测数据查询: %s", deviceName)
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
	relay.SendFrame(eidStr, frame)
	d.lc.Infof("检测数据查询 已发送检测数据查询命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleRouterParameterQuery(deviceName string) error {
	d.lc.Debugf("拓扑查询: %s", deviceName)

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
	relay.SendFrame(eidStr, frame)
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
func ensureUpgradeTCPServer(cfgPort uint32, deviceName string) (uint32, error) {
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

			// 把 deviceName 传给处理协程
			go handleUpgradeConn(conn, deviceName)
		}
	}(upgradeListener)

	return actual, nil
}

// 十六进制 + ASCII，按 16 字节一行
func dumpHex1(tag string, b []byte, base int) {
	const per = 16
	for off := 0; off < len(b); off += per {
		end := off + per
		if end > len(b) {
			end = len(b)
		}
		chunk := b[off:end]
		ascii := make([]byte, len(chunk))
		for i, c := range chunk {
			if c >= 0x20 && c <= 0x7E {
				ascii[i] = c
			} else {
				ascii[i] = '.'
			}
		}
		log.Printf("%s %04X: % X  | %s", tag, base+off, chunk, ascii)
	}
}

// 仅基于“帧头 + 帧尾(0x96)”摘出一整帧；不依赖 Packet_Length。
// 返回：frame 切片、应丢弃的字节 used、是否需要更多数据 needMore、错误。
func spliceOneFrame(acc []byte) (frame []byte, used int, needMore bool, err error) {
	// 最小“看起来像一帧”的总长度：Sync2 + Len2 + CMD_ID17 + FT1 + PT1 + NO1 + CRC2 + End1 = 27+1=28
	// （B1/D1 的 payload 至少会有 1B，因此 28 是个保守最小值）
	const minTotal = 28
	if len(acc) < 2 {
		return nil, 0, true, nil // 可能只有半个同步头，继续等
	}

	// 找同步头（谁先出现用谁）
	findSync := func(b []byte) (start int, be bool) {
		i := bytes.Index(b, []byte{0x5A, 0xA5}) // BE
		j := bytes.Index(b, []byte{0xA5, 0x5A}) // LE
		switch {
		case i >= 0 && (j < 0 || i < j):
			return i, true
		case j >= 0:
			return j, false
		default:
			return -1, true
		}
	}

	start, _ := findSync(acc)
	if start < 0 {
		// 保留最后 1B（可能是半个同步头：0x5A/0xA5），避免把半个头丢掉
		last := acc[len(acc)-1]
		if last == 0x5A || last == 0xA5 {
			return nil, len(acc) - 1, true, nil
		}
		return nil, len(acc), false, io.EOF
	}

	// 有头，开始扫尾标 0x96；起点跳过最小头部（避免把 payload 里的零散 0x96 过早匹配）
	scanFrom := start + (4 + 17 + 1 + 1 + 1) // = 到 Frame_No 之后
	if scanFrom < start+4 {
		scanFrom = start + 4
	}
	if scanFrom >= len(acc) {
		return nil, start, true, nil // 还没够到能找尾的位置
	}

	// 向后找 0x96
	rel := bytes.IndexByte(acc[scanFrom:], 0x96)
	if rel < 0 {
		// 没有尾标：丢掉头前垃圾，保留从头开始的半帧
		return nil, start, true, nil
	}
	endIdx := scanFrom + rel // 指向 0x96

	// 校验“总长 ≥ 最小帧长”
	if endIdx-start+1 < minTotal {
		// 太短，不像合法帧；跳过当前同步头的两个字节，继续找
		return nil, start + 2, false, fmt.Errorf("frame too short: total=%d", endIdx-start+1)
	}

	// 取 [start, endIdx] 作为完整帧（含 0x96）
	f := acc[start : endIdx+1]
	return f, endIdx + 1, false, nil
}

func handleUpgradeConn(c net.Conn, deviceName string) {
	setUpgradeConn(c)
	defer c.Close()

	const idle = 5 * time.Minute
	_ = c.SetReadDeadline(time.Now().Add(idle))
	br := bufio.NewReaderSize(c, 8<<10)

	var lastDeviceName string
	var acc []byte

	for {
		buf := make([]byte, 4096)
		n, err := br.Read(buf)

		if n > 0 {
			dumpHex1("RX-chunk", buf[:n], 0)
			acc = append(acc, buf[:n]...)

			// ——一次性把 acc 里能凑成的所有帧都取出来——
			for {

				frame, used, needMore, e := spliceOneFrame(acc)

				if needMore {
					// 半帧：丢掉 sync 前垃圾，保留从 sync 开始
					if used > 0 && used < len(acc) {
						acc = acc[used:]
					}
					break
				}
				if e == io.EOF {
					// 没有头：清空
					acc = acc[:0]
					break
				}
				if e != nil && frame == nil {
					// 坏帧：至少推进 used（若无 used 则推进 1）
					if used <= 0 || used > len(acc) {
						used = 1
					}
					acc = acc[used:]
					continue
				}

				// 成功摘到完整帧：从缓存剥离
				if used > 0 && used <= len(acc) {
					acc = acc[used:]
				} else {
					acc = acc[:0]
				}
				// ——到这里才交给解析器（你原来的 TryDecodeOne / ParseXxx）——
				fr, perr := frameparser.ParseFrameBytes(frame)
				if perr != nil {
					log.Printf("[PARSE] err: %v", perr)
					continue
				}

				lastDeviceName = deviceName
				frameparser.PrintFrameBrief(fr)
				// 分发
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
					// 其它类型…
				}
			}
		}

		// 读错误最后处理；确保 n>0 的字节已利用
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				// 连接关闭前可能还有残留半帧：可选 flush 一次
				if len(acc) > 0 {
					_, _, _, _ = spliceOneFrame(acc) // 触发一次提取（可按需处理）
				}
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				_ = c.SetReadDeadline(time.Now().Add(idle))
				continue
			}
			return
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
		actualPort, err := ensureUpgradeTCPServer(config.UpgradeTCPPort, deviceName)
		if err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("start tcp server failed: %v", err)
			return
		}
		d.lc.Infof("upgrade TCP server listening on port=%d", actualPort)

		if err := relay.SendPortDecWithQoS(
			mqttclient.MqttClient,
			"edgex/server/response/device-wiresink/down",
			"update",
			config.EidStr,
			uint32(actualPort),
			0,
			10*time.Second,
		); err != nil {
			d.report(deviceName, "failed", err)
			d.lc.Errorf("publish upgrade activation (decimal) failed: %v", err)
			return
		}
		d.lc.Infof("publish upgrade activation ok (DEC port=%d) for %s", actualPort, deviceName)

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
		// 丢弃
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

// 十六进制 + ASCII 双视图转储，按 16 字节一行
func dumpHex(tag string, b []byte) {
	const per = 16
	for off := 0; off < len(b); off += per {
		end := off + per
		if end > len(b) {
			end = len(b)
		}
		chunk := b[off:end]

		// ASCII 可视化
		ascii := make([]byte, len(chunk))
		for i, c := range chunk {
			if c >= 0x20 && c <= 0x7E { // 可打印
				ascii[i] = c
			} else {
				ascii[i] = '.'
			}
		}
		log.Printf("%s %04X: % X  | %s", tag, off, chunk, ascii)
	}
	log.Printf("%s len=%d bytes", tag, len(b))
}

// 发送一帧（带写超时、处理短写）
func sendTCPFrame(ctx context.Context, data []byte) error {
	c, err := waitUpgradeConn(ctx)
	if err != nil {
		return fmt.Errorf("wait tcp conn: %w", err)
	}

	// 发送前转储
	dumpHex("TX", data)

	// 写超时（可按需调整）
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	left := len(data)
	for len(data) > 0 {
		n, err := c.Write(data)
		if err != nil {
			return fmt.Errorf("tcp write: %w", err)
		}
		data = data[n:]
	}
	sent := left
	log.Printf("TX done: wrote %d bytes", sent)
	config.SetAck(false)
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
	filePath := "./res/updata/HY-BDWG-F470-V10-final-092901.hpk"
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

	//升级请求帧
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

	frameparser.MuReady.Lock()
	frameparser.ReadyFlag = 0 //未就绪
	frameparser.MuReady.Unlock()

	//TCP发送
	if err := sendTCPFrame(ctx, pktReq); err != nil {
		return fmt.Errorf("发送升级请求(TCP)失败: %w", err)
	}

	// 等设备就绪
	fmt.Printf("等待升级中....")
	if err := waitReady(ctx, 10000*time.Second); err != nil {
		fmt.Printf("等待失败....")
		return err
	}
	d.lc.Infof("设备应答就绪，开始传输数据... (总包数=%d)", totalPackets)

	//  逐包发送升级数据
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		subNo := uint16(i + 1)
		frameNo++
		pktData, err := frameparser.BuildUpgradeDataPacket("HY_HJWG_202500002", frameNo, subNo, chunk)
		if err != nil {
			return fmt.Errorf("BuildUpgradeDataPacket sub=%d 失败: %w", subNo, err)
		}

		fmt.Printf("[UPGRADE] TX 子包号=%d 长度=%d (frameNo=%d)\n", subNo, len(chunk), frameNo)

		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			config.Mu3.Lock()
			ready := (config.AckReceived == 1)
			config.Mu3.Unlock()
			if ready {
				break
			}
			time.Sleep(5 * time.Millisecond) // 短睡眠避免忙等
		}

		// 进入等待ACK状态
		config.Mu3.Lock()
		config.AckReceived = 0
		config.Mu3.Unlock()

		// 直接 TCP 发送
		if err := sendTCPFrame(ctx, pktData); err != nil {
			config.Mu3.Lock()
			config.AckReceived = 1
			config.Mu3.Unlock()
			return fmt.Errorf("发送数据帧(TCP)失败 sub=%d: %w", subNo, err)
		}

		// 轮询等待 ACK
		const ackWait = 200 * time.Second
		if err := waitAck(ctx, ackWait); err != nil {
			d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
			return err
		}
	}
	fmt.Printf("发送完毕，进行结束补包阶段")

	const (
		compCollectWindow = 5 * time.Second
		maxCompRounds     = 10
	)
	for round := 1; round <= maxCompRounds; round++ {
		//发送结束报文
		pktEnd, err := frameparser.CompReg.BuildUpgradeEndPacket("HY_HJWG_202500002", fileName, frameNo)
		if err != nil {
			return fmt.Errorf("构建结束报文失败: %w", err)
		}
		if err := sendTCPFrame(ctx, pktEnd); err != nil {
			return fmt.Errorf("发送结束报文(TCP)失败: %w", err)
		}

		//轮询，直到收到 ACK
		deadline := time.Now().Add(compCollectWindow)
		const pollInterval = 80 * time.Millisecond

		var (
			sum uint16
			nos []uint16
			ok  bool
		)
		for {
			// 先看上下文
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// 收到 ACK + 数据
			sum, nos, ok = frameparser.CompReg.Snapshot(deviceName)
			if ok {
				break
			}

			if time.Now().After(deadline) {
				d.lc.Infof("设备 %s 补包 ACK 等待超时(窗口=%s)，本轮结束（round=%d）", deviceName, compCollectWindow, round)
				goto NEXT_ROUND
			}

			time.Sleep(pollInterval)
		}

		// 清理设备记录
		frameparser.CompReg.Clear(deviceName)

		// 补包处理
		if sum == 0 {
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
			if err := sendTCPFrame(ctx, pkt); err != nil {
				return fmt.Errorf("补包发送失败(TCP) sub=%d: %w", subNo, err)
			}

			const ackWait = 2 * time.Second
			if err := waitAck(ctx, ackWait); err != nil {
				d.lc.Errorf("sub=%d 等待ACK超时(%s): %v", subNo, ackWait, err)
				return err
			}
		}

		// 处理完清理
		frameparser.CompReg.Clear(deviceName)

	NEXT_ROUND:
		continue
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
		frameparser.MuReady.RLock()
		ready := (frameparser.ReadyFlag == 1)
		frameparser.MuReady.RUnlock()
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
