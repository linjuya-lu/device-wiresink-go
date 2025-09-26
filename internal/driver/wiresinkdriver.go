package driver

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/linjuya-lu/device-wiresink-go/internal/config"
	"github.com/linjuya-lu/device-wiresink-go/internal/frameparser"
	"github.com/linjuya-lu/device-wiresink-go/internal/mqttclient"
)

type WireSinkDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *dsModels.AsyncValues
	locker  sync.Mutex
	sdk     interfaces.DeviceServiceSDK

	upgMu     sync.Mutex //异步升级专业锁
	upgrading map[string]context.CancelFunc
	progCh    chan UpgradeProgress // 未实现：进度/结果上报

	upgCtx    context.Context
	upgCancel context.CancelFunc
}

var once sync.Once
var driver *WireSinkDriver

func WireSinkDeviceDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(WireSinkDriver)
	})
	return driver
}

func (d *WireSinkDriver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdk = sdk
	d.lc = sdk.LoggingClient()
	d.asyncCh = sdk.AsyncValuesChannel()
	//MQTT初始化参数
	brokerURL := "tcp://192.168.75.137:1883"
	host, _ := os.Hostname()
	clientID := fmt.Sprintf("Initialize wiresink-%s-%d", host, os.Getpid())

	client, err := mqttclient.NewClient(brokerURL, clientID)
	if err != nil {
		return fmt.Errorf("Initialize 初始化 MQTT 客户端失败: %w", err)
	}
	mqttclient.MqttClient = client

	if d.upgrading == nil {
		d.upgrading = make(map[string]context.CancelFunc)
	}
	return nil
}

func (d *WireSinkDriver) Start() error {
	//元信息文件目录
	devicesYAML := "../cmd/res/devices/devices.yaml"
	profilesDir := "../cmd/res/profiles"

	if err := config.InitDeviceResources(devicesYAML, profilesDir); err != nil {
		return fmt.Errorf("Start 初始化设备资源失败: %w", err)
	}

	// MQTT订阅
	if err := mqttclient.SubscribeData(mqttclient.MqttClient, "edgex/service/request/device_wiresink/up", 0); err != nil {
		return err
	}

	// 升级报文解析协程
	d.upgCtx, d.upgCancel = context.WithCancel(context.Background())
	go d.runUpgradeDispatcher(d.upgCtx)

	// 业务数据解析协程
	frameparser.StartParser(mqttclient.SinkRawDataCh, d.AsyncReporting)

	// 业务数据分片解析协程
	go func() {
		if err := frameparser.ShardingParser(frameparser.SDUCh); err != nil {
			d.lc.Errorf("Start 分片解析 异常退出: %v", err)
		}
	}()

	// EID、设备名映射
	config.UpdateSensorMapping()

	startHealthCheckLoop() // 健康检查

	d.lc.Infof("有线汇聚服务已启动......")
	return nil
}

func (d *WireSinkDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	d.locker.Lock()
	defer d.locker.Unlock()
	d.lc.Infof("上层读取命令 : 设备=%s, 资源数=%d", deviceName, len(reqs))

	values, ok := config.GetDeviceValues(deviceName)
	if !ok {
		return nil, fmt.Errorf(" 设备 %s 未找到或无可用值", deviceName)
	}
	for _, req := range reqs {
		resName := req.DeviceResourceName
		// 如果是请求路由
		if resName == "topologyDiagram" {
			topo := config.GetTopoList()
			fmt.Printf("拓扑路由:%s", topo)
			cv, cerr := dsModels.NewCommandValue(
				resName,
				common.ValueTypeObject,
				topo,
			)
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue函数 失败: %w", cerr)
			}
			res = append(res, cv)
			continue
		}
		// 常规资源
		val, exists := values[resName]
		if !exists {
			return nil, fmt.Errorf(" 设备 %s 上未找到资源 %s 的值", deviceName, resName)
		}
		cv, err := makeCV(resName, req.Type, val)
		if err != nil {
			return nil, err
		}
		d.lc.Infof("HandleReadCommands函数 读取值: %s.%s = %v", deviceName, resName, val)
		res = append(res, cv)
	}
	return res, nil
}

