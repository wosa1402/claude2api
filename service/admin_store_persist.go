package service

import (
	"claude2api/config"
	"claude2api/logger"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	statsPersistVersion  = 1
	statsPersistFilename = "admin_stats.json"

	// 10s 一次快照：对磁盘/性能压力极小，且足够保证“重启不丢冷却/统计”。
	statsPersistInterval = 10 * time.Second
)

var statsPersistOnce sync.Once
var statsChangeCounter atomic.Uint64

// markStatsDirty 标记统计数据发生变化（用于触发后台落盘）。
func markStatsDirty() {
	statsChangeCounter.Add(1)
}

func init() {
	startStatsPersistence()
}

func startStatsPersistence() {
	statsPersistOnce.Do(func() {
		if err := loadStatsFromDisk(); err != nil {
			logger.Error("加载管理后台统计失败：%v", err)
		}
		go statsPersistLoop()
	})
}

func statsPersistLoop() {
	ticker := time.NewTicker(statsPersistInterval)
	defer ticker.Stop()

	var lastFlushed uint64
	for range ticker.C {
		cur := statsChangeCounter.Load()
		if cur == lastFlushed {
			continue
		}
		if err := flushStatsToDisk(); err != nil {
			logger.Error("落盘管理后台统计失败：%v", err)
			continue
		}
		lastFlushed = cur
	}
}

type persistedRecentWindow struct {
	Data   []int8 `json:"data"`
	Cursor int    `json:"cursor"`
}

type persistedAccountStat struct {
	OK            int                   `json:"ok"`
	Fail          int                   `json:"fail"`
	FailStreak    int                   `json:"failStreak"`
	CooldownUntil time.Time             `json:"cooldownUntil"`
	Recent        persistedRecentWindow `json:"recent"`
	LastAt        time.Time             `json:"lastAt"`
	LastError     string                `json:"lastError"`
	LastRemoteIP  string                `json:"lastRemoteIP"`
	LastRemoteAt  time.Time             `json:"lastRemoteAt"`
	LastEgressIP  string                `json:"lastEgressIP"`
	LastEgressAt  time.Time             `json:"lastEgressAt"`
}

type persistedGlobalStat struct {
	OK        int       `json:"ok"`
	Fail      int       `json:"fail"`
	LastAt    time.Time `json:"lastAt"`
	LastOkAt  time.Time `json:"lastOkAt"`
	LastError string    `json:"lastError"`
}

type persistedAdminStats struct {
	Version  int                             `json:"version"`
	SavedAt  time.Time                       `json:"savedAt"`
	Global   persistedGlobalStat             `json:"global"`
	Sessions map[string]persistedAccountStat `json:"sessions"` // sessionKeyHash -> stat
}

func hashSessionKey(sessionKey string) string {
	sum := sha256.Sum256([]byte(sessionKey))
	return hex.EncodeToString(sum[:])
}

func defaultPersistDir() string {
	// 优先：已有 config.yaml 所在目录（与配置落盘位置一致）
	if wd, err := os.Getwd(); err == nil && wd != "" {
		if _, err := os.Stat(filepath.Join(wd, "config.yaml")); err == nil {
			return wd
		}
		// 默认优先工作目录，避免 go run 时写到临时目录
		return wd
	}
	execDir := filepath.Dir(os.Args[0])
	if execDir != "" && execDir != "." {
		if _, err := os.Stat(filepath.Join(execDir, "config.yaml")); err == nil {
			return execDir
		}
	}
	if execDir != "" && execDir != "." {
		return execDir
	}
	return "."
}

func statsPersistPath() string {
	// 允许用户显式指定落盘路径（便于 Docker volume 挂载）
	if v := strings.TrimSpace(os.Getenv("ADMIN_STATS_PATH")); v != "" {
		return v
	}
	return filepath.Join(defaultPersistDir(), statsPersistFilename)
}

