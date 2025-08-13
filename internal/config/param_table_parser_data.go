package config

import (
	"encoding/binary"
	"fmt"
	"math"
)

type ParamKey struct {
	FeatureBits byte   // 高3位（参量特征）
	CodeBits    uint16 // 低11位（类型编码）
}

type ParamInfo struct {
	Name     string
	Unit     string
	ByteLen  int
	DataType string
	Parse    func([]byte) (any, error)
}

var paramMap = map[ParamKey]ParamInfo{
	//-------------------------------------------------------D.1通用状态参量类型表------------------------------------------------
	// 基本量
	{0b000, 0b00000000001}: {"Length", "m", 4, "float32", parseFloat32},
	{0b000, 0b00000000010}: {"Mass", "kg", 4, "float32", parseFloat32},
	{0b000, 0b00000000011}: {"Time", "s", 4, "uint32", parseUint32},
	{0b000, 0b00000000100}: {"ElectricCurrent", "A", 4, "float32", parseFloat32},
	{0b000, 0b00000000101}: {"Temperature", "℃", 4, "float32", parseFloat32},
	{0b000, 0b00000000110}: {"AmountOfSubstance", "mol", 4, "float32", parseFloat32},
	{0b000, 0b00000000111}: {"LuminousIntensity", "cd", 4, "float32", parseFloat32},

	// 状态量 & 扩展
	{0b000, 0b00000011100}: {"HeartbeatStatus", "", 1, "uint8", parseUint8},
	{0b000, 0b00000011101}: {"BatteryRemaining", "%", 2, "uint16", parseUint16},
	{0b000, 0b00000011110}: {"BatteryVoltage", "V", 4, "float32", parseFloat32},
	{0b000, 0b00000011111}: {"SensorSelfTestStatus", "", 1, "uint8", parseUint8},
	{0b000, 0b00000100000}: {"NetworkConnectionStatus", "", 1, "uint8", parseUint8},
	{0b000, 0b00000100001}: {"PowerStatus", "", 1, "uint8", parseUint8},
	{0b000, 0b00000100010}: {"DataCollectionInterval", "s", 2, "uint16", parseUint16},
	{0b000, 0b00000100011}: {"SignalStrength", "", 4, "float32", parseFloat32},

	// 电气类
	{0b000, 0b00000111000}: {"PrimaryCurrent", "kA", 4, "float32", parseFloat32},
	{0b000, 0b00000111001}: {"SecondaryCurrent", "mA", 4, "float32", parseFloat32},
	{0b000, 0b00000111010}: {"PrimaryVoltage", "kV", 4, "float32", parseFloat32},
	{0b000, 0b00000111011}: {"SecondaryVoltage", "mV", 4, "float32", parseFloat32},
	{0b000, 0b00000111100}: {"Waveform", "", -1, "float32[]", parsefloat32Array},
	{0b000, 0b00000111101}: {"PhaseAngle", "°", 4, "float32", parseFloat32},
	{0b000, 0b00000111110}: {"Phase", "", 2, "uint16", parseUint16},
	{0b000, 0b00000111111}: {"Frequency", "Hz", 4, "float32", parseFloat32},
	{0b000, 0b00010000000}: {"ActivePower", "W", 4, "float32", parseFloat32},
	{0b000, 0b00010000001}: {"ReactivePower", "W", 4, "float32", parseFloat32},
	{0b000, 0b00010000010}: {"ElectricEnergy", "kWh", 4, "float32", parseFloat32},
	{0b000, 0b00010000011}: {"PowerFactor", "", 4, "float32", parseFloat32},
	{0b000, 0b00010000100}: {"VoltagePresenceIndicator", "", 2, "uint16", parseUint16},
	{0b000, 0b00010000101}: {"ElectricCharge", "C", 4, "float32", parseFloat32},

	//运动与力学类
	{0b000, 0b00001011010}: {"Longitude", "", 4, "float32", parseFloat32},
	{0b000, 0b00001011011}: {"Latitude", "", 4, "float32", parseFloat32},
	{0b000, 0b00001011100}: {"Altitude", "", 4, "float32", parseFloat32},
	{0b000, 0b00001011101}: {"Displacement", "", 4, "float32", parseFloat32},
	{0b000, 0b00001011110}: {"DisplacementTrajectory", "mm", -1, "float32[]", parsefloat32Array},
	{0b000, 0b00001011111}: {"Velocity", "m/s", 4, "float32", parseFloat32},
	{0b000, 0b00001100000}: {"Acceleration", "m/s²", 4, "float32", parseFloat32},
	{0b000, 0b00001100001}: {"Angle", "rad", 4, "float32", parseFloat32},
	{0b000, 0b00001100010}: {"AngularVelocity", "rad/s", 4, "float32", parseFloat32},
	{0b000, 0b00001100011}: {"AngularAcceleration", "rad/s²", 4, "float32", parseFloat32},
	{0b000, 0b00001100100}: {"Strain", "%", 4, "float32", parseFloat32},
	{0b000, 0b00001100101}: {"StressOrPressure", "Pa", 4, "float32", parseFloat32},
	{0b000, 0b00001100110}: {"VibrationSpectrum", "m/s²", -1, "float32[]", parsefloat32Array},
	{0b000, 0b00001100111}: {"Force", "N", 4, "float32", parseFloat32},
	//-------------------------------------------------------D.2输电业务状态参量类型表----------------------------------------------------
	{0b001, 0b00000000001}: {"10minAvgWindSpeed", "m/s", 4, "float32", parseFloat32},
	{0b001, 0b00000000010}: {"10minAvgWindDirection", "°", 2, "int16", parseInt16},
	{0b001, 0b00000000011}: {"MaxWindSpeed", "m/s", 4, "float32", parseFloat32},
	{0b001, 0b00000100100}: {"ExtremeWindSpeed", "m/s", 4, "float32", parseFloat32},
	{0b001, 0b0000010101}:  {"StandardWindSpeed", "m/s", 4, "float32", parseFloat32},
	{0b001, 0b00000110}:    {"Temperature1", "°C", 4, "float32", parseFloat32},
	{0b001, 0b00000111}:    {"Humidity1", "%RH", 2, "uint16", parseUint16},
	{0b001, 0b00001000}:    {"Pressure", "hPa", 4, "float32", parseFloat32},
	{0b001, 0b00001001}:    {"Rainfall10min", "mm", 4, "float32", parseFloat32},
	{0b001, 0b00001010}:    {"RainIntensity", "mm/min", 4, "float32", parseFloat32},
	{0b001, 0b00001011}:    {"SolarRadiation", "W/m2", 2, "uint16", parseUint16},
	{0b001, 0b00001100}:    {"InstantWindSpeed", "m/s", 4, "float32", parseFloat32},
	{0b001, 0b00001101}:    {"InstantWindDirection", "°", 2, "int16", parseInt16},
	{0b001, 0b00001110}:    {"WindDirectionDeviation", "°", 2, "int16", parseInt16},
	//-------------------------------------------------------D.3变电业务状态状态参量类型表------------------------------------------------
	//避雷器泄露电流传感器
	// 1 避雷器泄漏电流全电流
	{0b010, 0b00000000001}: {"ArresterLeakageTotalCurrent", "mA", 4, "float32", parseFloat32},
	// 2 避雷器泄漏电流阻性电流
	{0b010, 0b00000000010}: {"ArresterLeakageResistiveCurrent", "mA", 4, "float32", parseFloat32},
	// 3 泄漏电流采集相位
	{0b010, 0b00000000011}: {"LeakageCurrentSamplingPhase", "°", 4, "float32", parseFloat32},
	// 4 避雷器动作次数
	{0b010, 0b00000000100}: {"ArresterOperationCount", "times", 2, "uint16", parseUint16},
	// 5 避雷器阻性电流（峰值）
	{0b010, 0b00000000101}: {"ArresterResistiveLeakageCurrentPeak", "mA", 4, "float32", parseFloat32},
	// 6 母线电压采集相位
	{0b010, 0b00000000110}: {"BusVoltageSamplingPhase", "°", 4, "float32", parseFloat32},
	//变压器铁芯电流传感器
	// 27 变压器铁芯/夹件接地电流
	{0b010, 0b00000011011}: {"TransformerCoreClipGroundingCurrent", "A", 4, "float32", parseFloat32},
	// 28 变压器铁芯/夹件接地电流频谱
	{0b010, 0b00000011100}: {"TransformerCoreClipGroundingCurrentSpectrum", "A", -1, "uint16[]", parseUint16Array},
	//套管等容性设备传感器
	// 29 介质损耗因数
	{0b010, 0b00000011101}: {"DielectricLossFactor", "°", 4, "float32", parseFloat32},
	// 30 电容量
	{0b010, 0b00000011110}: {"Capacitance", "pF", 4, "float32", parseFloat32},
	// 31 全电流
	{0b010, 0b00000011111}: {"TotalCurrent", "mA", 4, "float32", parseFloat32},
	// 32 初相角
	{0b010, 0b00000100000}: {"InitialPhaseAngle", "°", 4, "float32", parseFloat32},
	// 33 参考电流
	{0b010, 0b00000100001}: {"ReferenceCurrent", "mA", 4, "float32", parseFloat32},
	// 34 参考相角
	{0b010, 0b00000100010}: {"ReferencePhaseAngle", "°", 4, "float32", parseFloat32},
	// 55 合闸位移
	{0b010, 0b00000110111}: {"ClosingDisplacement", "mm", 4, "float32", parseFloat32},
	// 56 合闸角位移
	{0b010, 0b00000111000}: {"ClosingAngularDisplacement", "°", 4, "float32", parseFloat32},
	// 57 合闸速度
	{0b010, 0b00000111001}: {"ClosingSpeed", "m/s", 4, "float32", parseFloat32},
	// 58 合闸时间
	{0b010, 0b00000111010}: {"ClosingTime", "s", 4, "float32", parseFloat32},
	// 59 合闸线圈电流峰值
	{0b010, 0b00000111011}: {"ClosingCoilCurrentPeak", "A", 4, "float32", parseFloat32},
	// 60 合闸线圈电流带电时间
	{0b010, 0b00000111100}: {"ClosingCoilCurrentOnTime", "ms", 4, "float32", parseFloat32},
	// 61 分闸位移
	{0b010, 0b00000111101}: {"OpeningDisplacement", "mm", 4, "float32", parseFloat32},
	// 62 分闸角位移
	{0b010, 0b00000111110}: {"OpeningAngularDisplacement", "°", 4, "float32", parseFloat32},
	// 63 分闸速度
	{0b010, 0b00000111111}: {"OpeningSpeed", "m/s", 4, "float32", parseFloat32},
	// 64 分闸时间
	{0b010, 0b00001000000}: {"OpeningTime", "s", 4, "float32", parseFloat32},
	// 65 分闸线圈电流峰值
	{0b010, 0b00001000001}: {"OpeningCoilCurrentPeak", "A", 4, "float32", parseFloat32},
	// 66 分闸线圈电流带电时间
	{0b010, 0b00001000010}: {"OpeningCoilCurrentOnTime", "ms", 4, "float32", parseFloat32},
	// 67 储能电机工作电流最大值
	{0b010, 0b00001000011}: {"EnergyStorageMotorOperatingCurrentMax", "A", 4, "float32", parseFloat32},
	// 68 储能电机启动电流最大值
	{0b010, 0b00001000100}: {"EnergyStorageMotorStartingCurrentMax", "A", 4, "float32", parseFloat32},
	// 69 储能电机电流时长
	{0b010, 0b00001000101}: {"EnergyStorageMotorCurrentDuration", "ms", 4, "float32", parseFloat32},
	// 70 机构动作次数
	{0b010, 0b00001000110}: {"MechanismOperationCount", "", 1, "uint8", parseUint8},
	// 71 开关分合位置
	{0b010, 0b00001000111}: {"SwitchContactPosition", "", 1, "uint8", parseUint8},
	// 72 传动机构位移一阶时间波形
	{0b010, 0b00001001000}: {"DriveMechanismDisplacementTimeWaveform", "", -1, "float32[]", parsefloat32Array},
	// 73 合闸线圈电流一阶时间波形
	{0b010, 0b00001001001}: {"ClosingCoilCurrentTimeWaveform", "", -1, "float32[]", parsefloat32Array},
	// 74 分闸线圈电流一阶时间波形
	{0b010, 0b00001001010}: {"OpeningCoilCurrentTimeWaveform", "", -1, "float32[]", parsefloat32Array},
	// 75 储能电机电流一阶时间波形
	{0b010, 0b00001001011}: {"EnergyStorageMotorCurrentTimeWaveform", "", -1, "float32[]", parsefloat32Array},
	// 76 开关触头压力
	{0b010, 0b00001001100}: {"SwitchContactPressure", "N", 4, "float32", parseFloat32},
	// 77 有载分接开关档位
	{0b010, 0b00001001101}: {"LoadTapChangerPosition", "", 2, "uint16", parseUint16},
	//局放传感器
	// 98 高频多图谱
	{0b010, 0b00001100010}: {"HighFrequencyMultiSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 99 高频PRPD图
	{0b010, 0b00001100011}: {"HighFrequencyPRPDSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 100 高频PRPS图
	{0b010, 0b00001100100}: {"HighFrequencyPRPSSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 101 高频TF谱图
	{0b010, 0b00001100101}: {"HighFrequencyTFSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 102 特高频多图谱
	{0b010, 0b00001100110}: {"UltraHighFrequencyMultiSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 103 特高频PRPD图
	{0b010, 0b00001100111}: {"UltraHighFrequencyPRPDSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 104 特高频PRPS图
	{0b010, 0b00001101000}: {"UltraHighFrequencyPRPSSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 105 超声多图谱
	{0b010, 0b00001101001}: {"UltrasonicMultiSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 106 超声特征图
	{0b010, 0b00001101010}: {"UltrasonicFeatureSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 107 超声相位图
	{0b010, 0b00001101011}: {"UltrasonicPhaseSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 108 超声脉冲图
	{0b010, 0b00001101100}: {"UltrasonicPulseSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 109 超声波形图
	{0b010, 0b00001101101}: {"UltrasonicWaveform", "", -1, "float32[]", parsefloat32Array},
	// 110 暂态电压多图谱
	{0b010, 0b00001101110}: {"TransientVoltageMultiSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 111 暂态电压幅值
	{0b010, 0b00001101111}: {"TransientVoltageAmplitude", "", -1, "float32[]", parsefloat32Array},
	// 112 暂态电压PRPD图
	{0b010, 0b00001110000}: {"TransientVoltagePRPDSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 113 暂态电压PRPS图
	{0b010, 0b00001110001}: {"TransientVoltagePRPSSpectrum", "", -1, "float32[]", parsefloat32Array},
	// 114 振荡入射波
	{0b010, 0b00001110010}: {"OscillationIncidentWave", "", -1, "float32[]", parsefloat32Array},
	// 115 振荡反射波
	{0b010, 0b00001110011}: {"OscillationReflectedWave", "", -1, "float32[]", parsefloat32Array},
	//油状态类传感器
	// 136 甲烷
	{0b010, 0b00010001000}: {"Methane", "μL/L", 4, "float32", parseFloat32},
	// 137 乙烷
	{0b010, 0b00010001001}: {"Ethane", "μL/L", 4, "float32", parseFloat32},
	// 138 乙烯
	{0b010, 0b00010001010}: {"Ethylene", "μL/L", 4, "float32", parseFloat32},
	// 139 乙炔
	{0b010, 0b00010001011}: {"Acetylene", "μL/L", 4, "float32", parseFloat32},
	// 140 一氧化碳
	{0b010, 0b00010001100}: {"CarbonMonoxide", "μL/L", 4, "float32", parseFloat32},
	// 141 二氧化碳
	{0b010, 0b00010001101}: {"CarbonDioxide", "μL/L", 4, "float32", parseFloat32},
	// 142 氢气
	{0b010, 0b00010001110}: {"Hydrogen", "μL/L", 4, "float32", parseFloat32},
	// 143 水分
	{0b010, 0b00010001111}: {"WaterContent", "μL/L", 4, "float32", parseFloat32},
	// 144 氮气
	{0b010, 0b00010010000}: {"Nitrogen", "μL/L", 4, "float32", parseFloat32},
	// 145 氧气
	{0b010, 0b00010010001}: {"Oxygen", "μL/L", 4, "float32", parseFloat32},
	// 146 总烃
	{0b010, 0b00010010010}: {"TotalHydrocarbon", "μL/L", 4, "float32", parseFloat32},
	// 147 油温
	{0b010, 0b00010010011}: {"OilTemperature", "℃", 4, "float32", parseFloat32},
	// 148 油压
	{0b010, 0b00010010100}: {"OilPressure", "Pa", 4, "float32", parseFloat32},
	// 149 总可燃气
	{0b010, 0b00010010101}: {"TotalCombustibleGas", "μL/L", 4, "float32", parseFloat32},
	// 150 载气压力
	{0b010, 0b00010010110}: {"CarrierGasPressure", "MPa", 4, "float32", parseFloat32},
	//SF6气体状态类传感器
	// 171 SF6 露点
	{0b010, 0b00010101011}: {"SF6DewPoint", "°C", 4, "float32", parseFloat32},
	// 172 SF6 微水
	{0b010, 0b00010101100}: {"SF6Moisture", "μL/L", 4, "float32", parseFloat32},
	// 173 SF6 纯度
	{0b010, 0b00010101101}: {"SF6Purity", "%", 4, "float32", parseFloat32},
	// 174 H2S（分解产物）
	{0b010, 0b00010101110}: {"H2S", "μL/L", 4, "float32", parseFloat32},
	// 175 SO2（分解产物）
	{0b010, 0b00010101111}: {"SO2", "μL/L", 4, "float32", parseFloat32},
	// 176 HF（分解产物）
	{0b010, 0b00010110000}: {"HF", "μL/L", 4, "float32", parseFloat32},
	// 177 SOF2（分解产物）
	{0b010, 0b00010110001}: {"SOF2", "μL/L", 4, "float32", parseFloat32},
	// 178 CF4（分解产物）
	{0b010, 0b00010110010}: {"CF4", "μL/L", 4, "float32", parseFloat32},
	// 179 SO2F2（分解产物）
	{0b010, 0b00010110011}: {"SO2F2", "μL/L", 4, "float32", parseFloat32},
	// 180 CO（分解产物）
	{0b010, 0b00010110100}: {"CO", "μL/L", 4, "float32", parseFloat32},
	// 181 CO2（分解产物）
	{0b010, 0b00010110101}: {"CO2", "μL/L", 4, "float32", parseFloat32},
	// 182 SF6 气体表压
	{0b010, 0b00010110110}: {"SF6GaugePressure", "Pa", 4, "float32", parseFloat32},
	// 183 SF6 气体绝压
	{0b010, 0b00010110111}: {"SF6AbsolutePressure", "Pa", 4, "float32", parseFloat32},
	// 184 SF6 气体 O2+N2
	{0b010, 0b00010111000}: {"SF6O2N2", "μL/L", 4, "float32", parseFloat32},
	// 185 SF6 气体实际压力
	{0b010, 0b00010111001}: {"SF6ActualPressure", "Pa", 4, "float32", parseFloat32},
	// 186 SF6 气体温度
	{0b010, 0b00010111010}: {"SF6Temperature", "°C", 4, "float32", parseFloat32},
	//环境气体传感器
	// 207 氮气
	{0b010, 0b00011001111}: {"Nitrogen", "μL/L", 4, "float32", parseFloat32},
	// 208 氨气
	{0b010, 0b00011010000}: {"Ammonia", "μL/L", 4, "float32", parseFloat32},
	// 209 可燃气体（浓度）
	{0b010, 0b00011010001}: {"CombustibleGasConcentration", "μL/L", 4, "float32", parseFloat32},
	// 210 有毒气体（浓度）
	{0b010, 0b00011010010}: {"ToxicGasConcentration", "μL/L", 4, "float32", parseFloat32},
	// 211 SF6 气体（浓度）
	{0b010, 0b00011010011}: {"SF6GasConcentration", "μL/L", 4, "float32", parseFloat32},
	// 212 其他气体（浓度）
	{0b010, 0b00011010100}: {"OtherGasConcentration", "μL/L", 4, "float32", parseFloat32},
	//-------------------------------------------------------D.4辅助设施业务状态参量类型表------------------------------------------------
	{0b011, 0b00000000001}: {"ArcFlashIntensity", "mW/cm2", 4, "float32", parseFloat32},
	{0b011, 0b00000000010}: {"Noise", "dB", 4, "float32", parseFloat32},
	{0b011, 0b00000000011}: {"WaterIngressStatus", "", 2, "uint16", parseUint16},
	{0b011, 0b00000000100}: {"WaterLevel", "m", 4, "float32", parseFloat32},
	{0b011, 0b00000000101}: {"Settlement", "mm", 4, "float32", parseFloat32},
	{0b011, 0b00000000110}: {"EquipmentRunningStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000000111}: {"DoorWindowLockStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001000}: {"PerimeterAlarmStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001001}: {"ManholeCoverStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001010}: {"SmokeDetectorStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001011}: {"SwitchControl", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001100}: {"SwitchStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000001101}: {"ACSetTemperature", "℃", 4, "float32", parseFloat32},
	{0b011, 0b00000001110}: {"ACCurrentTemperature", "℃", 4, "float32", parseFloat32},
	{0b011, 0b00000100011}: {"StringVoltage", "V", 4, "float32", parseFloat32},
	{0b011, 0b00000100100}: {"StringCurrent", "A", 4, "float32", parseFloat32},
	{0b011, 0b00000100101}: {"BatteryGroupStatus", "", 1, "uint8", parseUint8},
	{0b011, 0b00000100110}: {"BalanceDegree", "%", 2, "uint16", parseUint16},
	{0b011, 0b00000100111}: {"CellVoltage", "mV", 4, "float32", parseFloat32},
	{0b011, 0b00000101000}: {"CellInternalResistance", "mΩ", 4, "float32", parseFloat32},
	{0b011, 0b00000101001}: {"CellSOC", "%", 2, "uint16", parseUint16},
	{0b011, 0b00000101010}: {"CellSOH", "%", 2, "uint16", parseUint16},
	{0b011, 0b00000101011}: {"CellTemperature", "℃", 4, "float32", parseFloat32},
}

func LookupParamInfo(paramType uint16) (ParamInfo, bool) {
	feature := byte((paramType >> 11) & 0x07)
	code := paramType & 0x7FF
	fmt.Printf("🔍 TypeCode=0x%04X → Feature=%03b (0x%X), Code=%011b (0x%X)\n", paramType, feature, feature, code, code)

	key := ParamKey{feature, code}
	info, ok := paramMap[key]
	return info, ok
}

// ===================== 通用解析函数 =====================

// 通用解析函数
func parseFloat32(data []byte) (any, error) {
	if len(data) != 4 {
		return nil, fmt.Errorf("期望4字节，实际%d", len(data))
	}
	bits := binary.LittleEndian.Uint32(data)
	val := math.Float32frombits(bits)
	return val, nil
}

func parseUint8(data []byte) (any, error) {
	if len(data) != 1 {
		return nil, fmt.Errorf("期望1字节，实际%d", len(data))
	}
	return data[0], nil
}

func parseUint16(data []byte) (any, error) {
	if len(data) != 2 {
		return nil, fmt.Errorf("期望2字节，实际%d", len(data))
	}
	return binary.LittleEndian.Uint16(data), nil
}

func parseUint32(data []byte) (any, error) {
	if len(data) != 4 {
		return nil, fmt.Errorf("期望4字节，实际%d", len(data))
	}
	return binary.LittleEndian.Uint32(data), nil
}

// 读取 4*N 字节为 []float32
func parsefloat32Array(data []byte) (any, error) {
	// 数据长度应为 4*N
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("波形数据长度非4的倍数: %d", len(data))
	}
	n := len(data) / 4
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}

// parseUint16Array 将长度为 2*N 的字节切片解析为 []uint16
func parseUint16Array(data []byte) (any, error) {
	// 数据长度应为 2*N
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("uint16 数组数据长度非2的倍数: %d", len(data))
	}
	n := len(data) / 2
	values := make([]uint16, n)
	for i := 0; i < n; i++ {
		values[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return values, nil
}

// parseInt16 将 2 字节的小端序数据解析为 int16
func parseInt16(data []byte) (any, error) {
	if len(data) != 2 {
		return nil, fmt.Errorf("期望2字节，实际%d", len(data))
	}
	u := binary.LittleEndian.Uint16(data)
	// 将 uint16 按位模式转换为 int16
	val := int16(u)
	return val, nil
}
