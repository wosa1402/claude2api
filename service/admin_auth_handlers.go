package service

import (
	"claude2api/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type adminLoginReq struct {
	Password string `json:"password"`
}

func AdminLoginHandler(c *gin.Context) {
	if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) == "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "请先完成初始化设置"})
		return
	}

	var req adminLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "密码不能为空"})
		return
	}
	if !config.CheckPassword(config.ConfigInstance.AdminPasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "密码错误"})
		return
	}
	if err := createAdminSession(c.Writer, c.Request); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "登录失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func AdminLogoutHandler(c *gin.Context) {
	destroyAdminSession(c.Request)
	clearAdminSIDCookie(c.Writer)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type adminSetupReq struct {
	APIKey         string `json:"apiKey"`
	AdminPassword  string `json:"adminPassword"`
	AdminPassword2 string `json:"adminPassword2"`
}

func AdminSetupHandler(c *gin.Context) {
	// 已初始化：不允许重复走引导（保持简单）
	if strings.TrimSpace(config.ConfigInstance.AdminPasswordHash) != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "已完成初始化，无需重复设置"})
		return
	}

	var req adminSetupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.AdminPassword = strings.TrimSpace(req.AdminPassword)
	req.AdminPassword2 = strings.TrimSpace(req.AdminPassword2)

	if req.AdminPassword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "管理后台密码不能为空"})
		return
	}
	if req.AdminPassword != req.AdminPassword2 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "两次输入的密码不一致"})
		return
	}
	// 如果当前没有 APIKey，要求用户提供（否则 /v1 无法用）
	if strings.TrimSpace(config.ConfigInstance.APIKey) == "" && req.APIKey == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "APIKey 不能为空"})
		return
	}

	hash, err := config.HashPassword(req.AdminPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "生成密码失败：" + err.Error()})
		return
	}

	config.ConfigInstance.RwMutx.Lock()
	config.ConfigInstance.AdminPasswordHash = hash
	prevAPIKey := config.ConfigInstance.APIKey
	if req.APIKey != "" {
		config.ConfigInstance.APIKey = req.APIKey
	}
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		// 回滚
		config.ConfigInstance.RwMutx.Lock()
		config.ConfigInstance.AdminPasswordHash = ""
		config.ConfigInstance.APIKey = prevAPIKey
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败：" + err.Error()})
		return
	}

	if err := createAdminSession(c.Writer, c.Request); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "初始化成功但登录失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "savedPath": path})
}
