package download

import (
	"fmt"
	"net/http"
	"time"
)

// 获取文件信息
func GetFileInfo(url string, client *http.Client) (int64, time.Time, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, time.Time{}, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	lastModified, _ := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	return resp.ContentLength, lastModified, nil
}
