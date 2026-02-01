package service

import (
	"claude2api/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type setIPFamilyReq struct {
	Family string `json:"family"`
}

func AdminSetIPFamilyHandler(c *gin.Context) {
	var req setIPFamilyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}
	f := strings.ToLower(strings.TrimSpace(req.Family))
	switch f {
	case "", "auto":
		f = "auto"
	case "ipv4", "4":
		f = "ipv4"
	case "ipv6", "6":
		f = "ipv6"
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "family 只允许 auto/ipv4/ipv6"})
		return
	}

	config.ConfigInstance.RwMutx.Lock()
	prev := config.ConfigInstance.ForceIPFamily
	config.ConfigInstance.ForceIPFamily = f
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		config.ConfigInstance.RwMutx.Lock()
		config.ConfigInstance.ForceIPFamily = prev
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "savedPath": path, "forceIPFamily": f})
}

type setRecordEgressReq struct {
	Enabled bool `json:"enabled"`
}

func AdminSetRecordEgressHandler(c *gin.Context) {
	var req setRecordEgressReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}

	config.ConfigInstance.RwMutx.Lock()
	prev := config.ConfigInstance.RecordEgressIP
	config.ConfigInstance.RecordEgressIP = req.Enabled
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		config.ConfigInstance.RwMutx.Lock()
		config.ConfigInstance.RecordEgressIP = prev
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "savedPath": path, "recordEgressIP": req.Enabled})
}
