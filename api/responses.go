package api

import (
	"PaiDownloader/download"
	"github.com/gin-gonic/gin"
)

// RenderAPIResponse 渲染API响应
func RenderAPIResponse(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, download.APIResponse{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// RenderError 渲染错误响应
func RenderError(c *gin.Context, code int, message string, err error) {
	RenderAPIResponse(c, code, message, map[string]interface{}{
		"error": err.Error(),
	})
}
