package driver

import (
	"encoding/hex"
	"fmt"
	"log"
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
	// -- 初始化 MQTT 客户端 -- //
	brokerURL := "tcp://172.16.19.101:1883"
	host, _ := os.Hostname()
	clientID := fmt.Sprintf("wiresink-%s-%d", host, os.Getpid())

	client, err := mqttclient.NewClient(brokerURL, clientID)
	if err != nil {
		return fmt.Errorf("初始化 MQTT 客户端失败: %w", err)
	}
	mqttclient.MqttClient = client
	return nil
}

func (d *WireSinkDriver) Start() error {

	devicesYAML := "../cmd/res/devices/devices.yaml"
	profilesDir := "../cmd/res/profiles"

	if err := config.InitDeviceResources(devicesYAML, profilesDir); err != nil {
		return fmt.Errorf("初始化设备资源失败: %w", err)
	}
	//订阅
	if err := mqttclient.SubscribeSinkData(mqttclient.MqttClient, "edgex/service/request/device_wiresink/up", 0); err != nil {
		log.Fatal(err)
	}
	// 解协程
	frameparser.StartParser(mqttclient.SinkRawDataCh, d.AsyncReporting)

	//分片解析
	go func() {
		if err := frameparser.ShardingParser(frameparser.SDUCh); err != nil {
			d.lc.Error("ShardingParser 异常退出: %v", err)
		}
	}()

	//EID和设备名映射
	config.UpdateSensorMapping()

	startHealthCheckLoop() //状态控制
	d.lc.Infof("有线汇聚类边代已启动")
	return nil
}

func (d *WireSinkDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	d.locker.Lock()
	defer d.locker.Unlock()
	d.lc.Infof("HandleReadCommands 调用: 设备=%s, 请求资源数=%d", deviceName, len(reqs))

	values, ok := config.GetDeviceValues(deviceName)
	if !ok {
		return nil, fmt.Errorf("设备 %s 未找到或无可用值", deviceName)
	}
	for _, req := range reqs {
		resName := req.DeviceResourceName
		// 如果是路由信息，取数据，序列化
		if resName == "topologyDiagram" {
			topo := config.GetTopoList() // []config.NodeTopology
			fmt.Printf("topo:%s", topo)
			// 序列化成 CommandValue
			cv, cerr := dsModels.NewCommandValue(
				resName,
				common.ValueTypeObject, //  Object 类型
				topo,
			)
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue 失败: %w", cerr)
			}
			res = append(res, cv)
			continue
		}
		// 一般资源从 config 读取
		val, exists := values[resName]
		if !exists {
			return nil, fmt.Errorf("设备 %s 上未找到资源 %s 的值", deviceName, resName)
		}
		cv, err := makeCV(resName, req.Type, val)
		if err != nil {
			return nil, err
		}
		d.lc.Infof("读取值: %s.%s = %v", deviceName, resName, val)
		res = append(res, cv)
	}
	return res, nil
}

func (d *WireSinkDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest, params []*dsModels.CommandValue) error {
	d.locker.Lock()
	defer d.locker.Unlock()

	d.lc.Infof("HandleWriteCommands 调用: 设备=%s, 写入请求数=%d", deviceName, len(reqs))

	if len(reqs) != len(params) {
		d.lc.Errorf("请求数与参数数不匹配: %d vs %d", len(reqs), len(params))
		return fmt.Errorf("请求数与参数数不匹配")
	}
	for i, req := range reqs {
		resName := req.DeviceResourceName
		cv := params[i]
		// 命令类型转换
		v, _ := cv.Int8Value()
		d.lc.Infof("Int8Value = %d", v)
		// 如果是时间参数查询且值为 1
		if resName == "Time_Parameter_Query" && v == 1 {
			if err := d.handleTimeParameterQuery(deviceName); err != nil {
				return err
			}
		}
		// 如果是时间参数设置且值为 1
		if resName == "Time_Parameter_Set" && v == 1 {
			if err := d.handleTimeParameterSet(deviceName); err != nil {
				return err
			}
		}
		// 如果是复位命令且值为 1
		if resName == "Reset_Set" && v == 1 {
			if err := d.handleResetCommand(deviceName); err != nil {
				return err
			}
		}
		// 如果是ID查询命令且值为 1
		if resName == "ID_Query" && v == 1 {
			if err := d.handleIdQuery(deviceName); err != nil {
				return err
			}
		}
		// 如果是所有通用参数查询命令且值为 1
		if resName == "General_Parameter_Query" && v == 1 {
			if err := d.handleGeneParaQuery(deviceName); err != nil {
				return err
			}
		}
		// 如果是所有告警数据查询命令且值为 1
		if resName == "Alarm_Parameter_Query" && v == 1 {
			if err := d.handleIdAlarmParaQuery(deviceName); err != nil {
				return err
			}
		}
		// 如果是所有检测参数查询命令且值为 1
		if resName == "Monitoring_Data_Query" && v == 1 {
			if err := d.handleIdMoniDataQuery(deviceName); err != nil {
				return err
			}
		}
		// 如果是网络拓扑查询命令且值为 1
		if resName == "topologyDiagramQuery" && v == 1 {
			if err := d.handleRouterParameterQuery(deviceName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *WireSinkDriver) Stop(force bool) error {
	d.lc.Info("wireSinkDriver.Stop: device-wiresink driver is stopping...")
	// 关闭通道
	close(config.WriteChan)
	return nil
}

// AddDevice 在设备被添加到 Core Metadata 时调用，
// 从 Metadata 中加载 Device 和对应的 DeviceProfile，
// 并针对每个 DeviceResource 调用 CopyDeviceValues 进行初始化。
func (d *WireSinkDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("新设备已添加: %s", deviceName)
	// 获取 Device 对象
	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}
	// 取出 Profile 名称
	profileName := dev.ProfileName
	// 获取DeviceProfile
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("获取设备配置文件 %s 失败: %w", profileName, err)
	}
	// 针对每个资源执行初始化，传递默认值和类型
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("初始化设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Infof("已将设备 %s 的资源 %s 初始化为默认值: %s (类型: %s)", deviceName, resName, defaultValue, valueType)
	}
	return nil
}

func (d *WireSinkDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("Device %s is updated", deviceName)

	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}
	profileName := dev.ProfileName
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("获取设备配置文件 %s 失败: %w", profileName, err)
	}
	// 针对每个资源重新初始化，传递默认值和类型
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("更新设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Infof("已将设备 %s 的资源 %s 重新初始化为默认值: %s (类型: %s)", deviceName, resName, defaultValue, valueType)
	}

	d.lc.Infof("已刷新设备 %s 的资源值为最新默认配置", deviceName)
	return nil
}

