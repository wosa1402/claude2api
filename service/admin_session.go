package service

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const (
	adminSIDCookieName = "admin_sid"
	adminSIDTTL        = 14 * 24 * time.Hour
)

type adminSessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

var adminSessions = &adminSessionStore{sessions: map[string]time.Time{}}

func newAdminSID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setAdminSIDCookie(w http.ResponseWriter, r *http.Request, sid string) {
	ck := &http.Cookie{
		Name:     adminSIDCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(adminSIDTTL.Seconds()),
	}
	if r != nil && r.TLS != nil {
		ck.Secure = true
	}
	http.SetCookie(w, ck)
}

func clearAdminSIDCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSIDCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func isAdminLoggedIn(r *http.Request) bool {
	if r == nil {
		return false
	}
	ck, err := r.Cookie(adminSIDCookieName)
	if err != nil || ck == nil || ck.Value == "" {
		return false
	}
	sid := ck.Value

	adminSessions.mu.Lock()
	defer adminSessions.mu.Unlock()

	exp, ok := adminSessions.sessions[sid]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(adminSessions.sessions, sid)
		return false
	}
	// 滑动过期
	adminSessions.sessions[sid] = time.Now().Add(adminSIDTTL)
	return true
}

// AdminIsLoggedIn 用于中间件判断当前请求是否已登录管理后台
func AdminIsLoggedIn(r *http.Request) bool {
	return isAdminLoggedIn(r)
}

func createAdminSession(w http.ResponseWriter, r *http.Request) error {
	sid, err := newAdminSID()
	if err != nil {
		return err
	}
	adminSessions.mu.Lock()
	adminSessions.sessions[sid] = time.Now().Add(adminSIDTTL)
	adminSessions.mu.Unlock()

	setAdminSIDCookie(w, r, sid)
	return nil
}

func destroyAdminSession(r *http.Request) {
	if r == nil {
		return
	}
	ck, err := r.Cookie(adminSIDCookieName)
	if err != nil || ck == nil || ck.Value == "" {
		return
	}
	adminSessions.mu.Lock()
	delete(adminSessions.sessions, ck.Value)
	adminSessions.mu.Unlock()
}
