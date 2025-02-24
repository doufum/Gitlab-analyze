package gitlab

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"sync/atomic"
)

// GitLab API 客户端
type GitLabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// 提交统计信息
type CommitStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

// 提交信息
type Commit struct {
	ID         string      `json:"id"`
	AuthorName string      `json:"author_name"`
	Stats      CommitStats `json:"stats"`
	ParentIDs  []string    `json:"parent_ids"`
	Message    string      `json:"message"`
}

// CommitIdentifier 用于标识相同的提交
type CommitIdentifier struct {
    Message    string
    AuthorName string
    Stats      CommitStats
}

// 项目统计信息
type ProjectStats struct {
	Additions int
	Deletions int
	Changes   int
}

// 用户统计信息
type UserStats struct {
	Additions int
	Deletions int
	Changes   int
	Total     int
	Projects  map[string]ProjectStats
}

// NewGitLabClient 创建新的 GitLab 客户端
func NewGitLabClient() *GitLabClient {
	// 创建自定义的 HTTP 客户端，禁用 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &GitLabClient{
		baseURL:    fmt.Sprintf("%s/api/%s", os.Getenv("GITLAB_URL"), os.Getenv("API_VERSION")),
		token:      os.Getenv("GITLAB_TOKEN"),
		httpClient: &http.Client{Transport: tr},
	}
}

// doRequest 发送 HTTP 请求到 GitLab API
func (c *GitLabClient) doRequest(method, path string, params map[string]string) ([]byte, error) {
	// 构建完整的 URL
	url := c.baseURL + path
	if len(params) > 0 {
		queryParams := make([]string, 0, len(params))
		for k, v := range params {
			queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
		}
		url += "?" + strings.Join(queryParams, "&")
	}

	// 创建请求
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("PRIVATE-TOKEN", c.token)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 请求失败: %s (状态码: %d)", string(body), resp.StatusCode)
	}

	return body, nil
}

