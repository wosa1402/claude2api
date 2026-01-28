package service

import (
	"claude2api/config"
	"embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed admin_assets/*.html
var adminAssets embed.FS

func AdminPageHandler(c *gin.Context) {
	if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) == "" {
		c.Redirect(http.StatusFound, "/admin/setup")
		return
	}
	if !isAdminLoggedIn(c.Request) {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}
	b, err := adminAssets.ReadFile("admin_assets/admin.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load admin page")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

func AdminLoginPageHandler(c *gin.Context) {
	if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) == "" {
		c.Redirect(http.StatusFound, "/admin/setup")
		return
	}
	if isAdminLoggedIn(c.Request) {
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	b, err := adminAssets.ReadFile("admin_assets/login.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load login page")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

func AdminSetupPageHandler(c *gin.Context) {
	if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) != "" {
		// 已初始化，跳转登录即可
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}
	b, err := adminAssets.ReadFile("admin_assets/setup.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load setup page")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}
