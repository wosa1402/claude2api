package openlist

import (
	"bytes"
	"claude2api/config"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"
)

type settings struct {
	baseURL   string
	username  string
	password  string
	directory string
}

func Enabled() bool {
	cfg := snapshotSettings()
	return cfg.baseURL != ""
}

func UploadFile(id string, filename string, contentType string, data []byte) (string, error) {
	cfg := snapshotSettings()
	if cfg.baseURL == "" {
		return "", fmt.Errorf("openlist webdav is not configured")
	}

	remotePath := path.Join(cfg.directory, time.Now().UTC().Format("2006/01/02"), id+"_"+filename)
	client := newHTTPClient()
	if err := ensureDir(client, cfg, path.Dir(remotePath)); err != nil {
		return "", err
	}

	req, err := newRequest(http.MethodPut, cfg, remotePath, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = int64(len(data))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload file to openlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("upload file to openlist status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return remotePath, nil
}

func DownloadFile(remotePath string) (io.ReadCloser, string, int64, error) {
	cfg := snapshotSettings()
	if cfg.baseURL == "" {
		return nil, "", 0, fmt.Errorf("openlist webdav is not configured")
	}

	client := newHTTPClient()
	req, err := newRequest(http.MethodGet, cfg, remotePath, nil)
	if err != nil {
		return nil, "", 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("download file from openlist: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", 0, fmt.Errorf("download file from openlist status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

func DeleteFile(remotePath string) error {
	cfg := snapshotSettings()
	if cfg.baseURL == "" || strings.TrimSpace(remotePath) == "" {
		return nil
	}

	client := newHTTPClient()
	req, err := newRequest(http.MethodDelete, cfg, remotePath, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete file from openlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("delete file from openlist status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func snapshotSettings() settings {
	config.ConfigInstance.RwMutx.RLock()
	defer config.ConfigInstance.RwMutx.RUnlock()

	return settings{
		baseURL:   strings.TrimSpace(config.ConfigInstance.OpenListWebDAVURL),
		username:  strings.TrimSpace(config.ConfigInstance.OpenListUsername),
		password:  strings.TrimSpace(config.ConfigInstance.OpenListPassword),
		directory: config.ConfigInstance.OpenListDirectory,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
}

func ensureDir(client *http.Client, cfg settings, dir string) error {
	clean := strings.Trim(path.Clean("/"+dir), "/")
	if clean == "" {
		return nil
	}

	current := ""
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" {
			continue
		}
		current = current + "/" + segment
		req, err := newRequest("MKCOL", cfg, current, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("create openlist directory %s: %w", current, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusConflict {
			continue
		}
		return fmt.Errorf("create openlist directory %s status %d", current, resp.StatusCode)
	}
	return nil
}

func newRequest(method string, cfg settings, remotePath string, body io.Reader) (*http.Request, error) {
	targetURL, err := buildURL(cfg.baseURL, remotePath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("build openlist request: %w", err)
	}
	if cfg.username != "" || cfg.password != "" {
		req.SetBasicAuth(cfg.username, cfg.password)
	}
	return req, nil
}

func buildURL(baseURL string, remotePath string) (string, error) {
	u, err := neturl.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("parse openlist webdav url: %w", err)
	}

	basePath := strings.TrimRight(u.Path, "/")
	relPath := strings.TrimPrefix(escapePath(remotePath), "/")
	switch {
	case basePath == "" && relPath == "":
		u.Path = "/"
	case basePath == "":
		u.Path = "/" + relPath
	case relPath == "":
		u.Path = basePath
	default:
		u.Path = basePath + "/" + relPath
	}

	return u.String(), nil
}

func escapePath(remotePath string) string {
	clean := path.Clean("/" + strings.ReplaceAll(strings.TrimSpace(remotePath), "\\", "/"))
	if clean == "/" {
		return clean
	}

	segments := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	for i, segment := range segments {
		segments[i] = neturl.PathEscape(segment)
	}
	return "/" + strings.Join(segments, "/")
}
