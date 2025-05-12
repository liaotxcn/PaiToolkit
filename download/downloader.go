package download

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

var TaskStatusLock sync.Mutex              // 公共锁变量，保护并发访问任务状态
var DownloadHistory []DownloadHistoryEntry // 存储下载历史记录
var HistoryLock sync.Mutex                 // 保护下载历史记录的并发访问
var downloader *ResourceDownloader         // 资源下载器实例

// ResourceDownloader 定义资源下载器的结构体，包含下载所需的各种配置和客户端
type ResourceDownloader struct {
	BaseURL       *url.网站// 目标网页的URL
	OutputDir     string        // 下载文件存放目录
	MaxConcurrent int           // 最大并发下载数
	FileTypes     []string      // 允许下载的文件类型
	Client        *http.Client  // HTTP客户端
	LogFile       *os.File      // 日志文件
	UserAgent     string        // 用户代理
	RetryTimes    int           // 下载失败重试次数
	Timeout       time.Duration // 请求超时时间
}

// DownloadTask 定义下载任务的结构体，包含任务的各种信息
type DownloadTask struct {
网站string    // 资源URL
类型string    // 资源类型
	Filename     string    // 保存文件名
	Size         int64     // 文件大小(字节)
	LastModified time.Time // 最后修改时间
	RetryCount   int       // 已重试次数
状态string    // 下载状态
	StartTime    time.Time // 开始时间
	EndTime      time.Time // 结束时间
	HistoryID    string    // 历史记录 ID
}

// Progress 定义下载进度的结构体，记录下载任务的总体进度
type Progress struct {
	Total     int       // 总任务数
	Completed int       // 已完成数
	Failed    int       // 失败数
	Skipped   int       // 跳过数
	StartTime time.Time // 开始时间
	Lock      sync.Mutex
}

