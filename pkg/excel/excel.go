package excel

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/doufum/gitlab-analyze/pkg/gitlab"
	"github.com/xuri/excelize/v2"
)

// ProjectInfo 项目信息
type ProjectInfo struct {
	ID               string
	Name             string
	PathWithNamespace string
}

// GetProjectsFromExcel 从 Excel 文件中读取项目信息
func GetProjectsFromExcel(filePath string) ([]ProjectInfo, error) {
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

	// 解析项目信息
	var projects []ProjectInfo
	for i, row := range rows {
		// 跳过表头
		if i == 0 {
			continue
		}
		// 确保行数据完整
		if len(row) >= 3 {
			projects = append(projects, ProjectInfo{
				ID:               row[0],
				Name:             row[1],
				PathWithNamespace: row[2],
			})
		}
	}

	return projects, nil
}

// ExportStatsToCSV 导出统计结果到 CSV 文件
func ExportStatsToCSV(stats map[string]gitlab.UserStats, startDate, endDate string, projects []ProjectInfo) error {
	// 创建输出目录
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 获取当前时间戳
	timestamp := time.Now().Format("20060102_150405")

	// 为每个用户创建独立的统计文件
	for user, stat := range stats {
		// 生成文件名，包含用户名称
		fileName := fmt.Sprintf("gitlab_stats_%s_%s_%s_%s.csv", user, startDate, endDate, timestamp)
		filePath := filepath.Join(outputDir, fileName)

		// 创建 CSV 文件
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("创建 CSV 文件失败: %v", err)
		}
		defer file.Close()

		// 写入 UTF-8 BOM
		file.Write([]byte{0xEF, 0xBB, 0xBF})

		// 创建 CSV writer
		writer := csv.NewWriter(file)
		defer writer.Flush()

		// 写入表头
		header := []string{"用户名", "项目名称", "项目路径", "增加行数", "删除行数", "变更行数", "总代码量"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("写入表头失败: %v", err)
		}

		// 写入用户在每个项目中的统计数据
		for projectID, projectStat := range stat.Projects {
			// 查找项目信息
			var projectName, projectPath string
			for _, project := range projects {
				if project.ID == projectID {
					projectName = project.Name
					projectPath = project.PathWithNamespace
					break
				}
			}

			// 写入项目统计数据
			row := []string{
				user,
				projectName,
				projectPath,
				fmt.Sprintf("%d", projectStat.Additions),
				fmt.Sprintf("%d", projectStat.Deletions),
				fmt.Sprintf("%d", projectStat.Changes),
				fmt.Sprintf("%d", projectStat.Additions+projectStat.Deletions),
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("写入数据失败: %v", err)
			}
		}
	}

	return nil
}