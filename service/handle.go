package service

import (
	"claude2api/config"
	"claude2api/core"
	"claude2api/logger"
	"claude2api/model"
	"claude2api/utils"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthCheckHandler handles the health check endpoint
func HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func MoudlesHandler(c *gin.Context) {
	models := []map[string]interface{}{
		{"id": "claude-sonnet-4-20250514"},
		{"id": "claude-sonnet-4-5-20250929"},
		{"id": "claude-sonnet-4-6"},
		{"id": "claude-haiku-4-5-20251001"},
	}

	extendedModels := make([]map[string]interface{}, 0, len(models)*2)
	for _, m := range models {
		// 保留原有 id
		extendedModels = append(extendedModels, m)
		// 追加 -think 版本
		if id, ok := m["id"].(string); ok {
			extendedModels = append(extendedModels, map[string]interface{}{
				"id": id + "-think",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": extendedModels,
	})
}

// ChatCompletionsHandler handles the chat completions endpoint
func ChatCompletionsHandler(c *gin.Context) {
	useMirror, exist := c.Get("UseMirrorApi")
	if exist && useMirror.(bool) {
		MirrorChatHandler(c)
		return
	}

	// Parse and validate request
	req, err := parseAndValidateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Process messages into prompt and extract images
	processor := utils.NewChatRequestProcessor()
	processor.ProcessMessages(req.Messages)

	// Get model or use default
	model := getModelOrDefault(req.Model)

	// 根据模型选择 session 池：
	// - Claude 4（sonnet-4-20250514）优先走低权重池（low）
	// - 4.5/4.6/haiku 走正常权重池（high）
	baseModel := strings.TrimSuffix(model, "-think")
	wantPool := "high"
	if baseModel == "claude-sonnet-4-20250514" {
		wantPool = "low"
	}

	// 构建候选 session 列表（按 pool + enabled 过滤）
	config.ConfigInstance.RwMutx.RLock()
	sessionsSnap := make([]config.SessionInfo, len(config.ConfigInstance.Sessions))
	copy(sessionsSnap, config.ConfigInstance.Sessions)
	retryCount := config.ConfigInstance.RetryCount
	config.ConfigInstance.RwMutx.RUnlock()

	now := time.Now()
	candidates := make([]config.SessionInfo, 0, len(sessionsSnap))
	poolEnabledTotal := 0
	cooldownSkipped := 0
	var earliestCooldown time.Time

	for _, s := range sessionsSnap {
		if !s.Enabled {
			continue
		}
		pool := strings.TrimSpace(s.Pool)
		if pool == "" {
			pool = "low"
		}
		if pool == wantPool {
			poolEnabledTotal++

			// 冷却中的账号直接跳过
			statsMu.Lock()
			st := perSession[s.SessionKey]
			var cd time.Time
			if st != nil {
				cd = st.CooldownUntil
			}
			statsMu.Unlock()

			if !cd.IsZero() && cd.After(now) {
				cooldownSkipped++
				if earliestCooldown.IsZero() || cd.Before(earliestCooldown) {
					earliestCooldown = cd
				}
				continue
			}
			candidates = append(candidates, s)
		}
	}

	if len(candidates) == 0 {
		// 池子里有账号，但都在冷却
		if poolEnabledTotal > 0 && cooldownSkipped == poolEnabledTotal && !earliestCooldown.IsZero() {
			sec := int(time.Until(earliestCooldown).Seconds())
			if sec < 1 {
				sec = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", sec))
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error: fmt.Sprintf("所有账号处于冷却中，最早恢复时间：%s", earliestCooldown.Format(time.RFC3339)),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: fmt.Sprintf("无可用账号：pool=%s（请在 /admin 为 session 设置 pool）", wantPool),
		})
		return
	}

	sr := config.SrHigh
	if wantPool == "low" {
		sr = config.SrLow
	}
	start := sr.NextIndex(len(candidates))

	// Attempt with retry mechanism（优先用配置的 retryCount，但不超过候选数量）
	maxTry := retryCount
	if maxTry <= 0 || maxTry > len(candidates) {
		maxTry = len(candidates)
	}
	for i := 0; i < maxTry; i++ {
		session := candidates[(start+i)%len(candidates)]

		logger.Info(fmt.Sprintf("Using session for model %s: %s", model, session.SessionKey))
		if i > 0 {
			processor.Prompt.Reset()
			processor.Prompt.WriteString(processor.RootPrompt.String())
		}
		// Initialize client and process request
		ok, errType, remoteIP, remoteAt, egressIP, egressAt := handleChatRequest(c, session, model, processor, req.Stream)
		recordAttempt(session, ok, errType, remoteIP, remoteAt, egressIP, egressAt)
		if ok {
			return // Success, exit the retry loop
		}

		// If we're here, the request failed - retry with another session
		logger.Info("Retrying another session")
	}

	logger.Error("Failed for all retries")
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: "Failed to process request after multiple attempts"})
}

func MirrorChatHandler(c *gin.Context) {
	if !config.ConfigInstance.EnableMirrorApi {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Mirror API is not enabled",
		})
		return
	}

	// Parse and validate request
	req, err := parseAndValidateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Process messages into prompt and extract images
	processor := utils.NewChatRequestProcessor()
	processor.ProcessMessages(req.Messages)

	// Get model or use default
	model := getModelOrDefault(req.Model)

	// Extract session info from auth header
	session, err := extractSessionFromAuthHeader(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("Invalid authorization: %v", err),
		})
		return
	}

	// Process the request with the provided session
	ok, errType, remoteIP, remoteAt, egressIP, egressAt := handleChatRequest(c, session, model, processor, req.Stream)
	recordAttempt(session, ok, errType, remoteIP, remoteAt, egressIP, egressAt)
	if !ok {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to process request",
		})
		return
	}
}