func (d *WireSinkDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest, params []*dsModels.CommandValue) error {
	d.locker.Lock()
	defer d.locker.Unlock()

	d.lc.Infof("获取命令: 设备=%s, 请求数=%d", deviceName, len(reqs))

	for i, req := range reqs {
		resName := req.DeviceResourceName
		cv := params[i]
		v, _ := cv.Int8Value()
		d.lc.Infof("常规命令 %d Resource=%s", i, resName)
		// 时间查询
		if resName == "Time_Parameter_Query" && v == 1 {
			if err := d.handleTimeParameterQuery(deviceName); err != nil {
				return err
			}
		}
		// 时间设置
		if resName == "Time_Parameter_Set" && v == 1 {
			if err := d.handleTimeParameterSet(deviceName); err != nil {
				return err
			}
		}
		// 复位
		if resName == "Reset_Set" && v == 1 {
			if err := d.handleResetCommand(deviceName); err != nil {
				return err
			}
		}
		// ID查询
		if resName == "ID_Query" && v == 1 {
			if err := d.handleIdQuery(deviceName); err != nil {
				return err
			}
		}
		// 告警数据查询
		if resName == "Alarm_Parameter_Query" && v == 1 {
			if err := d.handleIdAlarmParaQuery(deviceName); err != nil {
				return err
			}
		}
		// 检测参数查询
		if resName == "Monitoring_Data_Query" && v == 1 {
			if err := d.handleIdMoniDataQuery(deviceName); err != nil {
				return err
			}
		}
		// 网络拓扑查询
		if resName == "topologyDiagramQuery" && v == 1 {
			if err := d.handleRouterParameterQuery(deviceName); err != nil {
				return err
			}
		}
		// 升级
		if resName == "Upgrade" && v == 1 {
			config.Frames.Clear() // 清帧状态表
			_ = d.startUpgradeAsync(deviceName)
		}
	}
	return nil
}

func (d *WireSinkDriver) Stop(force bool) error {
	d.lc.Info("Stop: device-wiresink driver is stopping...")
	close(config.WriteChan)
	return nil
}

func (d *WireSinkDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("AddDevice 新设备已添加: %s", deviceName)

	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}

	profileName := dev.ProfileName

	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("AddDevice 获取设备配置文件 %s 失败: %w", profileName, err)
	}

	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("AddDevice 初始化设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Infof("AddDevice 已将设备 %s 的资源 %s 初始化为默认值: %s (类型: %s)", deviceName, resName, defaultValue, valueType)
	}
	return nil
}

func (d *WireSinkDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("UpdateDevice Device %s is updated", deviceName)

	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("UpdateDevice 获取设备 %s 失败: %w", deviceName, err)
	}
	profileName := dev.ProfileName
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("UpdateDevice 获取设备配置文件 %s 失败: %w", profileName, err)
	}
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("UpdateDevice 更新设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Infof("UpdateDevice 已将设备 %s 的资源 %s 重新初始化为默认值: %s (类型: %s)", deviceName, resName, defaultValue, valueType)
	}

	d.lc.Infof("UpdateDevice 已刷新设备 %s 的资源值为最新默认配置", deviceName)
	return nil
}

func (d *WireSinkDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.lc.Debugf("RemoveDevice Device %s is removed", deviceName)

	if err := config.DeleteDeviceValues(deviceName); err != nil {
		d.lc.Errorf("RemoveDevice 删除设备 %s 的运行时值失败: %v", deviceName, err)
		return fmt.Errorf("RemoveDevice 删除设备 %s 的运行时值失败: %w", deviceName, err)
	}

	if err := config.DeleteSensorIDMappingsByDevice(deviceName); err != nil {
		d.lc.Errorf("RemoveDevice 删除设备 %s 的传感器映射失败: %v", deviceName, err)
		return fmt.Errorf("RemoveDevice 删除设备 %s 的传感器映射失败: %w", deviceName, err)
	}
	d.lc.Infof("RemoveDevice 已移除设备 %s 的所有运行时数据和映射", deviceName)
	return nil
}

func (d *WireSinkDriver) ValidateDevice(device models.Device) error {
	d.lc.Debug("ValidateDevice 未实现")
	return nil
}
func (d *WireSinkDriver) Discover() error {
	return fmt.Errorf("ValidateDevice 未实现")
}

