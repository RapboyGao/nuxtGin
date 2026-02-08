package utils

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	re "github.com/dlclark/regexp2"

	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type StringDict map[string]string

/**
 * @param colNameExamples 列名示例，用于确定标题行的位置
 * @param rowData 行数据，用于检查是否包含所有示例列名
 * @return 是否包含所有示例列名
 */
func allExamplesIncluded(colNameExamples []string, rowData []string) bool {
	for _, example := range colNameExamples {
		if !lo.Contains(rowData, example) {
			return false
		}
	}
	return true
}

/*
*

	@param rows 二维字符串数组，每一行表示Excel中的一行数据
	@param colNameExamples 列名示例，用于确定标题行的位置
	@param dataIndexOffset 数据行索引的偏移量，用于确定数据行的位置
	@return 转换后的字符串字典数组
*/
func RowsToDict2(rows [][]string, colNameExamples []string, dataIndexOffset int) ([](StringDict), error) {
	colNames := make([]string, 0)
	colNameIndex := -1
	result := make([]StringDict, 0)
	for i, rowData := range rows {
		if allExamplesIncluded(colNameExamples, rowData) {
			colNames = rowData
			colNameIndex = i
		} else if i > colNameIndex+dataIndexOffset {
			rowResult := StringDict{}
			for j, columnValue := range rowData {
				columnName := colNames[j]
				rowResult[columnName] = columnValue
			}
			result = append(result, rowResult)
		}
	}
	if colNameIndex == -1 {
		return result, errors.New("未找到标题行")
	}
	return result, nil
}

/**
 * @param rows 二维字符串数组，每一行表示Excel中的一行数据
 * @param headerIndex 标题行索引
 * @param dataIndex 数据行索引
 * @return 转换后的字符串字典数组
 */
func RowsToDict1(rows [][]string, headerIndex int, dataIndex int) [](StringDict) {
	rowOfHeader := rows[headerIndex]   // 获取rows的标题行
	var result = make([]StringDict, 0) // 用于返回结果

	for i, rowData := range rows { // 遍历其中每一行
		if i < dataIndex {
			continue // 只有在dataIndex之后才开始运算到result
		}
		var rowResult = StringDict{}          // 该行的结果
		for j, columnValue := range rowData { // 遍历每一列
			columnName := rowOfHeader[j]        // 列名
			rowResult[columnName] = columnValue // 赋值给Dict
		}
		result = append(result, rowResult)
	}
	return result
}

/**
 * @param path Excel文件路径
 * @return 转换后的字符串字典数组
 */
func ReadFirstSheetRaw(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		fmt.Println(err)
		return make([][]string, 0), err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Println(err)
		}
	}()
	// 获取 Sheet1 上所有单元格
	sheets := f.WorkBook.Sheets.Sheet[0]
	rows, err := f.GetRows(sheets.Name)
	return rows, err
}

/**
 * @param path Excel文件路径
 * @param headerIndex 标题行索引
 * @param dataIndex 数据行索引
 * @return 转换后的字符串字典数组
 */
func ReadFirstSheet1(path string, headerIndex int, dataIndex int) ([](StringDict), error) {
	f, err := excelize.OpenFile(path)
	var fakeResult = make([]StringDict, 0) // 用于返回结果
	if err != nil {
		fmt.Println(err)
		return fakeResult, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Println(err)
		}
	}()
	// 获取 Sheet1 上所有单元格
	sheets := f.WorkBook.Sheets.Sheet[0]
	rows, err := f.GetRows(sheets.Name)
	rowsData := RowsToDict1(rows, 0, 1)
	return rowsData, err
}

/**
 * @param path Excel文件路径
 * @param colNameExamples 列名示例，用于确定标题行的位置
 * @param dataIndexOffset 数据行索引的偏移量，用于确定数据行的位置
 * @return 转换后的字符串字典数组
 */
func ReadFirstSheet2(path string, colNameExamples []string, dataIndexOffset int) ([](StringDict), error) {
	f, err := excelize.OpenFile(path)
	var fakeResult = make([]StringDict, 0) // 用于返回结果
	if err != nil {
		fmt.Println(err)
		return fakeResult, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Println(err)
		}
	}()
	// 获取 Sheet1 上所有单元格
	sheets := f.WorkBook.Sheets.Sheet[0]
	rows, err := f.GetRows(sheets.Name)
	rowsData, err := RowsToDict2(rows, colNameExamples, dataIndexOffset)
	return rowsData, err
}

/**
 * @param rows 二维字符串数组，每一行表示Excel中的一行数据
 * @param keywords 关键词数组，用于确定标题行的位置
 * @return 标题行索引、标题行数据、错误信息
 */
func HeaderIndexByKeywords(rows [][]string, keywords []string) (int, []string, error) {
	// 获取 Sheet1 上所有单元格
	for index, row := range rows {
		thisRowSatisfies := lo.EveryBy(keywords, func(keyword string) bool {
			return lo.ContainsBy(row, func(colName string) bool {
				return strings.Contains(colName, keyword)
			})
		})
		if thisRowSatisfies {
			return index, row, nil
		}
	}
	return 0, make([]string, 0), errors.New("未找到合适的行")
}

