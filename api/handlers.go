package api

import (
	"PaiDownloader/download"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 全局变量声明
var downloader *download.ResourceDownloader    // 下载器实例 负责处理资源下载任务
var downloadChannel chan download.DownloadTask // 下载任务的通道 用于向工作协程分发任务
var progress *download.Progress                // 记录下载任务的进度信息
var taskStatuses []download.TaskStatus         // 存储每个下载任务的状态
var taskStatusLock sync.Mutex                  // 保护 taskStatuses 的并发访问
var downloadHistory []DownloadHistory          // 存储下载历史记录
var historyLock sync.Mutex                     // 保护 downloadHistory 的并发访问
var historyFilePath = "download_history.json"  // 下载历史记录文件的路径

// init 函数在包被加载时执行，用于初始化下载器、日志文件、下载目录和加载历史记录
func init() {
	// 初始化下载器，设置默认参数
	downloader = &download.ResourceDownloader{
		OutputDir:     "./download_data",
		MaxConcurrent: 5, // 最大并发下载任务数
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		RetryTimes:    3,
		Timeout:       30 * time.Second,
	}

	// 打开或创建日志文件，用于记录下载过程中的信息
	logFile, err := os.OpenFile("download.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("创建日志文件失败: %v", err))
	}
	downloader.LogFile = logFile

	// 创建下载目录，如果目录已存在则不做处理
	if err := os.MkdirAll(downloader.OutputDir, 0755); err != nil {
		panic(fmt.Sprintf("创建下载目录失败: %v", err))
	}

	// 加载下载历史记录
	loadDownloadHistory()
}

// APIResponse 定义 API 响应的结构体，包含状态码、消息和数据
type APIResponse struct {
代码int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// 历史记录结构体
// DownloadHistory 存储每次下载任务的详细信息
type DownloadHistory struct {
网站string    `json:"url"`
	Filename     string    `json:"filename"`
类型string    `json:"type"`
	Size         int64     `json:"size"`
状态string    `json:"status"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	RetryCount   int       `json:"retry_count"`
	LastModified time.Time `json:"last_modified"`
	ID           string    `json:"id"`
	FileTypes    []string  `json:"file_types"`
	Total        int       `json:"total"`
	Completed    int       `json:"completed"`
	Failed       int       `json:"failed"`
}

// HandleDownloadRequest 处理下载请求，解析请求参数，初始化下载任务并启动下载
func HandleDownloadRequest(c *gin.Context) {
	// 定义请求结构体，用于绑定请求的 JSON 数据
	var请求struct {
网站string   `json:"url" binding:"required"`
		FileTypes []string `json:"file_types"`
		OutputDir string   `json:"output_dir"`
	}

	// 绑定请求的 JSON 数据到 request 结构体
	if err := c.ShouldBindJSON(&request); err != nil {
		// 若绑定失败，返回错误响应
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数",
			Data:    err.Error(),
		})
		return
	}

	// 解析请求的 URL，检查 URL 格式是否有效
	parsedURL, err := url.Parse(request.URL)
	if err != nil || parsedURL.Scheme == "" {
		// 若 URL 格式无效，返回错误响应
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的URL格式",
		})
		return
	}

	// 设置下载器的基础 URL
	downloader.BaseURL = parsedURL

	// 如果请求中指定了输出目录，创建该目录并更新下载器的输出目录
	if请求.OutputDir != "" {
		if err := os.MkdirAll(request.OutputDir, 0755); err != nil {
			// 若创建目录失败，返回错误响应
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "创建输出目录失败",
				Data:    err.Error(),
			})
			return
		}
		downloader.OutputDir = request.OutputDir
	}

	// 如果请求中指定了文件类型，更新下载器的文件类型列表；否则使用默认列表
	if len(request.FileTypes) > 0 {
		downloader.FileTypes = request.FileTypes
	} else {
		downloader.FileTypes = []string{
			"image", "script", "style", "video", "audio",
			"font", "document", "archive", "html", "data",
		}
	}

	// 如果下载通道未初始化，创建下载通道并启动工作协程
	if downloadChannel == nil {
		createDownloadChannel()
	}

	// 获取网页内容
	htmlContent, err := downloader.FetchHTML()
	if err != nil {
		// 若获取网页内容失败，返回错误响应
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "获取网页内容失败",
			Data:    err.Error(),
		})
		return
	}

	// 从网页内容中提取可下载的资源任务
	tasks, err := downloader.ExtractResources(htmlContent)
	if err != nil {
		// 若分析资源失败，返回错误响应
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "分析资源失败",
			Data:    err.Error(),
		})
		return
	}

	// 如果未找到可下载的资源，返回相应响应
	if len(tasks) == 0 {
		c.JSON(http.StatusOK, APIResponse{
			Code:    200,
			Message: "未找到可下载的资源",
		})
		return
	}

	// 初始化下载进度信息
	progress = &download.Progress{
		Total:     len(tasks),
		StartTime: time.Now(),
	}
	taskStatuses = make([]download.TaskStatus, len(tasks))

	// 生成唯一 ID 并初始化历史记录条目
	historyID := uuid.New().String()
	newHistory := DownloadHistory{
		ID:        historyID,
		URL:       request.URL,
		FileTypes: request.FileTypes,
		StartTime: time.Now(),
		EndTime:   time.Time{},
		Total:     len(tasks),
		Completed: 0,
		Failed:    0,
		Status:    "in_progress",
	}

	// 将新的历史记录条目添加到下载历史记录中
	historyLock.Lock()
	downloadHistory = append(downloadHistory, newHistory)
	historyLock.Unlock()

	// 初始化每个任务的状态并将任务发送到下载通道
	for i, task := range tasks {
		taskStatuses[i] = download.TaskStatus{
			URL:      task.URL,
			Filename: task.Filename,
			Type:     task.Type,
			Status:   "pending",
		}
		downloadChannel <- task
	}

	// 返回成功响应，告知客户端下载任务已开始
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "下载任务已开始",
		Data: map[string]interface{}{
			"total_tasks": len(tasks),
			"started_at":  progress.StartTime.Format(time.RFC3339),
		},
	})
}

// HandleProgressRequest 处理下载进度请求，返回当前下载任务的进度信息
func HandleProgressRequest(c *gin.Context) {
	// 初始化下载进度结果结构体
	var result download.DownloadProgress

	// 填充下载进度信息
	result.Total = progress.Total
	result.Completed = progress.Completed
	result.Failed = progress.Failed
	result.Skipped = progress.Skipped
	result.Duration = fmt.Sprintf("%f", time.Since(progress.StartTime).Seconds())
	result.Rate = float64(progress.Completed) / time.Since(progress.StartTime).Seconds()
	result.Tasks = taskStatuses

	// 返回下载进度响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "下载进度",
		Data:    result,
	})
}

// HandleCancelRequest 处理取消下载请求，关闭下载通道以停止所有下载任务
func HandleCancelRequest(c *gin.Context) {
	// 如果下载通道存在，关闭通道并置为 nil
	if downloadChannel != nil {
		close(downloadChannel)
		downloadChannel = nil
	}

	// 返回取消成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "下载已取消",
	})
}

// createDownloadChannel 创建下载通道并启动工作协程来处理下载任务
func createDownloadChannel() {
	// 创建一个带缓冲的下载任务通道
	downloadChannel = make(chan download.DownloadTask, 100)
	// 启动工作协程
	go startDownloadWorkers()
}

// startDownloadWorkers 启动多个工作协程来处理下载通道中的任务
func startDownloadWorkers() {
	// 使用 sync.WaitGroup 来等待所有工作协程完成
	var wg sync.WaitGroup
	for i := 0; i < downloader.MaxConcurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// 从下载通道中获取任务并处理
			for task := range downloadChannel {
				fmt.Printf("Worker %d 开始处理任务: %s\n", workerID, task.URL)
				// 调用下载函数并处理重试逻辑
				download.DownloadWithRetry(task, downloader, progress, &taskStatuses)
				fmt.Printf("Worker %d 完成任务: %s\n", workerID, task.URL)
			}
		}(i)
	}
	// 等待所有工作协程完成
	wg.Wait()
}

// HandleIndexPage 处理首页请求，返回静态 HTML 页面
func HandleIndexPage(c *gin.Context) {
	// 设置响应的 Content-Type 为 HTML
	c.Header("Content-Type", "text/html; charset=utf-8")

	// 读取静态 HTML 文件
	indexHTML, err := os.ReadFile("static/index.html")
	if err != nil {
		// 若读取失败，返回错误信息
		c.String(http.StatusInternalServerError, "Failed to load index page")
		return
	}

	// 返回 HTML 页面内容
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
}

// HandlePreviewRequest 处理资源预览请求，解析请求参数，获取网页内容并提取可下载的资源任务
func HandlePreviewRequest(c *gin.Context) {
	// 定义请求结构体，用于绑定请求的 JSON 数据
	var请求struct {
网站string   `json:"url" binding:"required"`
		FileTypes []string `json:"file_types"`
	}

	// 绑定请求的 JSON 数据到 request 结构体
	if err := c.ShouldBindJSON(&request); err != nil {
		// 若绑定失败，返回错误响应
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数",
			Data:    err.Error(),
		})
		return
	}

	// 解析请求的 URL，检查 URL 格式是否有效
	url, err := url.Parse(request.URL)
	if err != nil || url.Scheme == "" {
		// 若 URL 格式无效，返回错误响应
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的URL格式",
		})
		return
	}

	// 设置下载器的基础 URL
	downloader.BaseURL = url

	// 如果请求中指定了文件类型，更新下载器的文件类型列表；否则使用默认列表
	if len(request.FileTypes) > 0 {
		downloader.FileTypes = request.FileTypes
	} else {
		downloader.FileTypes = []string{
			"image", "script", "style", "video", "audio",
			"font", "document", "archive", "html", "data",
		}
	}

	// 获取网页内容
	htmlContent, err := downloader.FetchHTML()
	if err != nil {
		// 若获取网页内容失败，返回错误响应
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "获取网页内容失败",
			Data:    err.Error(),
		})
		return
	}

	// 从网页内容中提取可下载的资源任务
	tasks, err := downloader.ExtractResources(htmlContent)
	if err != nil {
		// 若分析资源失败，返回错误响应
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "分析资源失败",
			Data:    err.Error(),
		})
		return
	}

	// 如果未找到可下载的资源，返回相应响应
	if len(tasks) == 0 {
		c.JSON(http.StatusOK, APIResponse{
			Code:    200,
			Message: "未找到可下载的资源",
			Data:    nil,
		})
		return
	}

	// 整理预览任务信息
	previewTasks := make([]map[string]interface{}, 0)
	for _, task := range tasks {
		previewTasks = append(previewTasks, map[string]interface{}{
			"url":      task.URL,
			"filename": task.Filename,
			"type":     task.Type,
			"size":     task.Size,
		})
	}

	// 返回资源预览成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "资源预览成功",
		Data: map[string]interface{}{
			"tasks": previewTasks,
		},
	})
}

// HandleProgressSSE 处理 SSE 实时推送请求，定期向客户端发送下载进度信息
func HandleProgressSSE(c *gin.Context) {
	// 设置响应的 Content-Type 为 event-stream
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// 创建一个定时器，每秒触发一次
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done():
			// 若客户端断开连接，退出循环
			return
		case <-ticker.C:
			// 加锁保护 taskStatuses 的并发访问
			download.TaskStatusLock.Lock()
			var result download.DownloadProgress
			// 填充下载进度信息
			result.Total = progress.Total
			result.Completed = progress.Completed
			result.Failed = progress.Failed
			result.Skipped = progress.Skipped
			result.Duration = fmt.Sprintf("%f", time.Since(progress.StartTime).Seconds())
			result.Rate = float64(result.Completed) / time.Since(progress.StartTime).Seconds()
			result.Tasks = taskStatuses
			download.TaskStatusLock.Unlock()

			// 将下载进度信息转换为 JSON 格式
			data, _ := json.Marshal(result)
			// 发送 SSE 事件
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			// 刷新响应缓冲区
			c.Writer.Flush()
		}
	}
}

// loadDownloadHistory 从文件中加载下载历史记录
func loadDownloadHistory() {
	// 读取历史记录文件
	data, err := os.ReadFile(historyFilePath)
	if err != nil {
		// 若读取失败，打印错误信息
		fmt.Printf("读取历史记录文件失败: %v\n", err)
		return
	}
	// 将文件内容解析到 downloadHistory 切片中
	if err := json.Unmarshal(data, &downloadHistory); err != nil {
		// 若解析失败，打印错误信息
		fmt.Printf("解析历史记录文件失败: %v\n", err)
	}
}

// saveDownloadHistory 将下载历史记录保存到文件中
func saveDownloadHistory() error {
	data, err := json.Marshal(downloadHistory)
	if err != nil {
		return err
	}
	return os.WriteFile(historyFilePath, data, 0644)
}

// HandleGetHistory 处理获取下载历史记录的请求，返回下载历史记录信息
func HandleGetHistory(c *gin.Context) {
	// 加锁保护 downloadHistory 的并发访问
	historyLock.Lock()
	defer historyLock.Unlock()

	// 如果没有历史记录，返回相应响应
	if len(downloadHistory) == 0 {
		// 可以根据实际情况调整响应内容
		c.JSON(http.StatusOK, APIResponse{
			Code:    200,
			Message: "暂无历史记录",
			Data:    nil,
		})
		return
	}

	// 返回下载历史记录信息
	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "Success",
		Data:    downloadHistory,
	})
}

// HandleHistoryPage 处理历史记录页面请求，返回静态 HTML 页面
func HandleHistoryPage(c *gin.Context) {
	// 设置响应的 Content-Type 为 HTML
	c.Header("Content-Type", "text/html; charset=utf-8")

	// 读取静态 HTML 文件
	indexHTML, err := os.ReadFile("static/history.html")
	if err != nil {
		// 若读取失败，返回错误信息
		c.String(http.StatusInternalServerError, "Failed to load index page")
		return
	}

	// 返回 HTML 页面内容
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
}

// updateDownloadHistory 更新指定 ID 的下载历史记录信息，并保存到文件中
func updateDownloadHistory(historyID string, completed, failed int, status string) {
	historyLock.Lock()
	defer historyLock.Unlock()

	for i, history := range downloadHistory {
		if history.ID == historyID {
			downloadHistory[i].Completed = completed
			downloadHistory[i].Failed = failed
			downloadHistory[i].Status = status
			if status == "completed" || status == "failed" {
				downloadHistory[i].EndTime = time.Now()
			}
			if err := saveDownloadHistory(); err != nil {
				_, logErr := fmt.Fprintf(downloader.LogFile, "保存历史记录失败: %v\n", err)
				if logErr != nil {
					fmt.Printf("写入日志文件失败: %v\n", logErr)
				}
			}
			break
		}
	}
}
