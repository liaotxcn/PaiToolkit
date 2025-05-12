package main

import (
	"PaiDownloader/api"
	"PaiDownloader/config"
	"PaiDownloader/download"
	"PaiDownloader/middleware"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置文件
	config, err := config.LoadConfig("")
	if err != nil {
		panic(fmt.Sprintf("加载配置文件失败: %v", err))
	}
	downloader := download.NewResourceDownloader()
	downloader.OutputDir = config.DownloadDir
	downloader.MaxConcurrent = config.MaxConcurrent

	r := gin.Default()

	// 路由中间件(CORS、XSS)
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.XSSMiddleware())

	// 静态文件服务
	r.Static("/static", "./static")

	// API 路由
	r.GET("/", api.HandleIndexPage)
	r.POST("/download", api.HandleDownloadRequest)
	r.GET("/progress-sse", api.HandleProgressSSE)
	r.POST("/preview", api.HandlePreviewRequest)
	r.GET("/cancel", api.HandleCancelRequest)
	r.POST("/history", api.HandleGetHistory)
	r.GET("/history", api.HandleHistoryPage)

	// 服务启动
	r.Run(":8080")
}
