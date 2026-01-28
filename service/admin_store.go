package service

import (
	"claude2api/config"
	"os"
	"strconv"
	"sync"
	"time"
)

const recentWindowSize = 30

type recentWindow struct {
	data   [recentWindowSize]int8 // 1=成功 0=失败 -1=无数据
	cursor int
}

func newRecentWindow() recentWindow {
	w := recentWindow{}
	for i := 0; i < recentWindowSize; i++ {
		w.data[i] = -1
	}
	return w
}

func (w *recentWindow) push(ok bool) {
	if ok {
		w.data[w.cursor] = 1
	} else {
		w.data[w.cursor] = 0
	}
	w.cursor = (w.cursor + 1) % recentWindowSize
}

func (w recentWindow) snapshot() []interface{} {
	out := make([]interface{}, 0, recentWindowSize)
	// 输出为“按时间从旧到新”的顺序
	for i := 0; i < recentWindowSize; i++ {
		idx := (w.cursor + i) % recentWindowSize
		switch w.data[idx] {
		case 1:
			out = append(out, true)
		case 0:
			out = append(out, false)
		default:
			out = append(out, nil)
		}
	}
	return out
}

type accountStat struct {
	OK            int
	Fail          int
	FailStreak    int
	CooldownUntil time.Time
	Recent        recentWindow
	LastAt        time.Time
	LastError     string
}

type globalStat struct {
	OK        int
	Fail      int
	LastAt    time.Time
	LastOkAt  time.Time
	LastError string
}

var statsMu sync.Mutex
var perSession = map[string]*accountStat{}
var global = globalStat{}

func cooldownDuration() time.Duration {
	// 默认 60 分钟，可通过 COOLDOWN_MINUTES 覆盖
	minutes, err := strconv.Atoi(os.Getenv("COOLDOWN_MINUTES"))
	if err != nil || minutes <= 0 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

func maskKey(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 10 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func getOrInitSessionStat(sessionKey string) *accountStat {
	st, ok := perSession[sessionKey]
	if ok {
		return st
	}
	ns := &accountStat{Recent: newRecentWindow()}
	perSession[sessionKey] = ns
	return ns
}

func recordAttempt(session config.SessionInfo, ok bool, errType string) {
	statsMu.Lock()
	defer statsMu.Unlock()

	now := time.Now()
	s := getOrInitSessionStat(session.SessionKey)
	s.LastAt = now
	if ok {
		s.OK++
		s.LastError = ""
		s.FailStreak = 0
		s.CooldownUntil = time.Time{}
		s.Recent.push(true)
	} else {
		s.Fail++
		s.LastError = errType
		s.FailStreak++
		if errType == "429" {
			s.CooldownUntil = now.Add(cooldownDuration())
		}
		s.Recent.push(false)
	}

	global.LastAt = now
	if ok {
		global.OK++
		global.LastOkAt = now
	} else {
		global.Fail++
		global.LastError = errType
	}
}
