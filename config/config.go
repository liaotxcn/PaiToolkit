package config

import (
	"encoding/json"
	"os"
)

// AppConfig 定义应用程序的配置结构，用于存储服务器端口、下载目录和最大并发数等信息。
type AppConfig struct {
	ServerPort    string `json:"server_port"`
	DownloadDir   string `json:"download_dir"`
	MaxConcurrent int    `json:"max_concurrent"`
}

// LoadConfig 函数用于加载配置文件。如果配置文件不存在，则创建一个默认配置文件。
// 参数 configPath 配置文件的路径，如果为空，则使用默认路径 "config.json"。
// 返回值 AppConfig 指针和可能出现的错误。
func LoadConfig(configPath string) (*AppConfig, error) {
	// 如果未提供配置文件路径，则使用默认路径
	if configPath == "" {
		configPath = "config.json"
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 创建默认配置
		defaultConfig := &AppConfig{
			ServerPort:    "8080",            // 服务端口
			DownloadDir:   "./download_data", // 文件下载存放目录
			MaxConcurrent: 5,                 // 最大并发数
		}

		// 将默认配置转换为 JSON 格式
		data, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, err
		}

		// 将默认配置写入文件
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, err
		}

		return defaultConfig, nil
	}

	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config AppConfig
	// 将 JSON 数据解析到 AppConfig 结构体中
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