/**
 * @param number 要转换的数字
 * @param base 转换的基数
 * @return 转换后的结果
 */
func mathLog(number float64, base float64) float64 {
	return math.Log(number) / math.Log(base)
}

/**
 * @param index 要转换的数字
 * @return 转换后的结果
 */
func IntToCol(index int) string {
	num := float64(index)
	howMany := math.Floor(mathLog(num, 26)) // 有多少个 - 1
	chars := make([]string, 0)              // 所有的字符
	result := ""
	for i := howMany; i >= 0; i-- {
		integer := math.Round(num / math.Pow(26, i))
		num = math.Mod(num, math.Round(math.Pow(26, i)))
		chars = append(chars, string(rune(integer+64)))
	}
	for _, char := range chars {
		result += char
	}
	return result
}

/**
 * @param colName 要转换的列名
 * @return 转换后的结果
 */
func ColToInt(colName string) int {
	newStr := strings.ToUpper(colName)
	length := len(newStr)
	res := 0
	for index, char := range newStr {
		charCode := int(char) - 64
		toAdd := charCode * int(math.Pow(26, float64(length)-float64(index)-1))
		res = res + toAdd
	}
	return res
}

/**
 * @param col 要转换的数字
 * @param row 要转换的数字
 * @return 转换后的结果
 */
func Address(col int, row int) string {
	return IntToCol(col) + strconv.FormatInt(int64(row), 10)
}

/**
 * @param address 要解析的地址
 * @return 解析后的结果
 */
func ParseAddress(address string) (int, int, error) {
	col := 0
	row := 0

	// 匹配列名部分
	regexCompiled := re.MustCompile(`[A-Za-z]+`, 0)
	matches := make([]string, 0)
	match, err := regexCompiled.FindStringMatch(address)
	if err != nil {
		return 0, 0, fmt.Errorf("匹配列名时出错: %w", err)
	}
	for match != nil {
		matches = append(matches, match.String())
		match, err = regexCompiled.FindNextMatch(match)
		if err != nil {
			return 0, 0, fmt.Errorf("匹配列名时出错: %w", err)
		}
	}
	if len(matches) == 0 {
		return 0, 0, errors.New("未找到列名部分")
	}
	col = ColToInt(matches[0])

	// 匹配行号部分
	regexCompiled = re.MustCompile(`[0-9]+`, 0)
	matches = make([]string, 0)
	match, err = regexCompiled.FindStringMatch(address)
	if err != nil {
		return 0, 0, fmt.Errorf("匹配行号时出错: %w", err)
	}
	for match != nil {
		matches = append(matches, match.String())
		match, err = regexCompiled.FindNextMatch(match)
		if err != nil {
			return 0, 0, fmt.Errorf("匹配行号时出错: %w", err)
		}
	}
	if len(matches) == 0 {
		return 0, 0, errors.New("未找到行号部分")
	}
	row, err = strconv.Atoi(matches[0])
	if err != nil {
		return 0, 0, fmt.Errorf("转换行号为整数时出错: %w", err)
	}

	return col, row, nil
}

/**
 * @param wb Excel文件对象
 * @param sheet 工作表名称
 * @param colIndex 列索引
 * @param rowIndex 行索引
 * @param values 要写入的值
 * @return 错误数组
 */
func Write1[DataType any](wb *excelize.File, sheet string, colIndex int, rowIndex int, values [][]DataType) []error {
	errs := make([]error, 0)
	for rowPlus, row := range values {
		for colPlus, value := range row {
			address := Address(colPlus+colIndex, rowPlus+rowIndex)
			err := wb.SetCellValue(sheet, address, value)
			errs = append(errs, err)
		}
	}
	return errs
}

/**
 * @param wb Excel文件对象
 * @param sheet 工作表名称
 * @param colName 列名
 * @param rowIndex 行索引
 * @param values 要写入的值
 * @return 错误数组
 */
func Write2[DataType any](wb *excelize.File, sheet string, colName string, rowIndex int, values [][]DataType) []error {
	return Write1(wb, sheet, ColToInt(colName), rowIndex, values)
}

/**
 * @param wb Excel文件对象
 * @param sheet 工作表名称
 * @param start 起始行索引
 * @param end 结束行索引
 */
func ClearRows(wb *excelize.File, sheet string, start int, end int) {
	if start < end {
		for i := start; i < end; i++ {
			wb.RemoveRow(sheet, i)
		}
	} else {
		for i := start; i > end; i-- {
			wb.RemoveRow(sheet, i)
		}
	}

}

/**
 * @param wb Excel文件对象
 * @param sheet 工作表名称
 * @param start 起始行索引
 * @param end 结束行索引
 */
func CopyRowsBetween(wb *excelize.File, sheet string, start int, end int) {
	if start < end {
		for i := start; i < end; i++ {
			wb.DuplicateRowTo(sheet, start, i)
		}
	} else {
		for i := start; i > end; i-- {
			wb.DuplicateRowTo(sheet, start, i)
		}
	}
}
