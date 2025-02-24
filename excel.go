package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 项目信息结构
type ProjectInfo struct {
	ID              string
	Name            string
	Namespace       string
	PathWithNamespace string
}

// 从 Excel 文件读取项目信息
func GetProjectsFromExcel(filePath string) ([]ProjectInfo, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Excel 文件不存在: %s", filePath)
	}

	// 打开 Excel 文件
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开 Excel 文件失败: %v", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取工作表失败: %v", err)
	}

	if len(rows) < 2 { // 至少需要标题行和一行数据
		return nil, fmt.Errorf("Excel 文件为空或格式不正确")
	}

	// 验证表头
	headers := rows[0]
	columnMap := make(map[string]int)
	requiredColumns := []string{"项目名称", "项目ID", "项目分组", "项目路径"}

	for i, header := range headers {
		columnMap[header] = i
	}

	// 检查必要的列是否存在
	for _, col := range requiredColumns {
		if _, exists := columnMap[col]; !exists {
			return nil, fmt.Errorf("Excel 文件缺少必要的列: %s", col)
		}
	}

	// 解析项目信息
	var projects []ProjectInfo
	for _, row := range rows[1:] { // 跳过标题行
		if len(row) < len(headers) {
			continue // 跳过不完整的行
		}

		project := ProjectInfo{
			ID:              row[columnMap["项目ID"]],
			Name:            row[columnMap["项目名称"]],
			Namespace:       row[columnMap["项目分组"]],
			PathWithNamespace: row[columnMap["项目路径"]],
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// 导出统计结果到 CSV 文件
func ExportStatsToCSV(stats map[string]UserStats, startDate, endDate string, projectsInfo []ProjectInfo) error {
	// 获取目标用户列表
	targetUsers := []string{}
	if targetUsersEnv := os.Getenv("TARGET_USERS"); targetUsersEnv != "" {
		targetUsers = strings.Split(targetUsersEnv, ",")
		for i := range targetUsers {
			targetUsers[i] = strings.TrimSpace(targetUsers[i])
		}
	}

	// 创建输出目录
	outputDir := filepath.Join("output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 为每个用户创建 CSV 文件
	for author, data := range stats {
		// 如果指定了目标用户且当前用户不在目标列表中，则跳过
		if len(targetUsers) > 0 && !contains(targetUsers, author) {
			continue
		}

		// 创建 CSV 文件
		fileName := fmt.Sprintf("%s_stats_%s_to_%s.csv", author, startDate, endDate)
		filePath := filepath.Join(outputDir, fileName)

		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("创建 CSV 文件失败: %v", err)
		}
		defer file.Close()

		// 创建 CSV writer
		writer := csv.NewWriter(file)
		defer writer.Flush()

		// 写入表头
		if err := writer.Write([]string{"项目名称", "项目路径", "新增行数", "修改行数", "删除行数"}); err != nil {
			return fmt.Errorf("写入 CSV 表头失败: %v", err)
		}

		// 写入项目统计数据
		for projectID, projectStats := range data.Projects {
			// 查找项目信息
			var projectInfo ProjectInfo
			for _, p := range projectsInfo {
				if p.ID == projectID {
					projectInfo = p
					break
				}
			}

			// 写入一行数据
			row := []string{
				projectInfo.Name,
				projectInfo.PathWithNamespace,
				fmt.Sprintf("%d", projectStats.Additions),
				fmt.Sprintf("%d", projectStats.Changes),
				fmt.Sprintf("%d", projectStats.Deletions),
			}

			if err := writer.Write(row); err != nil {
				return fmt.Errorf("写入 CSV 数据失败: %v", err)
			}
		}

		fmt.Printf("已导出 %s 的统计数据到: %s\n", author, filePath)
	}

	return nil
}

// 辅助函数：检查字符串是否在切片中
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}