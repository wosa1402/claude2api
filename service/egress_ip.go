package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type egressInfo struct {
	IPv4      string
	IPv6      string
	CheckedAt time.Time
	Err       string
}

var (
	egressMu    sync.Mutex
	egressCache egressInfo
)

func fetchText(url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "claude2api-admin/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioReadAllLimit(resp.Body, 128)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: limit}
	return io.ReadAll(lr)
}

func isIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

// getEgressInfo 返回缓存的公网出口 IP（best-effort）。
// - 默认缓存 5 分钟
// - 获取失败不会阻塞主功能，只返回错误信息
func getEgressInfo() egressInfo {
	egressMu.Lock()
	defer egressMu.Unlock()

	if !egressCache.CheckedAt.IsZero() && time.Since(egressCache.CheckedAt) < 5*time.Minute {
		return egressCache
	}

	info := egressInfo{CheckedAt: time.Now()}
	var errs []string

	// v4
	if v, err := fetchText("https://api.ipify.org", 3*time.Second); err != nil {
		errs = append(errs, "v4:"+err.Error())
	} else if isIP(v) {
		info.IPv4 = v
	} else {
		errs = append(errs, "v4:invalid")
	}

	// v6
	if v, err := fetchText("https://api64.ipify.org", 3*time.Second); err != nil {
		errs = append(errs, "v6:"+err.Error())
	} else if isIP(v) {
		info.IPv6 = v
	} else {
		errs = append(errs, "v6:invalid")
	}

	if len(errs) > 0 && info.IPv4 == "" && info.IPv6 == "" {
		info.Err = strings.Join(errs, " | ")
	}
	egressCache = info
	return info
}
