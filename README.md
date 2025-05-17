# PaiToolkit-PaiDownloader基于Golang的资源嗅探下载器 

<div align="center">  

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)  

</div>  

---

## 📂 项目结构  
```plaintext
PaiDownloader/
├── api/                  # API处理逻辑
│   ├── handlers.go       # 请求处理器
│   └── responses.go      # 响应格式化
├── config/               # 配置管理
│   └── config.go
├── download/             # 核心下载功能
│   ├── downloader.go     # 下载器主逻辑
│   ├── resources.go      # 资源处理
│   └── utils.go          # 工具函数
├── middleware/           # 中间件
│   ├── cors.go           # CORS处理
│   ├── ratelimit.go      # 请求限流
│   └── xss.go            # XSS防护
├── static/               # 静态资源
│   ├── css/              
│   ├── js/               
│   └── image/            
├── go.mod                # 依赖管理
├── main.go               # 程序入口
└── config.json           # 配置文件(可选)
```

---

## 功能特性
- 自动分析网页内容并提取可下载资源
- 支持多文件类型筛选下载
- 提供实时下载进度监控
- 记录下载历史
- 支持断点续传
- 完善监控日志记录

---

## 🚀 快速开始
### 环境要求
- Go 1.20+
```bash
git clone https://github.com/liaotxcn/PaiToolkit.git  # 克隆仓库
```
```bash
go mod tidy   # 更新依赖
go run main.go 
```

ps：资源嗅探、下载速度等受网络环境影响

### 研发中，持续更新...
