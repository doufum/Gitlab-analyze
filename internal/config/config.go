package config

import (
	"os"
	"time"
)

// Config 应用配置
type Config struct {
	GitLabURL   string
	GitLabToken string
	APIVersion  string

	// 默认配置
	DefaultProjects   string
	DefaultStartDate string
	DefaultEndDate   string
	DefaultFile      string

	// 目标用户
	TargetUsers string
}

// LoadConfig 加载配置
func LoadConfig() *Config {
	// 获取当前时间
	now := time.Now()

	// 默认的开始日期为当前月份的第一天
	defaultStartDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	// 默认的结束日期为当前日期
	defaultEndDate := now.Format("2006-01-02")

	return &Config{
		GitLabURL:        os.Getenv("GITLAB_URL"),
		GitLabToken:      os.Getenv("GITLAB_TOKEN"),
		APIVersion:       getEnvOrDefault("API_VERSION", "v4"),
		DefaultProjects:  os.Getenv("DEFAULT_PROJECTS"),
		DefaultStartDate: getEnvOrDefault("DEFAULT_START_DATE", defaultStartDate),
		DefaultEndDate:   getEnvOrDefault("DEFAULT_END_DATE", defaultEndDate),
		DefaultFile:      getEnvOrDefault("DEFAULT_PROJECT_FILE", "projects.xlsx"),
		TargetUsers:      os.Getenv("TARGET_USERS"),
	}
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}