// EdgeX类型匹配
func coerceTo(val any, valueType string) (any, error) {
	switch valueType {

	case common.ValueTypeBool:
		switch x := val.(type) {
		case bool:
			return x, nil
		case string:
			b, err := strconv.ParseBool(x)
			if err != nil {
				return nil, fmt.Errorf("coerceTo parse %q as bool: %w", x, err)
			}
			return b, nil
		case float64:
			return x != 0, nil
		case int, int32, int64, uint, uint32, uint64:
			return fmt.Sprint(x) != "0", nil
		}

	case common.ValueTypeInt8:
		if v, ok := toInt64(val); ok {
			if v < math.MinInt8 || v > math.MaxInt8 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in int8 range", v)
			}
			return int8(v), nil
		}
		return nil, typeErr(val, "int8")

	case common.ValueTypeInt16:
		if v, ok := toInt64(val); ok {
			if v < math.MinInt16 || v > math.MaxInt16 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in int16 range", v)
			}
			return int16(v), nil
		}
		return nil, typeErr(val, "int16")

	case common.ValueTypeInt32:
		if v, ok := toInt64(val); ok {
			if v < math.MinInt32 || v > math.MaxInt32 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in int32 range", v)
			}
			return int32(v), nil
		}
		return nil, typeErr(val, "int32")

	case common.ValueTypeInt64:
		if v, ok := toInt64(val); ok {
			return v, nil
		}
		return nil, typeErr(val, "int64")

	case common.ValueTypeUint8:
		if v, ok := toUint64(val); ok {
			if v > math.MaxUint8 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in uint8 range", v)
			}
			return uint8(v), nil
		}
		return nil, typeErr(val, "uint8")

	case common.ValueTypeUint16:
		if v, ok := toUint64(val); ok {
			if v > math.MaxUint16 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in uint16 range", v)
			}
			return uint16(v), nil
		}
		return nil, typeErr(val, "uint16")

	case common.ValueTypeUint32:
		if v, ok := toUint64(val); ok {
			if v > math.MaxUint32 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in uint32 range", v)
			}
			return uint32(v), nil
		}
		return nil, typeErr(val, "uint32")

	case common.ValueTypeUint64:
		if v, ok := toUint64(val); ok {
			return v, nil
		}
		return nil, typeErr(val, "uint64")

	case common.ValueTypeFloat32:
		if f, ok := toFloat64(val); ok {
			if f < -math.MaxFloat32 || f > math.MaxFloat32 {
				return nil, fmt.Errorf("coerceTo overflow: %v not in float32 range", f)
			}
			return float32(f), nil
		}
		return nil, typeErr(val, "float32")

	case common.ValueTypeFloat64:
		if f, ok := toFloat64(val); ok {
			return f, nil
		}
		return nil, typeErr(val, "float64")

	case common.ValueTypeString:
		switch x := val.(type) {
		case string:
			return x, nil
		default:
			return fmt.Sprint(x), nil
		}

	case common.ValueTypeBinary:
		switch x := val.(type) {
		case []byte:
			return x, nil
		case string:
			b, err := hex.DecodeString(x)
			if err != nil {
				return nil, fmt.Errorf("coerceTo parse %q as hex []byte: %w", x, err)
			}
			return b, nil
		}
		return nil, typeErr(val, "[]byte")
	}

	return nil, fmt.Errorf("coerceTo unsupported ValueType %q", valueType)
}

func typeErr(v any, want string) error {
	return fmt.Errorf("type %T not compatible with %s", v, want)
}

// 类型转换
func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint:
		return int64(x), true
	case float64:
		return int64(x), true
	case float32:
		return int64(x), true
	case string:
		if i, err := strconv.ParseInt(x, 10, 64); err == nil {
			return i, true
		}
		if u, err := strconv.ParseUint(x, 10, 64); err == nil {
			return int64(u), true
		}
	}
	return 0, false
}
func toUint64(v any) (uint64, bool) {
	switch x := v.(type) {
	case uint8:
		return uint64(x), true
	case uint16:
		return uint64(x), true
	case uint32:
		return uint64(x), true
	case uint64:
		return x, true
	case uint:
		return uint64(x), true
	case int, int8, int16, int32, int64:
		i, _ := toInt64(x)
		if i >= 0 {
			return uint64(i), true
		}
	case float64:
		if x >= 0 {
			return uint64(x), true
		}
	case float32:
		if x >= 0 {
			return uint64(x), true
		}
	case string:
		if u, err := strconv.ParseUint(x, 10, 64); err == nil {
			return u, true
		}
	}
	return 0, false
}
func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int, int8, int16, int32, int64:
		i, _ := toInt64(x)
		return float64(i), true
	case uint, uint8, uint16, uint32, uint64:
		u, _ := toUint64(x)
		return float64(u), true
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
func makeCV(name string, valueType string, val any) (*dsModels.CommandValue, error) {
	cval, err := coerceTo(val, valueType)
	if err != nil {
		return nil, fmt.Errorf("coerce %s to %s failed: %w", name, valueType, err)
	}
	cv, err := dsModels.NewCommandValue(name, valueType, cval)
	if err != nil {
		return nil, err
	}
	cv.Origin = time.Now().UnixNano()
	return cv, nil
}

