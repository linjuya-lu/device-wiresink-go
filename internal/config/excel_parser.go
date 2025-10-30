package config

import (
	"fmt"
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
	{"4", "浮点型"}:     {"float32", parseFloat32},
	{"4", "无符号整型"}:   {"uint32", parseUint32},
	{"2", "无符号整型"}:   {"uint16", parseUint16},
	{"1", "整型"}:      {"uint8", parseUint8},
	{"2", "整型"}:      {"uint16", parseInt16},
	{"2*N", "有符号整型"}: {"uint16", parseUint16Array},
	{"N", "浮点型"}:     {"float32", parsefloat32Array},
}

// 数据类型匹配
func LookupTypeInfo(length string, cn string) (TypeInfo, bool) {
	ti, ok := typeMap[TypeKey{
		Len:    length,
		CNName: cn,
	}]
	return ti, ok
}

// Excel 解析
// 0 标准类型
// 1 所属版本
// 2 参量特征(3位二进制)
// 3 参量类型编码(11位二进制)
// 4 参量名称
// 5 数据类型
// 6 单位
// 7 数据长度
// 8 备注
func LoadParamMapFromExcel(excelPath string) error {
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return fmt.Errorf(" excel open error: %w", err)
	}
	defer f.Close()

	// 第一张表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("excel has no sheets")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("read rows failed: %w", err)
	}

	newParamMap := make(map[ParamKey]ParamInfo)

	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 {
			continue
		}

		// 安全取列的函数，避免越界
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

		// 行不完整跳过
		if featureBits == "" || typeCodeBits == "" || dataTypeCN == "" || dataLenBytes == "" {
			continue
		}

		// 找到解析函数
		ti, ok := LookupTypeInfo(dataLenBytes, dataTypeCN)
		if !ok {
			// 暂时没支持的，就先跳过，不报错
			continue
		}

		// 把 "000" / "011" / "1" / "100001" 这些二进制字符串转成数字
		featureVal, err := parseBinToUint8(featureBits)
		if err != nil {
			return fmt.Errorf("row %d: bad featureBits %q: %w", i+1, featureBits, err)
		}

		typeCodeVal, err := parseBinToUint16(typeCodeBits)
		if err != nil {
			return fmt.Errorf("row %d: bad typeCodeBits %q: %w", i+1, typeCodeBits, err)
		}

		key := ParamKey{
			FeatureBits: featureVal,  // 参量特征(3位二进制)
			CodeBits:    typeCodeVal, // 参量类型编码(11位二进制)
		}

		newParamMap[key] = ParamInfo{
			Parse: ti.Parse, // ← 用 ti.Parse，因为 TypeInfo 里字段名叫 Parse
		}

	}

	// 覆盖全局paramMap
	paramMap = newParamMap
	return nil
}

// -------------------- 小工具函数 --------------------

// "000" -> 0, "011" -> 3, ...
func parseBinToUint8(s string) (uint8, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 2, 8)
	return uint8(v), err
}

// "1" -> 1, "100001" -> 33, ...
func parseBinToUint16(s string) (uint16, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 2, 16)
	return uint16(v), err
}