func toPersistedRecent(w recentWindow) persistedRecentWindow {
	data := make([]int8, recentWindowSize)
	copy(data, w.data[:])
	cursor := w.cursor
	if cursor < 0 || cursor >= recentWindowSize {
		cursor = 0
	}
	return persistedRecentWindow{
		Data:   data,
		Cursor: cursor,
	}
}

func fromPersistedRecent(p persistedRecentWindow) recentWindow {
	w := newRecentWindow()
	if len(p.Data) == recentWindowSize {
		for i, v := range p.Data {
			if v == 1 || v == 0 || v == -1 {
				w.data[i] = v
			}
		}
	}
	if p.Cursor >= 0 && p.Cursor < recentWindowSize {
		w.cursor = p.Cursor
	}
	return w
}

func snapshotStatsForPersist() persistedAdminStats {
	out := persistedAdminStats{
		Version:  statsPersistVersion,
		SavedAt:  time.Now(),
		Sessions: map[string]persistedAccountStat{},
	}

	statsMu.Lock()
	out.Global = persistedGlobalStat{
		OK:        global.OK,
		Fail:      global.Fail,
		LastAt:    global.LastAt,
		LastOkAt:  global.LastOkAt,
		LastError: global.LastError,
	}
	for sessionKey, st := range perSession {
		if st == nil || strings.TrimSpace(sessionKey) == "" {
			continue
		}
		out.Sessions[hashSessionKey(sessionKey)] = persistedAccountStat{
			OK:            st.OK,
			Fail:          st.Fail,
			FailStreak:    st.FailStreak,
			CooldownUntil: st.CooldownUntil,
			Recent:        toPersistedRecent(st.Recent),
			LastAt:        st.LastAt,
			LastError:     st.LastError,
			LastRemoteIP:  st.LastRemoteIP,
			LastRemoteAt:  st.LastRemoteAt,
			LastEgressIP:  st.LastEgressIP,
			LastEgressAt:  st.LastEgressAt,
		}
	}
	statsMu.Unlock()

	return out
}

func flushStatsToDisk() error {
	ps := snapshotStatsForPersist()
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json 失败: %w", err)
	}
	b = append(b, '\n')

	path := statsPersistPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".admin_stats.json.*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod 失败: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename 失败: %w", err)
	}
	return nil
}

func loadStatsFromDisk() error {
	path := statsPersistPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}

	var ps persistedAdminStats
	if err := json.Unmarshal(b, &ps); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	if ps.Version != statsPersistVersion {
		return fmt.Errorf("不支持的统计版本: %d", ps.Version)
	}

	// 只加载当前配置里的 session（通过 hash 匹配），避免把敏感 sessionKey 写入持久化文件。
	config.ConfigInstance.RwMutx.RLock()
	sessions := make([]config.SessionInfo, len(config.ConfigInstance.Sessions))
	copy(sessions, config.ConfigInstance.Sessions)
	config.ConfigInstance.RwMutx.RUnlock()

	statsMu.Lock()
	global.OK = ps.Global.OK
	global.Fail = ps.Global.Fail
	global.LastAt = ps.Global.LastAt
	global.LastOkAt = ps.Global.LastOkAt
	global.LastError = ps.Global.LastError

	for _, s := range sessions {
		sk := strings.TrimSpace(s.SessionKey)
		if sk == "" {
			continue
		}
		pst, ok := ps.Sessions[hashSessionKey(sk)]
		if !ok {
			continue
		}
		perSession[sk] = &accountStat{
			OK:            pst.OK,
			Fail:          pst.Fail,
			FailStreak:    pst.FailStreak,
			CooldownUntil: pst.CooldownUntil,
			Recent:        fromPersistedRecent(pst.Recent),
			LastAt:        pst.LastAt,
			LastError:     pst.LastError,
			LastRemoteIP:  pst.LastRemoteIP,
			LastRemoteAt:  pst.LastRemoteAt,
			LastEgressIP:  pst.LastEgressIP,
			LastEgressAt:  pst.LastEgressAt,
		}
	}
	statsMu.Unlock()

	logger.Info("已加载管理后台统计：%s", path)
	return nil
}