func (d *WireSinkDriver) runUpgradeDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case raw := <-mqttclient.UpgradeRawDataCh:
			if err := d.handleUpgradeFrame(raw); err != nil {
				d.lc.Errorf("升级帧处理: %v", err)
			}
		}
	}
}

// 升级请求响应、升级补包、升级状态解析
func (d *WireSinkDriver) handleUpgradeFrame(data []byte) error {
	if len(data) < 24 { // 至少能取到 pktType(22) + End(最后1B)
		return fmt.Errorf("帧过短: %d", len(data))
	}

	// 头校验：同时接受 5A A5 / A5 5A
	beHeader := data[0] == 0x5A && data[1] == 0xA5
	leHeader := data[0] == 0xA5 && data[1] == 0x5A
	if !beHeader && !leHeader {
		return fmt.Errorf("sync 非 0x5AA5: %02X %02X", data[0], data[1])
	}

	// 尾校验
	if data[len(data)-1] != 0x96 {
		return fmt.Errorf("end 非 0x96: %02X", data[len(data)-1])
	}

	// 调试：打印前几个字节和类型
	pktType := data[22]
	d.lc.Debugf("[UPG] header=%s len=%d pktType=0x%02X raw[0:16]=% X",
		map[bool]string{true: "5A A5"}[beHeader], len(data), pktType, data[:min(16, len(data))])

	switch pktType {
	case 0xB1: // 升级请求响应
		resp, err := frameparser.ParseUpgradeResponse(data)
		if err != nil {
			return fmt.Errorf("ParseUpgradeResponse: %w", err)
		}
		d.lc.Infof("B1 响应: FrameNo=%d Status=0x%X", resp.FrameNo, resp.CommandStatus)

		// 写入准备就绪标志
		frameparser.MuReady.Lock()
		if resp.CommandStatus == 0xFF {
			frameparser.ReadyFlag = 1
		} else {
			frameparser.ReadyFlag = 0
		}
		frameparser.MuReady.Unlock()

	case 0xB4: // 补包请求
		cp, err := frameparser.ParseComplementPacket(data)
		if err != nil {
			return fmt.Errorf("ParseComplementPacket: %w", err)
		}
		dev := asciiTrim(cp.CMD_ID[:])
		frameparser.CompReg.Set(dev, cp.ComplementPackSum, cp.ComplementPackNo)
		d.lc.Infof("B4 补包: dev=%s sum=%d nos=%v file=%s",
			dev, cp.ComplementPackSum, cp.ComplementPackNo, cp.FileName)

	case 0xD1: // 升级状态
		config.SetAck(true)
		// st, err := frameparser.ParseUpgradeStatus(data)
		// if err != nil {
		// 	return fmt.Errorf("ParseUpgradeStatus: %w", err)
		// }
		// dev := asciiTrim(st.CMD_ID[:])
		// d.lc.Infof("D1 状态: dev=%s state=%d desc=%q", dev, st.DeviceState, st.Description)
		// if st.DeviceState == 2 { // 升级中 → 置 ACK
		// 	config.Frames.SetAcked(uint16(st.FrameNo), true)
		// 	d.lc.Infof("ACK 置位: no=%d -> true", st.FrameNo)
		// }

	default:
		d.lc.Warnf("未知升级报文类型: 0x%X", pktType)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ASCII码去尾部0
func asciiTrim(b []byte) string {
	n := len(b)
	for n > 0 && b[n-1] == 0 {
		n--
	}
	return string(b[:n])
}
