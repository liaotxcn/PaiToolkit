package download

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// 从HTML中提取URL
func ExtractURLsFromCSS(css string) []string {
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

// 检查是否为直接下载链接
func CheckDirectDownloadLink(url string) bool {
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

// 检查是否为HTML文件
func CheckHTMLFile(url string) bool {
	if url == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(url))
	return ext == ".html" || ext == ".htm" || ext == ".xhtml" ||
		ext == ".php" || ext == ".asp" || ext == ".aspx" || ext == ".jsp"
}

// 从URL获取资源类型
func DetermineResourceTypeFromURL(url string) string {
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

// 清理文件名
func CleanFilename(name string) string {
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

// 格式化文件大小
func FormatFileSize(bytes int64) string {
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

// 短化URL
func ShortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// 记录错误日志
func LogError(logFile *os.File, message string) {
	entry := map[string]string{
		"time":    time.Now().Format(time.RFC3339),
		"level":   "ERROR",
		"message": message,
	}

	data, _ := json.Marshal(entry)
	logFile.Write(append(data, '\n'))
}

// 检查是否是文本类型
func CheckTextType(resourceType string) bool {
	switch resourceType {
	case "html", "script", "style", "document", "data":
		return true
	default:
		return false
	}
}

// 将内容转换为UTF-8编码
func ConvertToUTF8(content []byte) ([]byte, error) {
	encoding, _, certain := charset.DetermineEncoding(content, "")
	if !certain {
		return content, nil
	}

	decoder := encoding.NewDecoder()
	reader := transform.NewReader(bytes.NewReader(content), decoder)
	return io.ReadAll(reader)
}
