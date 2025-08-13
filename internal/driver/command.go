package driver

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/frameparser"
	"github.com/linjuya-lu/device-wiresink-go/internal/relay"
)

// handleResetCommand 封装了对 Reset_Set 资源写入后的完整处理：
// 1. 获取设备 EID
// 2. 校验并解码为 6 字节 sensorID
// 3. 构建复位帧
// 4. 通过串口层发送 AT+DTXSTR 命令
func (d *WireSinkDriver) handleTimeParameterSet(deviceName string) error {
	d.lc.Infof("开始处理时间设置命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleResetCommand(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdQuery(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdMoniDataQuery(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleIdAlarmParaQuery(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}

func (d *WireSinkDriver) handleGeneParaQuery(deviceName string) error {
	d.lc.Infof("开始处理复位命令: %s", deviceName)
	// 获取设备的 EID 字符串
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
	d.lc.Infof("已发送复位命令到设备 %s (EID: %s)", deviceName, eidStr)
	return nil
}
