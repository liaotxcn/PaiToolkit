package主干

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ResourceDownloader 用于从网页下载资源
type ResourceDownloader struct {
	BaseURL       *url.网站// 目标网页的URL
	OutputDir     string   // 下载文件的存放目录
	MaxConcurrent int      // 最大并发下载数
	FileTypes     []string // 允许下载的文件类型
	Client        *http.Client
	LogFile       *os.File // 日志文件
}

// DownloadTask 下载任务
type DownloadTask struct {
网站string // 资源的URL
类型string // 资源的类型
	Filename string // 资源的文件名
	Size     int64  // 资源的大小
}

// Progress 用于跟踪下载进度
type Progress struct {
	Total     int // 总任务数
	Completed int // 已完成任务数
	Lock      sync.Mutex
}

// fileExtensions 定义了不同资源类型对应的文件扩展名
var fileExtensions = map[string][]string{
	"image":      []string{"jpg", "jpeg", "png", "gif", "bmp", "svg", "webp", "ico"},
	"script":     []string{"js"},
	"stylesheet": []string{"css"},
	"video":      []string{"mp4", "webm", "ogg", "flv", "avi", "mov", "wmv", "mkv"},
	"audio":      []string{"mp3", "wav", "ogg", "aac", "flac", "m4a", "wma"},
	"font":       []string{"woff", "woff2", "ttf", "otf", "eot", "svg"},
	"document":   []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt"},
}

func main() {
	// 目标网站URL
	webpageURL := "https://xxx.com"
	// 下载文件的存放目录
	outputDir := "./download_data"
	// 最大并发下载数
	maxConcurrent := 5

	// 创建下载器实例
	downloader, err := NewResourceDownloader(webpageURL, outputDir, maxConcurrent)
	if err != nil {
		fmt.Printf("创建下载器失败: %v\n", err)
		return
	}

	// 设置允许下载的文件类型
	downloader.FileTypes = []string{
		"image", "script", "stylesheet", "video", "audio", "font", "document",
	}

	fmt.Printf("开始分析 %s 中的资源...\n", webpageURL)

	// 获取网页内容
	htmlContent, err := downloader.fetchHTML()
	if err != nil {
		fmt.Printf("获取网页内容失败: %v\n", err)
		return
	}

	// 从HTML中提取资源链接
	tasks, err := downloader.extractResources(htmlContent)
	if err != nil {
		fmt.Printf("分析资源失败: %v\n", err)
		return
	}

	if len(tasks) == 0 {
		fmt.Println("未找到可下载的资源")
		return
	}

	// 显示嗅探到的资源信息
	fmt.Println("\n嗅探到的资源信息:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-40s %-15s %-15s %s\n", "URL", "类型", "大小", "保存路径")
	fmt.Println(strings.Repeat("=", 80))

	for _, task := range tasks {
		sizeStr := "未知"
		if task.Size > 0 {
			sizeStr = formatFileSize(task.Size)
		}
		savePath := filepath.Join(downloader.OutputDir, task.Type, task.Filename)
		fmt.Printf("%-40s %-15s %-15s %s\n",
			shortenURL(task.URL, 40),
			task.Type,
			sizeStr,
			savePath)
	}
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("共找到 %d 个资源\n\n", len(tasks))

	// 确认是否开始下载
	fmt.Print("是否开始下载这些资源? (y/n): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("下载已取消")
		return
	}

	fmt.Println("\n开始下载资源...")
	if err := downloader.downloadTasks(tasks); err != nil {
		fmt.Printf("\n下载失败: %v\n", err)
	} else {
		fmt.Println("\n所有资源下载完成!")
	}
}

// NewResourceDownloader 创建一个新的资源下载器
func NewResourceDownloader(rawURL, outputDir string, maxConcurrent int) (*ResourceDownloader, error) {
	// 解析URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("解析URL失败: %v", err)
	}

	// 如果URL没有指定协议，默认使用HTTP
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 创建日志文件

	logFilePath := "F:\\Person\\库\\技术文档\\gofile\\go\\github仓库\\PaiToolkit\\Downloader\\logs\\download.log"
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}

	// 配置HTTP客户端
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	return &ResourceDownloader{
		BaseURL:       parsedURL,
		OutputDir:     outputDir,
		MaxConcurrent: maxConcurrent,
		Client:        client,
		LogFile:       logFile,
	}, nil
}

// fetchHTML 获取网页内容
func (d *ResourceDownloader) fetchHTML() (string, error) {
	resp, err := d.Client.Get(d.BaseURL.String())
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("fetchHTML失败: %v", err))
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logError(d.LogFile, fmt.Sprintf("HTTP状态码错误: %d", resp.StatusCode))
		return "", fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("读取响应体失败: %v", err))
		return "", err
	}

	logInfo(d.LogFile, fmt.Sprintf("成功获取HTML内容，大小: %d字节", len(content)))
	return string(content), nil
}

