package service

import (
	"claude2api/config"
	"claude2api/core"
	"claude2api/logger"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var modelIDRe = regexp.MustCompile(`claude-[a-z0-9-]+`)

func classifyModelToPool(modelID string) string {
	m := strings.ToLower(strings.TrimSpace(modelID))
	if strings.Contains(m, "-4-5-") || strings.Contains(m, "haiku-4-5") {
		return "high"
	}
	if strings.Contains(m, "sonnet-4-20250514") || strings.HasPrefix(m, "claude-sonnet-4-") {
		return "low"
	}
	// 其它未知情况默认 low，避免误把低权重放进 high
	return "low"
}

func extractModelID(text string) string {
	if text == "" {
		return ""
	}
	m := modelIDRe.FindString(strings.ToLower(text))
	return strings.TrimSpace(m)
}

func classifyAskModel() string {
	if v := strings.TrimSpace(os.Getenv("ADMIN_CLASSIFY_MODEL")); v != "" {
		return v
	}
	// 默认用 4.5 haiku，低成本且足够区分“被降级到 4”的账号
	return "claude-haiku-4-5-20251001"
}

func runSessionAutoClassify(s config.SessionInfo) (askedModel, reportedModel, suggestedPool, raw, errType, errMsg string) {
	askedModel = classifyAskModel()
	prompt := "请只输出你当前的模型ID（例如 claude-sonnet-4-20250514 或 claude-sonnet-4-5-20250929），不要输出其他内容。"

	config.ConfigInstance.RwMutx.RLock()
	proxy := config.ConfigInstance.Proxy
	config.ConfigInstance.RwMutx.RUnlock()
	config.ConfigInstance.RwMutx.RLock()
	forceIPFamily := config.ConfigInstance.ForceIPFamily
	config.ConfigInstance.RwMutx.RUnlock()
	client := core.NewClient(s.SessionKey, proxy, askedModel, forceIPFamily)

	orgID := strings.TrimSpace(s.OrgID)
	if orgID == "" {
		oid, err := client.GetOrgID()
		if err != nil {
			errType = classifyErr(err)
			errMsg = err.Error()
			return
		}
		orgID = oid
	}
	client.SetOrgID(orgID)

	convID, err := client.CreateConversation()
	if err != nil {
		errType = classifyErr(err)
		errMsg = err.Error()
		return
	}

	_, txt, err := client.SendMessageExtractText(convID, prompt)
	raw = strings.TrimSpace(txt)
	if err != nil {
		errType = classifyErr(err)
		errMsg = err.Error()
		return
	}

	reportedModel = extractModelID(raw)
	if reportedModel == "" {
		// 兜底：把原文截断写入记录，方便人工审核
		errType = "unknown"
		errMsg = "无法从回答中提取模型ID"
		return
	}
	suggestedPool = classifyModelToPool(reportedModel)
	return
}

func truncateForRecord(s string, max int) string {
	if max <= 0 {
		return ""
	}
	ss := strings.TrimSpace(s)
	if len(ss) <= max {
		return ss
	}
	return ss[:max] + "…"
}

func writeClassifyRecord(id int, askedModel, reportedModel, suggestedPool, raw, errType, errMsg string) error {
	now := time.Now().Format(time.RFC3339)
	config.ConfigInstance.RwMutx.Lock()
	defer config.ConfigInstance.RwMutx.Unlock()
	if id < 0 || id >= len(config.ConfigInstance.Sessions) {
		return fmt.Errorf("id 超出范围")
	}
	s := &config.ConfigInstance.Sessions[id]
	s.ClassifyLastAt = now
	s.ClassifyAskedModel = askedModel
	s.ClassifyReportedModel = reportedModel
	s.ClassifySuggestedPool = suggestedPool
	s.ClassifyRaw = truncateForRecord(raw, 220)
	if errMsg != "" {
		s.ClassifyError = truncateForRecord(errType+": "+errMsg, 220)
	} else {
		s.ClassifyError = ""
	}
	return nil
}

func applySuggestedPoolIfNeeded(id int, apply bool) {
	if !apply {
		return
	}
	config.ConfigInstance.RwMutx.Lock()
	defer config.ConfigInstance.RwMutx.Unlock()
	if id < 0 || id >= len(config.ConfigInstance.Sessions) {
		return
	}
	s := &config.ConfigInstance.Sessions[id]
	if s.ClassifySuggestedPool == "low" || s.ClassifySuggestedPool == "high" {
		s.Pool = s.ClassifySuggestedPool
		logger.Info(fmt.Sprintf("Applied suggested pool for session %d: %s", id, s.Pool))
	}
}