// GetProjectCommitStats 获取项目提交统计信息
func (c *GitLabClient) GetProjectCommitStats(projectID, startDate, endDate string) (map[string]UserStats, error) {
    // 用于存储统计结果
    stats := make(map[string]UserStats)
    processedCommits := make(map[string]bool)
    // 用于检测重复提交
    commitSignatures := make(map[CommitIdentifier]bool)

	// 创建工作池
	type commitWork struct {
		message string
		commit Commit
		stats  CommitStats
		err    error
	}

	// 创建通道
	commitChan := make(chan Commit, 100)
	resultChan := make(chan commitWork)
	errChan := make(chan error, 1) // 用于传递致命错误

	// 启动工作协程
	var wg sync.WaitGroup
	workerCount := 10 // 增加并发工作协程数

	// 用于统计进度
	var totalCommits int32
	var processedCount int32

	// 启动工作协程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for commit := range commitChan {
				// 获取提交详情
				detailPath := fmt.Sprintf("/projects/%s/repository/commits/%s", projectID, commit.ID)

				// 添加重试机制
				maxRetries := 5 // 增加最大重试次数
				retryDelay := 1 * time.Second
				var body []byte
				var err error

				for retry := 0; retry < maxRetries; retry++ {
					if retry > 0 {
						time.Sleep(retryDelay)
						retryDelay *= 2 // 指数退避
						fmt.Printf("工作协程 %d: 正在重试获取提交 %s 的详情（第 %d 次重试）\n", workerID, commit.ID[:8], retry)
					}

					body, err = c.doRequest("GET", detailPath, nil)
					if err == nil {
						break
					}
				}

				if err != nil {
					fmt.Printf("工作协程 %d: 获取提交 %s 详情失败: %v\n", workerID, commit.ID[:8], err)
					resultChan <- commitWork{commit: commit, err: err}
					continue
				}

				// 解析提交详情
				var commitDetail Commit
				if err := json.Unmarshal(body, &commitDetail); err != nil {
					fmt.Printf("工作协程 %d: 解析提交 %s 详情失败: %v\n", workerID, commit.ID[:8], err)
					resultChan <- commitWork{commit: commit, err: err}
					continue
				}

				resultChan <- commitWork{commit: commit, stats: commitDetail.Stats}
				// 需要在文件顶部添加 "sync/atomic" 包导入
				// 这里使用 atomic 包来原子递增已处理的提交计数
				atomic.AddInt32(&processedCount, 1)
				
				// 每处理10个提交显示一次进度
				if processedCount%10 == 0 {
					fmt.Printf("进度: %.2f%% (%d/%d)\n", float64(processedCount)/float64(totalCommits)*100, processedCount, totalCommits)
				}
			}
		}(i)
	}

	// 启动结果处理 goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 启动提交获取 goroutine
	go func() {
		defer close(commitChan)

		page := 1
		for {
			params := map[string]string{
				"since":    startDate,
				"until":    endDate,
				"all":      "true",
				"per_page": "100", // 增加每页数量
				"page":     fmt.Sprintf("%d", page),
			}

			// 添加重试机制
			maxRetries := 5 // 增加最大重试次数
			retryDelay := 1 * time.Second
			var body []byte
			var err error

			for retry := 0; retry < maxRetries; retry++ {
				if retry > 0 {
					time.Sleep(retryDelay)
					retryDelay *= 2 // 指数退避
					fmt.Printf("正在重试获取提交列表（第 %d 页，第 %d 次重试）\n", page, retry)
				}

				body, err = c.doRequest("GET", fmt.Sprintf("/projects/%s/repository/commits", projectID), params)
				if err == nil {
					break
				}
			}

			if err != nil {
				fmt.Printf("获取提交列表失败（第 %d 页）: %v\n", page, err)
				errChan <- fmt.Errorf("获取提交列表失败（第 %d 页）: %v", page, err)
				return
			}

			// 添加请求间隔
			time.Sleep(200 * time.Millisecond) // 减少请求间隔时间

			var commits []Commit
			if err := json.Unmarshal(body, &commits); err != nil {
				fmt.Printf("解析提交数据失败（第 %d 页）: %v\n", page, err)
				errChan <- fmt.Errorf("解析提交数据失败（第 %d 页）: %v", page, err)
				return
			}

			if len(commits) == 0 {
				break
			}

			// 更新总提交数
			atomic.AddInt32(&totalCommits, int32(len(commits)))

			// 发送提交到工作通道
			for _, commit := range commits {
				commitChan <- commit
			}

			page++
		}
	}()

	// 在处理结果的部分修改
	for work := range resultChan {
		if work.err != nil {
			continue
		}
	
		commit := work.commit
		
		// 创建提交标识
		identifier := CommitIdentifier{
			Message:    commit.Message,
			AuthorName: commit.AuthorName,
			Stats:      work.stats,
		}
	
		// 检查是否是重复提交
		if commitSignatures[identifier] {
			continue
		}
		commitSignatures[identifier] = true
	
		// 如果是合并提交且已处理过其父提交，则跳过
		if len(commit.ParentIDs) > 1 {
			hasProcessedParent := false
			for _, parentID := range commit.ParentIDs {
				if processedCommits[parentID] {
					hasProcessedParent = true
					break
				}
			}
			if hasProcessedParent {
				continue
			}
		}
	
		// 检查是否已处理过此提交
		if processedCommits[commit.ID] {
			continue
		}
	
		// 记录已处理的提交
		processedCommits[commit.ID] = true
	
		// 更新统计信息
		if _, exists := stats[commit.AuthorName]; !exists {
			stats[commit.AuthorName] = UserStats{
				Projects: make(map[string]ProjectStats),
			}
		}
	
		userStats := stats[commit.AuthorName]
		userStats.Additions += work.stats.Additions
		userStats.Deletions += work.stats.Deletions
		userStats.Changes += work.stats.Total
		userStats.Total += work.stats.Additions + work.stats.Deletions
	
		// 更新项目统计信息
		if _, exists := userStats.Projects[projectID]; !exists {
			userStats.Projects[projectID] = ProjectStats{}
		}
	
		projectStats := userStats.Projects[projectID]
		projectStats.Additions += work.stats.Additions
		projectStats.Deletions += work.stats.Deletions
		projectStats.Changes += work.stats.Total
	
		userStats.Projects[projectID] = projectStats
		stats[commit.AuthorName] = userStats
	}

	// 检查是否有致命错误发生
	select {
	case err := <-errChan:
		return nil, err
	default:
		// 没有错误，继续处理
	}

    return stats, nil
}

