package service

import (
	"errors"
	"net"
	"strings"
)

func classifyErr(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// 常见 HTTP 状态码（来自 core/api.go 的 error 文案）
	for _, code := range []string{"401", "403", "404", "429", "500", "502", "503"} {
		if strings.Contains(msg, "status code: "+code) || strings.Contains(msg, "status code: "+code+",") {
			return code
		}
		if strings.Contains(msg, "status code: "+code) {
			return code
		}
		if strings.Contains(msg, "unexpected status code: "+code) {
			return code
		}
	}

	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() {
			return "timeout"
		}
		return "network"
	}

	if strings.Contains(strings.ToLower(msg), "timeout") {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(msg), "tls") || strings.Contains(strings.ToLower(msg), "x509") {
		return "tls"
	}
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return "429"
	}
	return "unknown"
}
