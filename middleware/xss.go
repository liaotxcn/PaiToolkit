package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

func XSSMiddleware() gin.HandlerFunc {
	policy := bluemonday.UGCPolicy()
	return func(c *gin.Context) {
		// 处理 POST 请求的表单数据
		c.Request.ParseForm()
		for key, values := range c.Request.PostForm {
			for i, value := range values {
				values[i] = policy.Sanitize(value)
			}
			c.Request.PostForm[key] = values
		}

		// 处理 URL 查询参数
		for key, values := range c.Request.URL.Query() {
			for i, value := range values {
				values[i] = policy.Sanitize(value)
			}
			c.Request.URL.Query()[key] = values
		}

		c.Next()
	}
}