// extractResources 从HTML中提取资源链接
func (d *ResourceDownloader) extractResources(htmlContent string) ([]DownloadTask, error) {
	var tasks []DownloadTask

	// 解析HTML
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("HTML解析失败: %v", err))
		return nil, err
	}

	// 遍历HTML节点
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var resourceURL string
			var resourceType string

			// 根据HTML元素提取资源链接
			switch n.Data {
			case "img":
				resourceURL = getAttr(n, "src")
				resourceType = "image"
			case "script":
				resourceURL = getAttr(n, "src")
				resourceType = "script"
			case "link":
				if getAttr(n, "rel") == "stylesheet" {
					resourceURL = getAttr(n, "href")
					resourceType = "stylesheet"
				} else if strings.Contains(getAttr(n, "rel"), "icon") {
					resourceURL = getAttr(n, "href")
					resourceType = "image"
				}
			case "video", "audio":
				resourceURL = getAttr(n, "src")
				resourceType = n.Data
				if resourceURL == "" {
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Data == "source" {
							resourceURL = getAttr(c, "src")
							break
						}
					}
				}
			case "source":
				resourceURL = getAttr(n, "src")
				parent := n.Parent
				if parent != nil && (parent.Data == "video" || parent.Data == "audio") {
					resourceType = parent.Data
				}
			case "object", "embed":
				resourceURL = getAttr(n, "data")
				resourceType = "document"
			case "param":
				if getAttr(n, "name") == "movie" {
					resourceURL = getAttr(n, "value")
					resourceType = "video"
				}
			}

			// 如果提取到了资源链接并且资源类型在允许范围内，则添加到任务列表
			if resourceURL != "" && d.shouldDownload(resourceType) {
				absoluteURL, err := d.resolveURL(resourceURL)
				if err == nil {
					filename := d.generateFilename(absoluteURL, resourceType)
					tasks = append(tasks, DownloadTask{
						URL:      absoluteURL,
						Type:     resourceType,
						Filename: filename,
					})
				}
			}
		}

		// 检查内联样式中的资源链接
		if n.Type == html.ElementNode && getAttr(n, "style") != "" {
			urls := extractURLsFromCSS(getAttr(n, "style"))
			for _, u := range urls {
				absoluteURL, err := d.resolveURL(u)
				if err == nil {
					filename := d.generateFilename(absoluteURL, "image")
					tasks = append(tasks, DownloadTask{
						URL:      absoluteURL,
						Type:     "image",
						Filename: filename,
					})
				}
			}
		}

		// 递归处理子节点
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// 去重
	tasks = uniqueTasks(tasks)

	// 获取文件大小信息
	for i := range tasks {
		size, err := d.getFileSize(tasks[i].URL)
		if err == nil {
			tasks[i].Size = size
		}
	}

	return tasks, nil
}

// generateFilename 生成文件名
func (d *ResourceDownloader) generateFilename(rawURL, resourceType string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "unknown_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	filename := filepath.Base(parsedURL.Path)
	if filename == "" || filename == "." || filename == "/" {
		filename = "resource_" + sanitizeFilename(resourceType) + "_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		switch resourceType {
		case "image":
			ext = ".jpg"
		case "stylesheet":
			ext = ".css"
		case "script":
			ext = ".js"
		case "video":
			ext = ".mp4"
		case "audio":
			ext = ".mp3"
		case "font":
			ext = ".woff"
		case "document":
			ext = ".pdf"
		}
		filename += ext
	}

	return filename
}

// getFileSize 获取文件大小
func (d *ResourceDownloader) getFileSize(url string) (int64, error) {
	resp, err := d.Client.Head(url)
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("HEAD请求失败: %v", err))
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logError(d.LogFile, fmt.Sprintf("HTTP状态码错误: %d", resp.StatusCode))
		return 0, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	return resp.ContentLength, nil
}

// shouldDownload 判断是否应该下载该类型的资源
func (d *ResourceDownloader) shouldDownload(resourceType string) bool {
	if len(d.FileTypes) == 0 {
		return true
	}

	for _, t := range d.FileTypes {
		if strings.EqualFold(t, resourceType) {
			return true
		}
	}
	return false
}

// resolveURL 将相对URL转换为绝对URL
func (d *ResourceDownloader) resolveURL(rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "data:") {
		return "", fmt.Errorf("不支持data URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("URL解析失败: %v", err))
		return "", err
	}

	if !parsed.IsAbs() {
		parsed = d.BaseURL.ResolveReference(parsed)
	}

	parsed.Fragment = ""

	return parsed.String(), nil
}

// downloadTasks 执行下载任务
func (d *ResourceDownloader) downloadTasks(tasks []DownloadTask) error {
	progress := &Progress{Total: len(tasks)}
	var wg sync.WaitGroup
	taskChan := make(chan DownloadTask, d.MaxConcurrent)
	errorChan := make(chan error, len(tasks))
	var errorList []error

	go d.displayProgressBar(progress)

	go func() {
		for err := range errorChan {
			errorList = append(errorList, err)
		}
	}()

	for i := 0; i < d.MaxConcurrent; i++ {
		go func() {
			for task := range taskChan {
				if err := d.downloadResource(task, progress); err != nil {
					errorChan <- fmt.Errorf("下载 %s 失败: %v", task.URL, err)
				}
				wg.Done()
			}
		}()
	}

	for _, task := range tasks {
		wg.Add(1)
		taskChan <- task
	}

	close(taskChan)
	wg.Wait()
	close(errorChan)

	if len(errorList) > 0 {
		return fmt.Errorf("完成下载但有 %d 个错误，第一个错误: %v", len(errorList), errorList[0])
	}

	return nil
}