// Helper functions

func parseAndValidateRequest(c *gin.Context) (*model.ChatCompletionRequest, error) {
	var req model.ChatCompletionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("Invalid request: %v", err),
		})
		return nil, err
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "No messages provided",
		})
		return nil, fmt.Errorf("no messages provided")
	}

	return &req, nil
}

func getModelOrDefault(model string) string {
	if model == "" {
		return "claude-3-7-sonnet-20250219"
	}
	return model
}

func extractSessionFromAuthHeader(c *gin.Context) (config.SessionInfo, error) {
	authInfo := c.Request.Header.Get("Authorization")
	authInfo = strings.TrimPrefix(authInfo, "Bearer ")

	if authInfo == "" {
		return config.SessionInfo{SessionKey: "", OrgID: "", Enabled: true}, fmt.Errorf("missing authorization header")
	}

	if strings.Contains(authInfo, ":") {
		parts := strings.Split(authInfo, ":")
		return config.SessionInfo{SessionKey: parts[0], OrgID: parts[1], Enabled: true}, nil
	}

	return config.SessionInfo{SessionKey: authInfo, OrgID: "", Enabled: true}, nil
}

func handleChatRequest(c *gin.Context, session config.SessionInfo, model string, processor *utils.ChatRequestProcessor, stream bool) (bool, string, string, time.Time, string, time.Time) {
	// Initialize the Claude client
	config.ConfigInstance.RwMutx.RLock()
	proxy := config.ConfigInstance.Proxy
	forceIPFamily := config.ConfigInstance.ForceIPFamily
	recordEgressIP := config.ConfigInstance.RecordEgressIP
	config.ConfigInstance.RwMutx.RUnlock()
	claudeClient := core.NewClient(session.SessionKey, proxy, model, forceIPFamily)

	// 如果开启了出口 IP 记录，则检测一次
	var egressIP string
	var egressAt time.Time
	if recordEgressIP {
		egressIP = FetchEgressIPOnce(forceIPFamily)
		if egressIP != "" {
			egressAt = time.Now()
		}
	}

	// Get org ID if not already set
	if session.OrgID == "" {
		orgId, err := claudeClient.GetOrgID()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get org ID: %v", err))
			remoteIP, remoteAt := claudeClient.LastDialRemoteIP()
			return false, classifyErr(err), remoteIP, remoteAt, egressIP, egressAt
		}
		session.OrgID = orgId
		config.ConfigInstance.SetSessionOrgID(session.SessionKey, session.OrgID)
	}

	claudeClient.SetOrgID(session.OrgID)

	// Upload images if any
	if len(processor.ImgDataList) > 0 {
		err := claudeClient.UploadFile(processor.ImgDataList)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to upload file: %v", err))
			remoteIP, remoteAt := claudeClient.LastDialRemoteIP()
			return false, classifyErr(err), remoteIP, remoteAt, egressIP, egressAt
		}
	}

	// Handle large context if needed
	if processor.Prompt.Len() > config.ConfigInstance.MaxChatHistoryLength {
		claudeClient.SetBigContext(processor.Prompt.String())
		processor.ResetForBigContext()
		logger.Info(fmt.Sprintf("Prompt length exceeds max limit (%d), using file context", config.ConfigInstance.MaxChatHistoryLength))
	}

	// Create conversation
	conversationID, err := claudeClient.CreateConversation()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create conversation: %v", err))
		remoteIP, remoteAt := claudeClient.LastDialRemoteIP()
		return false, classifyErr(err), remoteIP, remoteAt, egressIP, egressAt
	}

	// Send message
	if _, err := claudeClient.SendMessage(conversationID, processor.Prompt.String(), stream, c); err != nil {
		logger.Error(fmt.Sprintf("Failed to send message: %v", err))
		go cleanupConversation(claudeClient, conversationID, 3)
		remoteIP, remoteAt := claudeClient.LastDialRemoteIP()
		return false, classifyErr(err), remoteIP, remoteAt, egressIP, egressAt
	}

	// Clean up conversation if enabled
	if config.ConfigInstance.ChatDelete {
		go cleanupConversation(claudeClient, conversationID, 3)
	}

	remoteIP, remoteAt := claudeClient.LastDialRemoteIP()
	return true, "", remoteIP, remoteAt, egressIP, egressAt
}

func cleanupConversation(client *core.Client, conversationID string, retry int) {
	for i := 0; i < retry; i++ {
		if err := client.DeleteConversation(conversationID); err != nil {
			logger.Error(fmt.Sprintf("Failed to delete conversation: %v", err))
			time.Sleep(2 * time.Second)
			continue
		}
		logger.Info(fmt.Sprintf("Successfully deleted conversation: %s", conversationID))
		return // 成功后直接返回，不执行后面的错误日志
	}
	// 只有当所有重试都失败后，才会执行到这里
	logger.Error(fmt.Sprintf("Cleanup %s conversation %s failed after %d retries", client.SessionKey, conversationID, retry))
}
