package service

import (
	"claude2api/config"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type setOpenListReq struct {
	WebDAVURL     string `json:"webdavUrl"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Directory     string `json:"directory"`
	ClearPassword bool   `json:"clearPassword"`
}

func AdminSetOpenListHandler(c *gin.Context) {
	var req setOpenListReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}

	webdavURL := strings.TrimSpace(req.WebDAVURL)
	username := strings.TrimSpace(req.Username)
	directory := config.NormalizeOpenListDirectory(req.Directory)

	if webdavURL != "" {
		u, err := url.Parse(webdavURL)
		if err != nil || strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "webdavUrl 格式错误，必须包含协议和主机"})
			return
		}
	}

	config.ConfigInstance.RwMutx.Lock()
	prevURL := config.ConfigInstance.OpenListWebDAVURL
	prevUsername := config.ConfigInstance.OpenListUsername
	prevPassword := config.ConfigInstance.OpenListPassword
	prevDirectory := config.ConfigInstance.OpenListDirectory

	config.ConfigInstance.OpenListWebDAVURL = webdavURL
	config.ConfigInstance.OpenListUsername = username
	config.ConfigInstance.OpenListDirectory = directory
	switch {
	case req.ClearPassword:
		config.ConfigInstance.OpenListPassword = ""
	case strings.TrimSpace(req.Password) != "":
		config.ConfigInstance.OpenListPassword = strings.TrimSpace(req.Password)
	}
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		config.ConfigInstance.RwMutx.Lock()
		config.ConfigInstance.OpenListWebDAVURL = prevURL
		config.ConfigInstance.OpenListUsername = prevUsername
		config.ConfigInstance.OpenListPassword = prevPassword
		config.ConfigInstance.OpenListDirectory = prevDirectory
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败: " + err.Error()})
		return
	}

	config.ConfigInstance.RwMutx.RLock()
	passwordSet := strings.TrimSpace(config.ConfigInstance.OpenListPassword) != ""
	enabled := strings.TrimSpace(config.ConfigInstance.OpenListWebDAVURL) != ""
	config.ConfigInstance.RwMutx.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"ok":                  true,
		"savedPath":           path,
		"openListEnabled":     enabled,
		"openListPasswordSet": passwordSet,
		"openListDirectory":   directory,
	})
}