// downloadResource 下载单个资源
func (d *ResourceDownloader) downloadResource(task DownloadTask, progress *Progress) error {
	// 创建保存文件的目录
	subDir := filepath.Join(d.OutputDir, sanitizeFilename(task.Type))
	if err := os.MkdirAll(subDir, 0755); err != nil {
		logError(d.LogFile, fmt.Sprintf("创建目录失败: %v", err))
		return err
	}

	// 检查文件是否已存在
	outputPath := filepath.Join(subDir, task.Filename)
	if _, err := os.Stat(outputPath); err == nil {
		progress.Lock.Lock()
		progress.Completed++
		progress.Lock.Unlock()
		logInfo(d.LogFile, fmt.Sprintf("文件已存在，跳过下载: %s", outputPath))
		return nil
	}

	// 下载资源
	resp, err := d.Client.Get(task.URL)
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("GET请求失败: %v", err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logError(d.LogFile, fmt.Sprintf("HTTP状态码错误: %d", resp.StatusCode))
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	// 保存文件
	file, err := os.Create(outputPath)
	if err != nil {
		logError(d.LogFile, fmt.Sprintf("创建文件失败: %v", err))
		return err
	}
	defer file.Close()

	reader := &ProgressReader{
		Reader:   resp.Body,
		Total:    resp.ContentLength,
		Progress: 0,
		Task:     task,
		Callback: func(downloaded int64) {
			// 这里可以添加单个文件的下载进度显示
		},
	}

	_, err = io.Copy(file, reader)
	if err == nil {
		progress.Lock.Lock()
		progress.Completed++
		progress.Lock.Unlock()
		logInfo(d.LogFile, fmt.Sprintf("文件下载完成: %s", outputPath))
	} else {
		logError(d.LogFile, fmt.Sprintf("文件下载失败: %v", err))
	}
	return err
}

// displayProgressBar 显示下载进度条
func (d *ResourceDownloader) displayProgressBar(progress *Progress) {
	const barWidth = 50
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			progress.Lock.Lock()
			completed := progress.Completed
			total := progress.Total
			progress.Lock.Unlock()

			if completed >= total {
				fmt.Printf("\r[%s] %d/%d 已完成\n",
					strings.Repeat("=", barWidth),
					completed,
					total)
				return
			}

			percent := float64(completed) / float64(total)
			filled := int(percent * float64(barWidth))
			bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)

			fmt.Printf("\r[%s] %d/%d (%.1f%%)",
				bar,
				completed,
				total,
				percent*100)
		}
	}
}

// ProgressReader 用于跟踪下载进度
type ProgressReader struct {
	Reader   io.Reader
	Total    int64
	Progress int64
	Task     DownloadTask
	Callback func(downloaded int64)
}

// Read 实现io.Reader接口，用于读取数据并更新进度
func (r *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.Progress += int64(n)
	if r.Callback != nil {
		r.Callback(r.Progress)
	}
	return n, err
}

// getAttr 从HTML节点中获取指定属性的值
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// extractURLsFromCSS 从CSS中提取URL
func extractURLsFromCSS(css string) []string {
	var urls []string
	re := regexp.MustCompile(`url\((['"]?)(.*?)\1\)`)
	matches := re.FindAllStringSubmatch(css, -1)
	for _, m := range matches {
		if len(m) > 2 && m[2] != "" {
			urls = append(urls, m[2])
		}
	}
	return urls
}

// uniqueTasks 去重
func uniqueTasks(tasks []DownloadTask) []DownloadTask {
	seen := make(map[string]bool)
	var unique []DownloadTask

	for _, task := range tasks {
		if !seen[task.URL] {
			seen[task.URL] = true
			unique = append(unique, task)
		}
	}

	return unique
}

// sanitizeFilename 清理文件名，确保其合法性
func sanitizeFilename(name string) string {
	// 移除非法字符
	name = strings.Map(func(r rune) rune {
		switch {
		case r == ' ', r == '-', r == '_':
			return r
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)

	if len(name) > 50 {
		name = name[:50]
	}

	return strings.ToLower(name)
}

// formatFileSize 格式化文件大小
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// shortenURL 截断URL，使其适合显示
func shortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// logInfo 记录INFO级别的日志
func logInfo(logFile *os.File, message string) {
	logEntry := struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}{
		Time:    time.Now().Format(time.RFC3339),
		Level:   "INFO",
		Message: message,
	}

	logData, _ := json.Marshal(logEntry)
	logFile.WriteString(string(logData) + "\n")
}

// logError 记录ERROR级别的日志
func logError(logFile *os.File, message string) {
	logEntry := struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}{
		Time:    time.Now().Format(time.RFC3339),
		Level:   "ERROR",
		Message: message,
	}

	logData, _ := json.Marshal(logEntry)
	logFile.WriteString(string(logData) + "\n")
}

