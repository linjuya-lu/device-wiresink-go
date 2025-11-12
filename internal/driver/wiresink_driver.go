package driver

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/frameparser"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/mqttclient"
)

type WireSinkDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *dsModels.AsyncValues
	locker  sync.Mutex
	sdk     interfaces.DeviceServiceSDK

	upgMu     sync.Mutex //异步升级锁
	upgrading map[string]context.CancelFunc

	upgradeFiles map[string]string // deviceName -> firmware local path

}

var (
	once   sync.Once
	driver *WireSinkDriver
)

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

	host, _ := os.Hostname()
	clientID := fmt.Sprintf("wiresink-%s-%d", host, os.Getpid())

	client, err := mqttclient.NewClient(config.BrokerURL, clientID)
	if err != nil {
		return fmt.Errorf("初始化 MQTT 客户端失败: %w", err)
	}
	mqttclient.MqttClient = client
	if d.upgrading == nil {
		d.upgrading = make(map[string]context.CancelFunc)
	}
	if d.upgradeFiles == nil {
		d.upgradeFiles = make(map[string]string)
	}

	return nil
}

func (d *WireSinkDriver) Start() error {
	//后台升级
	if d.upgrading == nil {
		d.upgrading = make(map[string]context.CancelFunc)
	}
	// 每个设备的固件文件路径
	if d.upgradeFiles == nil {
		d.upgradeFiles = make(map[string]string)
	}
	if err := d.sdk.AddCustomRoute(
		"/custom/firmware-upgrade",
		interfaces.Unauthenticated,
		d.handleFirmwareUpgrade,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register firmware upgrade route failed: %w", err)
	}
	//配置文件下发
	if err := d.sdk.AddCustomRoute(
		"/custom/load-param-map",
		interfaces.Unauthenticated,
		d.handleLoadParamMap,
		http.MethodPost,
	); err != nil {
		return fmt.Errorf("register route failed: %w", err)
	}

	if err := InitDeviceValues(d.sdk); err != nil {
		return fmt.Errorf("Start 初始化设备资源失败: %w", err)
	}
	config.UpdateSensorMapping()

	// MQTT订阅
	if err := mqttclient.SubscribeData(mqttclient.MqttClient, config.MqttTopicUp, 0); err != nil {
		return err
	}

	frameparser.StartParser(mqttclient.SinkRawDataCh, d.AsyncReporting) // 业务数据解析协程
	d.startHealthCheckLoop()                                            // 健康检查
	d.StartAsyncReporter()                                              //心跳上传
	d.lc.Infof("有线汇聚已启动......")
	return nil
}

func (d *WireSinkDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	d.locker.Lock()
	defer d.locker.Unlock()
	d.lc.Debug("读取命令 : 设备=%s, 资源数=%d", deviceName, len(reqs))

	values, ok := config.GetDeviceValues(deviceName)
	if !ok {
		return nil, fmt.Errorf(" 设备 %s 未找到或无可用值", deviceName)
	}
	for _, req := range reqs {
		resName := req.DeviceResourceName
		// 请求路由
		if resName == "topo" {
			config.ClearTopo()
			if err := d.handleRouterParameterQuery(deviceName); err != nil {
				return nil, err
			}
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
		// 时间查询
		if resName == "timeQuery" {
			if err := d.handleTimeParameterSet(deviceName); err != nil {
				return nil, err
			}
			cv, cerr := dsModels.NewCommandValue(resName, common.ValueTypeString, "发送成功")
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue 失败: %w", cerr)
			}
			res = append(res, cv)
			continue
		}
		// 复位设置
		if resName == "reset" {
			if err := d.handleResetCommand(deviceName); err != nil {
				return nil, err
			}
			cv, cerr := dsModels.NewCommandValue(resName, common.ValueTypeString, "发送成功")
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue 失败: %w", cerr)
			}
			res = append(res, cv)
			continue
		}
		// 时间同步
		if resName == "timeSync" {
			if err := d.handleTimeParameterSet(deviceName); err != nil {
				return nil, err
			}
			cv, cerr := dsModels.NewCommandValue(resName, common.ValueTypeString, "发送成功")
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue 失败: %w", cerr)
			}
			res = append(res, cv)
			continue
		}
		// 工况查询
		if resName == "operStatus" {
			if err := d.handleIdMoniDataQuery(deviceName); err != nil {
				return nil, err
			}
			cv, cerr := dsModels.NewCommandValue(resName, common.ValueTypeString, "发送成功")
			if cerr != nil {
				return nil, fmt.Errorf("NewCommandValue 失败: %w", cerr)
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
		d.lc.Infof("读取值: %s.%s = %v", deviceName, resName, val)
		res = append(res, cv)
	}
	return res, nil
}

