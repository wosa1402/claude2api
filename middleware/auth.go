package middleware

import (
	"claude2api/config"
	"net/http"
	"strings"

	"claude2api/service"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware initializes the Claude client from the request header
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/admin") {
			// 允许未登录访问的页面/接口：/admin(会在 handler 里重定向)、/admin/login、/admin/setup、/admin/logout
			switch c.Request.URL.Path {
			case "/admin", "/admin/login", "/admin/setup", "/admin/logout":
				c.Next()
				return
			}

			// 管理后台未初始化：阻止访问需要登录的接口
			if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) == "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "请先访问 /admin/setup 完成初始化"})
				c.Abort()
				return
			}

			// 已初始化但未登录
			if !service.AdminIsLoggedIn(c.Request) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录，请先访问 /admin/login"})
				c.Abort()
				return
			}

			c.Next()
			return
		}

		if strings.HasPrefix(c.Request.URL.Path, "/v1/files/") {
			c.Next()
			return
		}

		if config.ConfigInstance.EnableMirrorApi && strings.HasPrefix(c.Request.URL.Path, config.ConfigInstance.MirrorApiPrefix) {
			c.Set("UseMirrorApi", true)
			c.Next()
			return
		}
		Key := c.GetHeader("Authorization")
		if Key != "" {
			Key = strings.TrimPrefix(Key, "Bearer ")
			if Key != config.ConfigInstance.APIKey {
				c.JSON(401, gin.H{
					"error": "Invalid API key",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}
		c.JSON(401, gin.H{
			"error": "Missing or invalid Authorization header",
		})
		c.Abort()
	}
}