func (d *WireSinkDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.lc.Debugf("Device %s is removed", deviceName)

	// 删除运行时值表
	if err := config.DeleteDeviceValues(deviceName); err != nil {
		d.lc.Errorf("删除设备 %s 的运行时值失败: %v", deviceName, err)
		return fmt.Errorf("删除设备 %s 的运行时值失败: %w", deviceName, err)
	}
	// 删除 sensorID 到 deviceName 的所有映射
	if err := config.DeleteSensorIDMappingsByDevice(deviceName); err != nil {
		d.lc.Errorf("删除设备 %s 的传感器映射失败: %v", deviceName, err)
		return fmt.Errorf("删除设备 %s 的传感器映射失败: %w", deviceName, err)
	}
	d.lc.Infof("已移除设备 %s 的所有运行时数据和映射", deviceName)
	return nil
}

func (d *WireSinkDriver) ValidateDevice(device models.Device) error {
	d.lc.Debug("Driver's ValidateDevice function isn't implemented")
	return nil
}
func (d *WireSinkDriver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}

// coerceTo 把任意 val 转换为与 EdgeX ValueType 匹配的 Go 具体类型。
func coerceTo(val any, valueType string) (any, error) {
	switch valueType {

	case common.ValueTypeBool:
		switch x := val.(type) {
		case bool:
			return x, nil
		case string:
			b, err := strconv.ParseBool(x)
			if err != nil {
				return nil, fmt.Errorf("parse %q as bool: %w", x, err)
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
				return nil, fmt.Errorf("overflow: %v not in int8 range", v)
			}
			return int8(v), nil
		}
		return nil, typeErr(val, "int8")

	case common.ValueTypeInt16:
		if v, ok := toInt64(val); ok {
			if v < math.MinInt16 || v > math.MaxInt16 {
				return nil, fmt.Errorf("overflow: %v not in int16 range", v)
			}
			return int16(v), nil
		}
		return nil, typeErr(val, "int16")

	case common.ValueTypeInt32:
		if v, ok := toInt64(val); ok {
			if v < math.MinInt32 || v > math.MaxInt32 {
				return nil, fmt.Errorf("overflow: %v not in int32 range", v)
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
				return nil, fmt.Errorf("overflow: %v not in uint8 range", v)
			}
			return uint8(v), nil
		}
		return nil, typeErr(val, "uint8")

	case common.ValueTypeUint16:
		if v, ok := toUint64(val); ok {
			if v > math.MaxUint16 {
				return nil, fmt.Errorf("overflow: %v not in uint16 range", v)
			}
			return uint16(v), nil
		}
		return nil, typeErr(val, "uint16")

	case common.ValueTypeUint32:
		if v, ok := toUint64(val); ok {
			if v > math.MaxUint32 {
				return nil, fmt.Errorf("overflow: %v not in uint32 range", v)
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
				return nil, fmt.Errorf("overflow: %v not in float32 range", f)
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
			// 允许 hex 字符串（可按需删掉）
			b, err := hex.DecodeString(x)
			if err != nil {
				return nil, fmt.Errorf("parse %q as hex []byte: %w", x, err)
			}
			return b, nil
		}
		return nil, typeErr(val, "[]byte")
	}

	return nil, fmt.Errorf("unsupported ValueType %q", valueType)
}

func typeErr(v any, want string) error {
	return fmt.Errorf("type %T not compatible with %s", v, want)
}

// 帮助：把 any 转成 int64 / uint64 / float64（支持 string / JSON 反序列化常见类型）
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