func (d *WireSinkDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest, params []*dsModels.CommandValue) error {
	d.locker.Lock()
	defer d.locker.Unlock()

	d.lc.Debug("设备=%s, 请求数=%d", deviceName, len(reqs))
	for i, req := range reqs {
		resName := req.DeviceResourceName
		d.lc.Debug("命令%d 写入%s", i, resName)
	}
	return nil
}

func (d *WireSinkDriver) Stop(force bool) error {
	d.lc.Info("有线汇聚结束......")
	close(config.WriteChan)
	return nil
}

// 辅助解析
func parseBin8(s string) (uint8, error) {
	u, err := strconv.ParseUint(s, 2, 8)
	return uint8(u), err
}
func parseBin16(s string) (uint16, error) {
	u, err := strconv.ParseUint(s, 2, 16)
	return uint16(u), err
}
func isBin(s string) bool {
	for _, ch := range s {
		if ch != '0' && ch != '1' {
			return false
		}
	}
	return true
}
func (d *WireSinkDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("添加设备: %s", deviceName)
	//添加EID
	if eid, ok := extractEID(protocols); ok {
		config.AddMapping(eid, deviceName)
	} else {
		d.lc.Warnf("设备 %s 未提供eid", deviceName)
	}
	//初始资源
	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}
	profileName := dev.ProfileName
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("获取设备配置文件 %s 失败: %w", profileName, err)
	}
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType

		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("初始化设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Debugf("已将设备 %s 的资源 %s 初始化: %s (类型: %s)", deviceName, resName, defaultValue, valueType)

		// lora属性解析
		var featStr, typeStr string
		if dr.Attributes != nil {
			if v, ok := dr.Attributes["paramFeatures"]; ok && v != nil {
				featStr = strings.TrimSpace(fmt.Sprint(v))
			}
			if v, ok := dr.Attributes["paramType"]; ok && v != nil {
				typeStr = strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if featStr == "" || typeStr == "" {
			d.lc.Debugf("资源 %s 未配置 attributes.paramFeatures/paramType，跳过登记", resName)
			continue
		}

		featureBits, err1 := parseBin8(featStr)
		typeBits, err2 := parseBin16(typeStr)
		if err1 != nil || err2 != nil {
			d.lc.Warnf("资源 %s 的二进制解析失败: %v %v，跳过登记", resName, err1, err2)
			continue
		}
		key := config.ParamKey{
			FeatureBits: featureBits,
			CodeBits:    typeBits,
		}
		config.ParamEidAdd(key, deviceName, resName)
		d.lc.Debugf("ParamEidRegistry 登记: dev=%s res=%s -> Feature=%03b Code=%011b",
			deviceName, resName, featureBits, typeBits)
	}
	return nil
}

func (d *WireSinkDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("更新设备 %s", deviceName)
	//更新EID
	if eid, ok := extractEID(protocols); ok {
		config.UpdateMapping(eid, deviceName)
	} else {
		d.lc.Warnf("设备 %s 未提供 LoRa.eid", deviceName)
	}
	//更新资源
	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}
	profileName := dev.ProfileName
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("获取设备配置文件 %s 失败: %w", profileName, err)
	}
	for _, dr := range prof.DeviceResources {
		resName := dr.Name
		defaultValue := dr.Properties.DefaultValue
		valueType := dr.Properties.ValueType
		if err := config.DeviceInit(deviceName, resName, defaultValue, valueType); err != nil {
			return fmt.Errorf("更新设备 %s 资源 %s 失败：%v", deviceName, resName, err)
		}
		d.lc.Debugf("已将设备 %s 的资源 %s 初始化: %s (类型: %s)", deviceName, resName, defaultValue, valueType)
		// lora属性解析
		var featStr, typeStr string
		if dr.Attributes != nil {
			if v, ok := dr.Attributes["paramFeatures"]; ok && v != nil {
				featStr = strings.TrimSpace(fmt.Sprint(v))
			}
			if v, ok := dr.Attributes["paramType"]; ok && v != nil {
				typeStr = strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if featStr == "" || typeStr == "" {
			d.lc.Debugf("资源 %s 未配置 attributes.paramFeatures/paramType，跳过登记", resName)
			continue
		}
		if len(featStr) != 3 || len(typeStr) != 11 || !isBin(featStr) || !isBin(typeStr) {
			d.lc.Warnf("资源 %s 的二进制长度/字符非法: paramFeatures=%q paramType=%q，跳过登记", resName, featStr, typeStr)
			continue
		}

		featureBits, err1 := parseBin8(featStr)
		typeBits, err2 := parseBin16(typeStr)
		if err1 != nil || err2 != nil {
			d.lc.Warnf("资源 %s 的二进制解析失败: %v %v，跳过登记", resName, err1, err2)
			continue
		}
		key := config.ParamKey{
			FeatureBits: featureBits,
			CodeBits:    typeBits,
		}
		config.ParamEidUpdate(key, deviceName, resName)
		d.lc.Debugf("ParamEidRegistry 登记: dev=%s res=%s -> Feature=%03b Code=%011b",
			deviceName, resName, featureBits, typeBits)
	}

	d.lc.Infof("刷新设备 %s 的资源值", deviceName)
	return nil
}

func (d *WireSinkDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.lc.Debugf("移除设备： %s", deviceName)
	//移除EID
	if eid, ok := extractEID(protocols); ok {
		config.DeleteMapping(eid)
	} else {
		d.lc.Warnf("设备 %s 未提供 LoRa.eid", deviceName)
	}
	//删除资源
	if err := config.DeleteDeviceValues(deviceName); err != nil {
		d.lc.Errorf("删除设备资源错误 %s : %v", deviceName, err)
		return fmt.Errorf(" %s删除错误 : %w", deviceName, err)
	}
	//删除参数表
	dev, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return fmt.Errorf("获取设备 %s 失败: %w", deviceName, err)
	}
	profileName := dev.ProfileName
	prof, err := d.sdk.GetProfileByName(profileName)
	if err != nil {
		return fmt.Errorf("获取设备配置文件 %s 失败: %w", profileName, err)
	}
	for _, dr := range prof.DeviceResources {
		resName := dr.Name

		var featStr, typeStr string
		if dr.Attributes != nil {
			if v, ok := dr.Attributes["paramFeatures"]; ok && v != nil {
				featStr = strings.TrimSpace(fmt.Sprint(v))
			}
			if v, ok := dr.Attributes["paramType"]; ok && v != nil {
				typeStr = strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if featStr == "" || typeStr == "" {
			d.lc.Debugf("资源 %s 未配置 attributes.paramFeatures/paramType，跳过登记", resName)
			continue
		}
		if len(featStr) != 3 || len(typeStr) != 11 || !isBin(featStr) || !isBin(typeStr) {
			d.lc.Warnf("资源 %s 的二进制长度/字符非法: paramFeatures=%q paramType=%q，跳过登记", resName, featStr, typeStr)
			continue
		}

		featureBits, err1 := parseBin8(featStr)
		typeBits, err2 := parseBin16(typeStr)
		if err1 != nil || err2 != nil {
			d.lc.Warnf("资源 %s 的二进制解析失败: %v %v，跳过登记", resName, err1, err2)
			continue
		}
		key := config.ParamKey{
			FeatureBits: featureBits,
			CodeBits:    typeBits,
		}
		config.ParamEidDelete(key, deviceName)
		d.lc.Debugf("ParamEidRegistry 删除: dev=%s res=%s -> Feature=%03b Code=%011b",
			deviceName, resName, featureBits, typeBits)
	}

	if err := config.DeleteSensorIDMappingsByDevice(deviceName); err != nil {
		d.lc.Errorf("删除设备映射错误 %s : %v", deviceName, err)
		return fmt.Errorf("删除错误 %s : %w", deviceName, err)
	}
	d.lc.Infof("成功移除 %s ", deviceName)

	return nil
}

func (s *WireSinkDriver) ValidateDevice(device models.Device) error {
	var lora models.ProtocolProperties
	for k, v := range device.Protocols {
		if strings.EqualFold(k, "LoRa") {
			lora = v
			break
		}
	}
	if lora == nil {
		return errors.New("协议字段未包含 'LoRa'")
	}

	raw, ok := lora["eid"]
	if !ok {
		return errors.New("未包含 'LoRa.eid'")
	}
	eid, ok := raw.(string)
	if !ok {
		return errors.New("LoRa.eid 不是字符串")
	}
	eid = strings.TrimSpace(eid)
	if eid == "" {
		return errors.New("LoRa.eid 为空")
	}

	return nil
}

func (d *WireSinkDriver) Discover() error {
	return fmt.Errorf("Discover 未实现")
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

// 提取 LoRa.eid
func extractEID(protocols map[string]models.ProtocolProperties) (string, bool) {
	// 找到 "lora"
	var loraProps models.ProtocolProperties
	for k, v := range protocols {
		if strings.EqualFold(k, "lora") {
			loraProps = v
			break
		}
	}
	if loraProps == nil {
		return "", false
	}

	// 读出 eid
	for _, key := range []string{"eid"} {
		if val, ok := loraProps[key]; ok {
			switch t := val.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s, true
				}
			default:
				fmt.Printf("LoRa.eid 非字符串类型: %T\n", t)
			}
		}
	}
	return "", false
}
