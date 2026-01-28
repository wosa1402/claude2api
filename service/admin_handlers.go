package service

import (
	"claude2api/config"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type adminStatusResp struct {
	Global struct {
		OK          int     `json:"ok"`
		Fail        int     `json:"fail"`
		SuccessRate float64 `json:"successRate"`
		LastAt      string  `json:"lastAt"`
		LastOkAt    string  `json:"lastOkAt"`
		LastError   string  `json:"lastError"`
	} `json:"global"`
	Accounts []adminAccountResp `json:"accounts"`
}

type adminAccountResp struct {
	ID          int           `json:"id"`
	MaskedKey   string        `json:"maskedKey"`
	Name        string        `json:"name"`
	Account     string        `json:"account"`
	Enabled     bool          `json:"enabled"`
	OK          int           `json:"ok"`
	Fail        int           `json:"fail"`
	SuccessRate float64       `json:"successRate"`
	Recent      []interface{} `json:"recent"`
	LastError   string        `json:"lastError"`
	LastAt      string        `json:"lastAt"`
}

func AdminStatusHandler(c *gin.Context) {
	resp := adminStatusResp{}

	statsMu.Lock()
	g := global
	statsMu.Unlock()

	total := g.OK + g.Fail
	if total > 0 {
		resp.Global.SuccessRate = float64(g.OK) / float64(total)
	}
	resp.Global.OK = g.OK
	resp.Global.Fail = g.Fail
	if !g.LastAt.IsZero() {
		resp.Global.LastAt = g.LastAt.Format(time.RFC3339)
	}
	if !g.LastOkAt.IsZero() {
		resp.Global.LastOkAt = g.LastOkAt.Format(time.RFC3339)
	}
	resp.Global.LastError = g.LastError

	config.ConfigInstance.RwMutx.RLock()
	sessions := make([]config.SessionInfo, len(config.ConfigInstance.Sessions))
	copy(sessions, config.ConfigInstance.Sessions)
	config.ConfigInstance.RwMutx.RUnlock()

	resp.Accounts = make([]adminAccountResp, 0, len(sessions))
	for idx, s := range sessions {
		a := adminAccountResp{
			ID:        idx,
			MaskedKey: maskKey(s.SessionKey),
			Name:      s.Name,
			Account:   s.Account,
			Enabled:   s.Enabled,
		}

		statsMu.Lock()
		st := perSession[s.SessionKey]
		statsMu.Unlock()
		if st != nil {
			a.OK = st.OK
			a.Fail = st.Fail
			totalA := st.OK + st.Fail
			if totalA > 0 {
				a.SuccessRate = float64(st.OK) / float64(totalA)
			}
			a.Recent = st.Recent.snapshot()
			a.LastError = st.LastError
			if !st.LastAt.IsZero() {
				a.LastAt = st.LastAt.Format(time.RFC3339)
			}
		} else {
			a.Recent = newRecentWindow().snapshot()
		}

		resp.Accounts = append(resp.Accounts, a)
	}

	c.JSON(http.StatusOK, resp)
}

type addSessionReq struct {
	Name       string `json:"name"`
	Account    string `json:"account"`
	SessionKey string `json:"sessionKey"`
	OrgID      string `json:"orgID"`
}

func AdminAddSessionHandler(c *gin.Context) {
	var req addSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}

	req.SessionKey = strings.TrimSpace(req.SessionKey)
	req.Name = strings.TrimSpace(req.Name)
	req.Account = strings.TrimSpace(req.Account)
	req.OrgID = strings.TrimSpace(req.OrgID)

	if req.SessionKey == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "sessionKey 不能为空"})
		return
	}

	newSession := config.SessionInfo{
		SessionKey: req.SessionKey,
		OrgID:      req.OrgID,
		Name:       req.Name,
		Account:    req.Account,
		Enabled:    true,
	}

	config.ConfigInstance.RwMutx.Lock()
	prevLen := len(config.ConfigInstance.Sessions)
	config.ConfigInstance.Sessions = append(config.ConfigInstance.Sessions, newSession)
	// RetryCount 规则：默认等于 session 数量，最大 5
	if n := len(config.ConfigInstance.Sessions); n > 0 {
		if n > 5 {
			config.ConfigInstance.RetryCount = 5
		} else {
			config.ConfigInstance.RetryCount = n
		}
	}
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		// 回滚：移除刚刚新增的 session
		config.ConfigInstance.RwMutx.Lock()
		if len(config.ConfigInstance.Sessions) > prevLen {
			config.ConfigInstance.Sessions = append(config.ConfigInstance.Sessions[:prevLen], config.ConfigInstance.Sessions[prevLen+1:]...)
		}
		if n := len(config.ConfigInstance.Sessions); n > 0 {
			if n > 5 {
				config.ConfigInstance.RetryCount = 5
			} else {
				config.ConfigInstance.RetryCount = n
			}
		}
		config.ConfigInstance.RwMutx.Unlock()

		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"maskedKey": maskKey(newSession.SessionKey),
		"savedPath": path,
	})
}

type toggleReq struct {
	Enabled bool `json:"enabled"`
}

func AdminToggleSessionHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id 不合法"})
		return
	}

	var req toggleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求格式错误"})
		return
	}

	config.ConfigInstance.RwMutx.Lock()
	if id >= len(config.ConfigInstance.Sessions) {
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id 超出范围"})
		return
	}
	prev := config.ConfigInstance.Sessions[id].Enabled
	config.ConfigInstance.Sessions[id].Enabled = req.Enabled
	config.ConfigInstance.RwMutx.Unlock()

	path, err := config.PersistConfig()
	if err != nil {
		// 回滚
		config.ConfigInstance.RwMutx.Lock()
		if id < len(config.ConfigInstance.Sessions) {
			config.ConfigInstance.Sessions[id].Enabled = prev
		}
		config.ConfigInstance.RwMutx.Unlock()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "保存配置失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "savedPath": path})
}
