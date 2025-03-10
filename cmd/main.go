package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/doufum/gitlab-analyze/pkg/gitlab"
	"github.com/doufum/gitlab-analyze/pkg/excel"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	// 命令行参数
	projects    string
	startDate   string
	endDate     string
	projectFile string
)

// 初始化环境变量
func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("警告: 未能加载 .env 文件: %v\n", err)
	}
}

// 主命令
var rootCmd = &cobra.Command{
	Use:   "gitlab-analyze",
	Short: "GitLab 项目成员代码贡献统计工具",
	Long:  `一个用于分析 GitLab 项目成员代码贡献的命令行工具，支持多项目统计和数据导出。`,
}

// analyze 子命令
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "分析项目代码贡献统计",
	Run: func(cmd *cobra.Command, args []string) {
		// 记录开始时间
		startTime := time.Now()

		// 验证日期格式
		if _, err := time.Parse("2006-01-02", startDate); err != nil {
			fmt.Printf("错误: 开始日期格式无效，请使用 YYYY-MM-DD 格式\n")
			os.Exit(1)
		}
		if _, err := time.Parse("2006-01-02", endDate); err != nil {
			fmt.Printf("错误: 结束日期格式无效，请使用 YYYY-MM-DD 格式\n")
			os.Exit(1)
		}

		// 读取项目信息
		fmt.Printf("正在从 %s 读取项目信息...\n", projectFile)
		projectsInfo, err := excel.GetProjectsFromExcel(projectFile)
		if err != nil {
			fmt.Printf("错误: 读取项目信息失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("成功读取 %d 个项目的信息\n", len(projectsInfo))

		// 创建项目信息映射
		projectInfoMap := make(map[string]excel.ProjectInfo)
		for _, info := range projectsInfo {
			projectInfoMap[info.ID] = info
		}

		// 创建 GitLab 客户端
		client := gitlab.NewGitLabClient()

		// 获取项目 ID 列表
		projectIDs := strings.Split(projects, ",")
		var projectsStats []map[string]gitlab.UserStats

		// 显示统计范围信息
		fmt.Printf("\n统计范围:\n")
		fmt.Printf("时间段: %s 至 %s\n", startDate, endDate)
		fmt.Printf("项目数量: %d\n\n", len(projectIDs))

		// 遍历每个项目获取统计信息
		for i, projectID := range projectIDs {
			projectID = strings.TrimSpace(projectID)
			if info, exists := projectInfoMap[projectID]; exists {
				fmt.Printf("[%d/%d] 正在分析项目: %s (%s) [ID: %s]\n", i+1, len(projectIDs), info.Name, info.PathWithNamespace, projectID)
			} else {
				fmt.Printf("[%d/%d] 正在分析项目 ID: %s (项目信息未找到)\n", i+1, len(projectIDs), projectID)
			}

			// 获取项目统计信息
			stats, err := client.GetProjectCommitStats(projectID, startDate, endDate)
			if err != nil {
				fmt.Printf("警告: 获取项目 %s 统计信息失败: %v\n", projectID, err)
				continue
			}
			projectsStats = append(projectsStats, stats)
		}

		// 从环境变量获取目标用户列表
		targetUsers := []string{}
		if targetUsersStr := os.Getenv("TARGET_USERS"); targetUsersStr != "" {
			targetUsers = strings.Split(targetUsersStr, ",")
			for i := range targetUsers {
				targetUsers[i] = strings.TrimSpace(targetUsers[i])
			}
		}

		// 合并所有项目的统计结果
		fmt.Printf("\n正在合并统计结果...\n")
		if len(targetUsers) > 0 {
			fmt.Printf("将只统计以下用户: %s\n", strings.Join(targetUsers, ", "))
		}
		mergedStats := gitlab.MergeProjectStats(projectsStats, targetUsers)

		// 导出统计结果
		fmt.Printf("正在导出统计结果...\n")
		if err := excel.ExportStatsToCSV(mergedStats, startDate, endDate, projectsInfo); err != nil {
			fmt.Printf("错误: 导出统计结果失败: %v\n", err)
			os.Exit(1)
		}

		// 计算并打印总耗时
		elapsed := time.Since(startTime)
		fmt.Printf("\n统计分析完成！总耗时: %s\n", elapsed)
		fmt.Printf("统计结果已保存到 output 目录\n")
	},
}

// list 子命令
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "显示所有可用的项目列表",
	Run: func(cmd *cobra.Command, args []string) {
		// 创建 GitLab 客户端
		client := gitlab.NewGitLabClient()

		// 获取项目列表
		fmt.Println("正在获取项目列表...")
		params := map[string]string{
			"membership": "true",
			"per_page":  "100",
		}

		body, err := client.GetProjects(params)
		if err != nil {
			fmt.Printf("错误: 获取项目列表失败: %v\n", err)
			os.Exit(1)
		}

		// 解析项目列表
		var projects []struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			Description       string `json:"description"`
		}
		if err := json.Unmarshal(body, &projects); err != nil {
			fmt.Printf("错误: 解析项目列表失败: %v\n", err)
			os.Exit(1)
		}


		// 打印项目列表
		fmt.Printf("\n找到 %d 个项目:\n\n", len(projects))
		fmt.Printf("%-10s %-30s %-50s %s\n", "ID", "名称", "路径", "描述")
		fmt.Println(strings.Repeat("-", 120))

		for _, project := range projects {
			// 截断过长的字段
			name := truncateString(project.Name, 28)
			path := truncateString(project.PathWithNamespace, 48)
			desc := truncateString(project.Description, 30)

			fmt.Printf("%-10d %-30s %-50s %s\n", project.ID, name, path, desc)
		}
		fmt.Println()
	},
}

func init() {
	// 添加子命令
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(listCmd)

	// 设置 analyze 命令的参数
	analyzeCmd.Flags().StringVarP(&projects, "projects", "p", os.Getenv("DEFAULT_PROJECTS"), "要分析的项目 ID 列表，用逗号分隔")
	analyzeCmd.Flags().StringVarP(&startDate, "start-date", "s", os.Getenv("DEFAULT_START_DATE"), "统计开始日期 (YYYY-MM-DD)")
	analyzeCmd.Flags().StringVarP(&endDate, "end-date", "e", os.Getenv("DEFAULT_END_DATE"), "统计结束日期 (YYYY-MM-DD)")
	analyzeCmd.Flags().StringVarP(&projectFile, "file", "f", os.Getenv("DEFAULT_PROJECT_FILE"), "项目信息 Excel 文件路径")

	// 所有参数都有默认值，不需要标记为必需
}

// truncateString 截断过长的字符串并添加省略号
func truncateString(s string, maxLen int) string {
	runeStr := []rune(s)
	if len(runeStr) <= maxLen {
		return s
	}
	return string(runeStr[:maxLen-3]) + "..."
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}