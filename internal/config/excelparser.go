package config

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type TypeInfo struct {
	GoType string
	Parse  func([]byte) (any, error)
}

type TypeKey struct {
	Len    string
	CNName string
}

var typeMap = map[TypeKey]TypeInfo{
	{"4", "浮点型"}:        {"float32", parseFloat32},
	{"4", "无符号整型"}:      {"uint32", parseUint32},
	{"2", "无符号整型"}:      {"uint16", parseUint16},
	{"1", "整型"}:         {"uint8", parseUint8},
	{"2", "整型"}:         {"uint16", parseInt16},
	{"2*N", "有符号整型"}:    {"uint16", parseUint16Array},
	{"N", "浮点型"}:        {"float32", parsefloat32Array},
	{"1", "1、正常；2、异常；"}: {"uint8", parseUint8},
}

// 数据类型匹配
func LookupTypeInfo(length string, cn string) (TypeInfo, bool) {
	ti, ok := typeMap[TypeKey{
		Len:    length,
		CNName: cn,
	}]
	return ti, ok
}

// 字符串转整数
func ParseBinToUint8(s string) (uint8, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 2, 8)
	return uint8(v), err
}

func ParseBinToUint16(s string) (uint16, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 2, 16)
	return uint16(v), err
}

// 解析Excel并加载解析表
func LoadParamMapFromReader(r io.Reader, name string) error {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return fmt.Errorf("excel open from %s error: %w", name, err)
	}
	defer f.Close()
	// 第一张表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("excel(%s) has no sheets", name)
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("read rows failed(%s): %w", name, err)
	}
	// 构建解析表
	newParamMap := make(map[ParamKey]ParamInfo)
	for i, row := range rows {
		if i == 0 { // 跳过表头
			continue
		}
		if len(row) == 0 {
			continue
		}
		// 取单元格
		col := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		featureBits := col(2)
		typeCodeBits := col(3)
		dataTypeCN := col(5)
		dataLenBytes := col(7)
		// 列缺失跳过
		if featureBits == "" || typeCodeBits == "" || dataTypeCN == "" || dataLenBytes == "" {
			continue
		}
		// 查找解析函数
		ti, ok := LookupTypeInfo(dataLenBytes, dataTypeCN)
		if !ok {
			fmt.Printf("LoadParamMapFromReader(%s) row=%d: 没有对应数据解析函数 len=%q type=%q\n",
				name, i+1, dataLenBytes, dataTypeCN)
			continue
		}
		featureVal, err := ParseBinToUint8(featureBits)
		if err != nil {
			return fmt.Errorf("row %d: bad featureBits %q: %w", i+1, featureBits, err)
		}
		typeCodeVal, err := ParseBinToUint16(typeCodeBits)
		if err != nil {
			return fmt.Errorf("row %d: bad typeCodeBits %q: %w", i+1, typeCodeBits, err)
		}
		key := ParamKey{
			FeatureBits: featureVal,  // 参量特征(3位二进制)
			CodeBits:    typeCodeVal, // 参量类型编码(11位二进制)
		}
		newParamMap[key] = ParamInfo{
			Parse: ti.Parse,
		}
	}
	//拓扑解析
	defaultKey := ParamKey{
		FeatureBits: 0b000,
		CodeBits:    0b00000010000,
	}
	newParamMap[defaultKey] = ParamInfo{Parse: ParseTopo}
	paramMu.Lock()
	paramMap = newParamMap
	paramMu.Unlock()
	return nil
}