// GetProjects 获取项目列表
func (c *GitLabClient) GetProjects(params map[string]string) ([]byte, error) {
	// 设置默认的分页参数
	if _, exists := params["per_page"]; !exists {
		params["per_page"] = "100"
	}
	if _, exists := params["page"]; !exists {
		params["page"] = "1"
	}

	// 添加重试机制
	maxRetries := 5
	retryDelay := 1 * time.Second
	var allProjects []byte
	var err error

	// 循环获取所有页面的数据
	for {
		// 重试机制
		var body []byte
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				time.Sleep(retryDelay)
				retryDelay *= 2 // 指数退避
				fmt.Printf("正在重试获取项目列表（第 %s 页，第 %d 次重试）\n", params["page"], retry)
			}

			body, err = c.doRequest("GET", "/projects", params)
			if err == nil {
				break
			}
		}

		if err != nil {
			return nil, fmt.Errorf("获取项目列表失败（第 %s 页）: %v", params["page"], err)
		}

		// 解析当前页的数据
		var projects []interface{}
		if err := json.Unmarshal(body, &projects); err != nil {
			return nil, fmt.Errorf("解析项目数据失败（第 %s 页）: %v", params["page"], err)
		}

		// 如果是第一页，直接使用当前数据
		if params["page"] == "1" {
			allProjects = body
		} else if len(projects) > 0 {
			// 不是第一页且有数据，则合并到现有结果中
			var existingProjects []interface{}
			if err := json.Unmarshal(allProjects, &existingProjects); err != nil {
				return nil, fmt.Errorf("解析现有项目数据失败: %v", err)
			}
			existingProjects = append(existingProjects, projects...)
			allProjects, err = json.Marshal(existingProjects)
			if err != nil {
				return nil, fmt.Errorf("合并项目数据失败: %v", err)
			}
		}

		// 如果当前页没有数据，说明已经获取完所有数据
		if len(projects) == 0 {
			break
		}

		// 更新页码，准备获取下一页
		currentPage, _ := strconv.Atoi(params["page"])
		params["page"] = strconv.Itoa(currentPage + 1)

		// 添加请求间隔，避免请求过于频繁
		time.Sleep(200 * time.Millisecond)
	}

	return allProjects, nil
}

// 合并多个项目的统计结果
func MergeProjectStats(projectsStats []map[string]UserStats, targetUsers []string) map[string]UserStats {
	mergedStats := make(map[string]UserStats)

	// 创建目标用户映射，用于快速查找
	targetUsersMap := make(map[string]bool)
	if len(targetUsers) > 0 {
		for _, user := range targetUsers {
			targetUsersMap[user] = true
		}
	}

	for _, stats := range projectsStats {
		for author, data := range stats {
			// 如果指定了目标用户且当前作者不在目标用户列表中，则跳过
			if len(targetUsersMap) > 0 && !targetUsersMap[author] {
				continue
			}

			if _, exists := mergedStats[author]; !exists {
				mergedStats[author] = UserStats{
					Projects: make(map[string]ProjectStats),
				}
			}

			userStats := mergedStats[author]
			userStats.Additions += data.Additions
			userStats.Deletions += data.Deletions
			userStats.Changes += data.Changes
			userStats.Total += data.Total
			mergedStats[author] = userStats

			// 合并项目级别的统计数据
			for projectID, projectData := range data.Projects {
				if _, exists := mergedStats[author].Projects[projectID]; !exists {
					mergedStats[author].Projects[projectID] = ProjectStats{}
				}

				projectStats := mergedStats[author].Projects[projectID]
				projectStats.Additions += projectData.Additions
				projectStats.Deletions += projectData.Deletions
				projectStats.Changes += projectData.Changes
				mergedStats[author].Projects[projectID] = projectStats
			}
		}
	}

	return mergedStats
}