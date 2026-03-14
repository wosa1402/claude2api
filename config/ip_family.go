package config

import "strings"

func NormalizeGlobalIPFamily(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ipv4", "4":
		return "ipv4"
	case "ipv6", "6":
		return "ipv6"
	default:
		return "auto"
	}
}

func NormalizeSessionIPFamily(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "inherit", "global":
		return "", true
	case "auto":
		return "auto", true
	case "ipv4", "4":
		return "ipv4", true
	case "ipv6", "6":
		return "ipv6", true
	default:
		return "", false
	}
}

func EffectiveIPFamily(sessionValue string, globalValue string) string {
	if v, ok := NormalizeSessionIPFamily(sessionValue); ok && v != "" {
		return v
	}
	return NormalizeGlobalIPFamily(globalValue)
}
