package config

import (
	"path"
	"strings"
)

const defaultOpenListDirectory = "/claude2api"

func NormalizeOpenListDirectory(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return defaultOpenListDirectory
	}

	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." || cleaned == "" {
		return defaultOpenListDirectory
	}
	return cleaned
}
