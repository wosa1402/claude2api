package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func getPersistPath() (string, error) {
	// 如果已有 config.yaml（可执行目录或工作目录），优先写回同一路径
	if exists, p := configFileExists(); exists && p != "" {
		return p, nil
	}

	// 默认优先工作目录，避免 systemd 二进制部署时写回到 /usr/local/bin
	wd, err := os.Getwd()
	if err == nil && wd != "" {
		return filepath.Join(wd, "config.yaml"), nil
	}

	// 其次使用可执行文件所在目录
	execDir := filepath.Dir(os.Args[0])
	if execDir != "" && execDir != "." {
		return filepath.Join(execDir, "config.yaml"), nil
	}

	// 最后退回当前目录
	return "config.yaml", nil
}

func PersistConfig() (string, error) {
	if ConfigInstance == nil {
		return "", fmt.Errorf("config not initialized")
	}
	path, err := getPersistPath()
	if err != nil {
		return "", err
	}

	// 拷贝一份当前配置（避免并发修改）
	ConfigInstance.RwMutx.RLock()
	c := *ConfigInstance
	c.Sessions = append([]SessionInfo(nil), ConfigInstance.Sessions...)
	ConfigInstance.RwMutx.RUnlock()

	// RetryCount 兜底：默认等于 session 数量，最大 5
	if c.RetryCount <= 0 {
		if n := len(c.Sessions); n > 0 {
			if n > 5 {
				c.RetryCount = 5
			} else {
				c.RetryCount = n
			}
		}
	}

	b, err := yaml.Marshal(&c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal yaml: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config.yaml.*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to chmod: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("failed to close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("failed to rename: %w", err)
	}
	return path, nil
}
