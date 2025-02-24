# Gitlab-analyze

一个用于分析 GitLab 项目成员代码贡献的命令行工具，支持多项目统计和数据导出。

## 功能特性

- 支持多项目批量统计
- 按时间范围统计代码贡献
- 统计每个用户的代码行数变更（新增、修改、删除）
- 支持项目级别的统计数据
- 自动过滤重复提交和合并提交
- 支持导出统计结果到 CSV 文件
- 支持从 Excel 文件导入项目信息
- 并发处理提高统计效率
- 支持失败重试和错误恢复

## 快速开始
1. 克隆仓库到本地：
```bash
git clone git clone github.com/doufum/gitlab-analyze.git
```
2. 进入项目目录：
```bash
cd gitlab-analyze
```
3. 编译项目：
```bash
go build -o gitlab-analyze cmd/main.go
```
4. 将编译后的二进制文件 `gitlab-analyze` 移动到系统可执行路径或自定义路径。

5. 设置环境变量（可选）：
```bash
export GITLAB_URL=gitlab-instance.com
export GITLAB_TOKEN=your-gitlab-token
```

6. 运行统计分析：
```bash
gitlab-analyze analyze
```

## 环境要求

- Go 1.21 或更高版本
- GitLab 实例（支持自托管或 GitLab.com）
- GitLab API Token（需要项目访问权限）


## 配置

在项目根目录创建 `.env` 文件，配置以下环境变量：

```env
# GitLab 配置
GITLAB_URL=https://your-gitlab-instance.com    # GitLab 实例地址
GITLAB_TOKEN=your-gitlab-token                  # GitLab API Token
API_VERSION=v4                                  # GitLab API 版本

# 默认配置（可选）
DEFAULT_PROJECTS=project1,project2             # 默认统计的项目 ID
DEFAULT_START_DATE=2023-01-01                  # 默认开始日期
DEFAULT_END_DATE=2023-12-31                    # 默认结束日期
DEFAULT_PROJECT_FILE=projects.xlsx             # 默认项目信息文件

# 目标用户（可选）
TARGET_USERS=user1,user2                       # 指定统计的目标用户
```

## 使用说明

### 1. 准备项目信息文件

创建一个 Excel 文件（默认名称为 `projects.xlsx`），包含以下列：
- 项目名称
- 项目ID
- 项目分组
- 项目路径

### 2. 运行统计分析

```bash
# 使用默认配置运行统计
gitlab-analyze analyze

# 指定参数运行统计
gitlab-analyze analyze \
  -p "project1,project2" \
  -s "2023-01-01" \
  -e "2023-12-31" \
  -f "projects.xlsx"
```

### 参数说明

- `-p, --projects`: 要分析的项目 ID 列表，用逗号分隔
- `-s, --start-date`: 统计开始日期（YYYY-MM-DD）
- `-e, --end-date`: 统计结束日期（YYYY-MM-DD）
- `-f, --file`: 项目信息 Excel 文件路径

## 实现细节

### 核心功能

1. **项目信息管理**
   - 从 Excel 文件读取项目基本信息
   - 支持批量项目处理

2. **提交统计**
   - 使用 GitLab API 获取提交历史
   - 支持分页获取大量提交数据
   - 自动过滤重复提交和合并提交
   - 并发处理提高效率

3. **数据导出**
   - 按用户分别导出统计结果
   - 支持导出到 CSV 格式
   - 包含详细的代码变更统计

### 性能优化

1. **并发处理**
   - 使用 goroutine 池并发处理提交统计
   - 通过通道（channel）协调工作
   - 支持自定义并发数量

2. **错误处理**
   - 实现指数退避重试机制
   - 优雅处理 API 限流和网络错误
   - 支持断点续传

3. **内存优化**
   - 分批处理大量数据
   - 及时释放不需要的资源
   - 避免内存泄漏

## 输出结果

统计结果将保存在 `output` 目录下，每个用户一个 CSV 文件，包含以下信息：
- 项目名称
- 项目路径
- 新增行数
- 修改行数
- 删除行数

## 注意事项

1. 确保 GitLab API Token 具有足够的权限访问目标项目
2. 大型项目的统计可能需要较长时间
3. 建议合理设置时间范围，避免请求次数过多
4. 统计结果不包含二进制文件的变更