package middleware

import (
	"claude2api/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware initializes the Claude client from the request header
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 管理页面单独鉴权：优先使用 ADMIN_KEY（如未配置则回退到 APIKEY）
		if strings.HasPrefix(c.Request.URL.Path, "/admin") {
			adminKey := config.ConfigInstance.AdminKey
			if adminKey == "" {
				adminKey = config.ConfigInstance.APIKey
			}

			// 1) Header: Authorization: Bearer <key>
			key := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
			if key == adminKey {
				c.Next()
				return
			}

			// 2) Query: ?key=<key>（便于浏览器首次打开 /admin）
			if q := c.Query("key"); q != "" && q == adminKey {
				http.SetCookie(c.Writer, &http.Cookie{
					Name:     "admin_key",
					Value:    q,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
				c.Next()
				return
			}

			// 3) Cookie: admin_key=<key>
			if ck, err := c.Request.Cookie("admin_key"); err == nil && ck != nil && ck.Value == adminKey {
				c.Next()
				return
			}

			c.JSON(401, gin.H{"error": "Missing or invalid admin key"})
			c.Abort()
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