// APIResponse 定义 API 响应的结构体，用于返回 API 调用的结果
type APIResponse struct {
代码int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// TaskStatus 定义任务状态的结构体，记录每个下载任务的详细状态
type TaskStatus struct {
网站string `json:"url"`
	Filename     string `json:"filename"`
类型string `json:"type"`
	Size         int64  `json:"size"`
状态string `json:"status"`
	RetryCount   int    `json:"retry_count"`
	LastModified string `json:"last_modified"`
}

// DownloadProgress 定义下载进度信息的结构体，用于返回给客户端
type DownloadProgress struct {
	Total     int          `json:"total"`
	Completed int          `json:"completed"`
	Failed    int          `json:"failed"`
	Skipped   int          `json:"skipped"`
	Duration  string       `json:"duration"`
	Tasks     []TaskStatus `json:"tasks"`
	Rate      float64      `json:"rate"`
}

// 历史记录结构体，记录每个下载任务的详细历史信息
type DownloadHistoryEntry struct {
网站string    `json:"url"`
	Filename     string    `json:"filename"`
类型string    `json:"type"`
	Size         int64     `json:"size"`
状态string    `json:"status"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	RetryCount   int       `json:"retry_count"`
	LastModified time.Time `json:"last_modified"`
}

// fileExtensions 定义不同资源类型对应的文件扩展名映射
var fileExtensions = map[string][]string{
	"image":    {"jpg", "jpeg", "png", "gif", "bmp", "svg", "webp", "ico", "tiff", "apng"},
	"script":   {"js", "mjs", "ts", "jsx", "coffee"},
	"style":    {"css", "scss", "less", "sass", "styl"},
	"video":    {"mp4", "webm", "ogg", "flv", "avi", "mov", "wmv", "mkv", "mpeg", "3gp"},
	"audio":    {"mp3", "wav", "ogg", "aac", "flac", "m4a", "wma", "opus"},
	"font":     {"woff", "woff2", "ttf", "otf", "eot", "svg", "fnt"},
	"document": {"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "rtf", "csv", "odt"},
	"archive":  {"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "iso"},
	"html":     {"html", "htm", "xhtml", "php", "asp", "aspx", "jsp", "cfm"},
	"data":     {"json", "xml", "yaml", "yml", "toml", "csv", "jsonld"},
	"binary":   {"exe", "dll", "so", "dmg", "pkg", "deb", "rpm", "msi"},
}

// NewResourceDownloader 创建一个新的资源下载器实例，使用默认配置
func NewResourceDownloader() *ResourceDownloader {
	return &ResourceDownloader{
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36",
		OutputDir:     "./download_data",
		MaxConcurrent: 5,
		RetryTimes:    5,
		Timeout:       40 * time.Second,
	}
}

// GetHTTPClient 获取 HTTP 客户端实例，如果客户端未初始化，则创建一个新的实例
func (d *ResourceDownloader) GetHTTPClient() *http.Client {
	if d.Client == nil {
		d.Client = d.createHTTPClient()
	}
	return d.Client
}

// createHTTPClient 创建一个新的 HTTP 客户端实例，配置 TLS 连接和连接池
func (d *ResourceDownloader) createHTTPClient() *http.Client {
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
		MaxIdleConns:        d.MaxConcurrent * 2,
		MaxIdleConnsPerHost: d.MaxConcurrent,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &http.Client{
		Timeout:   d.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	return client
}

// FetchHTML 获取目标网页的 HTML 内容，包含重试逻辑
func (d *ResourceDownloader) FetchHTML() (string, error) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		client := d.GetHTTPClient()

		req, err := http.NewRequest("GET", d.BaseURL.String(), nil)
		if err != nil {
			if i == maxRetries-1 {
				return "", fmt.Errorf("创建请求失败: %v", err)
			}
			continue
		}
		req.Header.Set("User-Agent", d.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Referer", d.BaseURL.String())

		resp, err := client.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return "", fmt.Errorf("请求失败: %v", err)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if i == maxRetries-1 {
				return "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
			}
			continue
		}

		var reader io.Reader
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			gz, err := gzip.NewReader(resp.Body)
			if err != nil {
				if i == maxRetries-1 {
					return "", fmt.Errorf("解压响应体失败: %v", err)
				}
				continue
			}
			defer gz.Close()
			reader = gz
		default:
			reader = resp.Body
		}

		content, err := io.ReadAll(reader)
		if err != nil {
			if i == maxRetries-1 {
				return "", fmt.Errorf("读取响应体失败: %v", err)
			}
			continue
		}

		utf8Content, err := convertToUTF8(content)
		if err != nil {
			return "", fmt.Errorf("编码转换失败: %v", err)
		}

		return string(utf8Content), nil
	}

	return "", fmt.Errorf("达到最大重试次数，仍未成功获取HTML")
}

// ExtractResources 从 HTML 内容中提取可下载的资源任务
func (d *ResourceDownloader) ExtractResources(htmlContent string) ([]DownloadTask, error) {
	var tasks []DownloadTask

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("HTML解析失败: %v", err)
	}

	// 递归处理 HTML 节点，提取资源任务
	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var resourceURL, resourceType string

			switch n.Data {
			case "img", "image":
				resourceURL = getAttribute(n, "src")
				resourceType = "image"
			case "script":
				resourceURL = getAttribute(n, "src")
				resourceType = "script"
			case "link":
				rel := getAttribute(n, "rel")
				if rel == "stylesheet" {
					resourceURL = getAttribute(n, "href")
					resourceType = "style"
				} else if strings.Contains(rel, "icon") {
					resourceURL = getAttribute(n, "href")
					resourceType = "image"
				}
			case "video", "audio":
				resourceURL = getAttribute(n, "src")
				resourceType = n.Data
				if resourceURL == "" {
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.ElementNode && c.Data == "source" {
							resourceURL = getAttribute(c, "src")
							break
						}
					}
				}
			case "iframe", "frame", "embed", "object":
				resourceURL = getAttribute(n, "src")
				if resourceURL == "" {
					resourceURL = getAttribute(n, "data")
				}
				if isHTMLFile(resourceURL) {
					resourceType = "html"
				} else {
					resourceType = "document"
				}
			case "a":
				href := getAttribute(n, "href")
				if href != "" {
					if isHTMLFile(href) {
						resourceURL = href
						resourceType = "html"
					} else if isDirectDownloadLink(href) {
						resourceURL = href
						resourceType = getResourceTypeFromURL(href)
					}
				}
			}

			if resourceURL != "" && d.isAllowedType(resourceType) {
				if absoluteURL, err := d.ResolveURL(resourceURL); err == nil {
					tasks = append(tasks, DownloadTask{
						URL:      absoluteURL,
						Type:     resourceType,
						Filename: d.GenerateFilename(absoluteURL, resourceType),
					})
				}
			}
		}

		if style := getAttribute(n, "style"); style != "" {
			for _, u := range extractURLsFromCSS(style) {
				if absoluteURL, err := d.ResolveURL(u); err == nil {
					tasks = append(tasks, DownloadTask{
						URL:      absoluteURL,
						Type:     "image",
						Filename: d.GenerateFilename(absoluteURL, "image"),
					})
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processNode(c)
		}
	}
	processNode(doc)

	return d.ProcessTasks(tasks)
}

// ProcessTasks 处理下载任务列表，去除重复的任务
func (d *ResourceDownloader) ProcessTasks(tasks []DownloadTask) ([]DownloadTask, error) {
	uniqueTasks := make([]DownloadTask, 0)
	seen := make(map[string]bool)

	for _, task := range tasks {
		if !seen[task.URL] {
			seen[task.URL] = true
			uniqueTasks = append(uniqueTasks, task)
		}
	}

	return uniqueTasks, nil
}

// DownloadWithRetry 带有重试逻辑的下载函数，尝试多次下载任务
func DownloadWithRetry(task DownloadTask, downloader *ResourceDownloader, progress *Progress, taskStatuses *[]TaskStatus) {
	task.StartTime = time.Now()
	var lastErr error
	for i := 0; i <= downloader.RetryTimes; i++ {
		task.RetryCount = i
		if err := DownloadResource(task, downloader, progress, taskStatuses); err == nil {
			task.EndTime = time.Now()
			task.Status = "completed"

			HistoryLock.Lock()
			DownloadHistory = append(DownloadHistory, DownloadHistoryEntry{
				URL:          task.URL,
				Filename:     task.Filename,
				Type:         task.Type,
				Size:         task.Size,
				Status:       task.Status,
				StartTime:    task.StartTime,
				EndTime:      task.EndTime,
				RetryCount:   task.RetryCount,
				LastModified: task.LastModified,
			})
			HistoryLock.Unlock()

			TaskStatusLock.Lock()
			for index := range *taskStatuses {
				if (*taskStatuses)[index].URL == task.URL {
					(*taskStatuses)[index].Status = "completed"
					(*taskStatuses)[index].RetryCount = i
					if !task.LastModified.IsZero() {
						(*taskStatuses)[index].LastModified = task.LastModified.Format(time.RFC3339)
					}
					break
				}
			}
			TaskStatusLock.Unlock()

			progress.Lock.Lock()
			progress.Completed++
			progress.Lock.Unlock()
			return
		} else {
			lastErr = err
			if i < downloader.RetryTimes {
				time.Sleep(time.Duration(i+1) * time.Second)
			}
		}
	}

	task.EndTime = time.Now()
	task.Status = "failed"

	HistoryLock.Lock()
	DownloadHistory = append(DownloadHistory, DownloadHistoryEntry{
		URL:          task.URL,
		Filename:     task.Filename,
		Type:         task.Type,
		Size:         task.Size,
		Status:       task.Status,
		StartTime:    task.StartTime,
		EndTime:      task.EndTime,
		RetryCount:   task.RetryCount,
		LastModified: task.LastModified,
	})
	HistoryLock.Unlock()

	TaskStatusLock.Lock()
	for index := range *taskStatuses {
		if (*taskStatuses)[index].URL == task.URL {
			(*taskStatuses)[index].Status = "failed"
			(*taskStatuses)[index].RetryCount = downloader.RetryTimes
			break
		}
	}
	TaskStatusLock.Unlock()

	progress.Lock.Lock()
	progress.Failed++
	progress.Lock.Unlock()

	LogError(downloader.LogFile, fmt.Sprintf("下载失败: %s", lastErr.Error()))
}

// DownloadResource 下载单个资源任务，处理文件保存和错误处理
func DownloadResource(task DownloadTask, downloader *ResourceDownloader, progress *Progress, taskStatuses *[]TaskStatus) error {
	saveDir := filepath.Join(downloader.OutputDir, task.Type)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	savePath := filepath.Join(saveDir, task.Filename)
	if info, err := os.Stat(savePath); err == nil {
		if task.Size > 0 && info.Size() == task.Size {
			return nil
		}
	}

	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", downloader.UserAgent)

	resp, err := downloader.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}

	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("关闭文件失败: %v", cerr)
		}
	}()

	if isTextType(task.Type) {
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			os.Remove(savePath)
			return fmt.Errorf("读取内容失败: %v", err)
		}

		utf8Content, err := convertToUTF8(content)
		if err != nil {
			os.Remove(savePath)
			return fmt.Errorf("编码转换失败: %v", err)
		}

		if _, err := file.Write(utf8Content); err != nil {
			os.Remove(savePath)
			return fmt.Errorf("写入文件失败: %v", err)
		}
	} else {
		if _, err := io.Copy(file, resp.Body); err != nil {
			os.Remove(savePath)
			return fmt.Errorf("下载失败: %v", err)
		}
	}

	return nil
}

// getAttribute 获取 HTML 节点的指定属性值
func getAttribute(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// extractURLsFromCSS 从 CSS 样式中提取 URL 列表
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

// isDirectDownloadLink 判断 URL 是否为直接下载链接
func isDirectDownloadLink(url string) bool {
	ext := strings.ToLower(filepath.Ext(url))
	for _, exts := range fileExtensions {
		for _, e := range exts {
			if "."+e == ext {
				return true
			}
		}
	}
	return false
}

// isHTMLFile 判断 URL 是否指向 HTML 文件
func isHTMLFile(url string) bool {
	if url == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(url))
	return ext == ".html" || ext == ".htm" || ext == ".xhtml" ||
		ext == ".php" || ext == ".asp" || ext == ".aspx" || ext == ".jsp"
}

// getResourceTypeFromURL 根据 URL 获取资源类型
func getResourceTypeFromURL(url string) string {
	ext := strings.ToLower(filepath.Ext(url))
	if ext == "" {
		if strings.Contains(url, ".php") || strings.Contains(url, ".asp") {
			return "html"
		}
		return "document"
	}

	for typ, exts := range fileExtensions {
		for _, e := range exts {
			if "."+e == ext {
				return typ
			}
		}
	}

	if strings.Contains(url, "/page/") || strings.Contains(url, "/article/") {
		return "html"
	}

	return "document"
}

// isAllowedType 判断资源类型是否在允许下载的类型列表中
func (d *ResourceDownloader) isAllowedType(resourceType string) bool {
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

// ResolveURL 将相对 URL 解析为绝对 URL，并处理 data URL 等特殊情况
func (d *ResourceDownloader) ResolveURL(rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "data:") {
		return "", fmt.Errorf("不支持data URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("URL解析失败: %v", err)
	}

	if !parsed.IsAbs() {
		parsed = d.BaseURL.ResolveReference(parsed)
	}

	parsed.Fragment = ""
	return parsed.String(), nil
}

// GenerateFilename 根据 URL 和资源类型生成保存文件名
func (d *ResourceDownloader) GenerateFilename(rawURL, resourceType string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Sprintf("unknown_%d", time.Now().UnixNano())
	}

	baseName := filepath.Base(parsed.Path)
	if baseName == "" || baseName == "." || baseName == "/" {
		baseName = fmt.Sprintf("resource_%d", time.Now().UnixNano())
	}

	ext := filepath.Ext(baseName)
	if ext == "" {
		if exts, ok := fileExtensions[resourceType]; ok && len(exts) > 0 {
			ext = "." + exts[0]
		} else {
			ext = ".bin"
		}
		baseName += ext
	}

	return sanitizeFilename(baseName)
}

// sanitizeFilename 清理文件名，去除非法字符并限制长度
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r == ' ', r == '-', r == '_', r == '.':
			return r
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return r
		default:
			return '_'
		}
	}, name)

	if len(name) > 100 {
		name = name[:100] + filepath.Ext(name)
	}

	return name
}

// formatFileSize 将文件大小转换为易读的格式
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

// shortenURL 缩短 URL 长度，超过指定长度时用省略号代替
func shortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// logError 将错误信息写入日志文件
func logError(logFile *os.File, message string) {
	entry := map[string]string{
		"time":    time.Now().Format(time.RFC3339),
		"level":   "ERROR",
		"message": message,
	}

	data, _ := json.Marshal(entry)
	logFile.Write(append(data, '\n'))
}

// isTextType 判断资源类型是否为文本类型
func isTextType(resourceType string) bool {
	switch resourceType {
	case "html", "script", "style", "document", "data":
		return true
	default:
		return false
	}
}

// convertToUTF8 将字节内容转换为 UTF-8 编码
func convertToUTF8(content []byte) ([]byte, error) {
	encoding, _, certain := charset.DetermineEncoding(content, "")
	if !certain {
		return content, nil
	}

	decoder := encoding.NewDecoder()
	reader := transform.NewReader(bytes.NewReader(content), decoder)
	return io.ReadAll(reader)
}

// GetDownloadHistory 获取下载历史记录，使用锁保护并发访问
func GetDownloadHistory() []DownloadHistoryEntry {
	HistoryLock.Lock()
	defer HistoryLock.Unlock()
	return DownloadHistory
}
