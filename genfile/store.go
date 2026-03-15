package genfile

import (
	"claude2api/openlist"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const retention = 24 * time.Hour

type Record struct {
	ID          string
	Name        string
	Path        string
	RemotePath  string
	ContentType string
	Size        int64
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type PublicFile struct {
	ID          string
	Name        string
	URL         string
	ContentType string
	Size        int64
}

var (
	storeMu  sync.Mutex
	store    = map[string]Record{}
	storeDir string
)

func Store(name string, contentType string, data []byte) (Record, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	cleanupExpiredLocked()

	safeName := sanitizeFilename(name)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	id := uuid.New().String()
	recordPath := ""
	remotePath := ""

	if openlist.Enabled() {
		var err error
		remotePath, err = openlist.UploadFile(id, safeName, contentType, data)
		if err != nil {
			return Record{}, err
		}
	} else {
		dir, err := ensureStoreDir()
		if err != nil {
			return Record{}, err
		}
		recordPath = filepath.Join(dir, id+"_"+safeName)
		if err := os.WriteFile(recordPath, data, 0600); err != nil {
			return Record{}, fmt.Errorf("write generated file: %w", err)
		}
	}

	now := time.Now()
	record := Record{
		ID:          id,
		Name:        safeName,
		Path:        recordPath,
		RemotePath:  remotePath,
		ContentType: contentType,
		Size:        int64(len(data)),
		CreatedAt:   now,
		ExpiresAt:   now.Add(retention),
	}
	store[id] = record
	return record, nil
}

func Get(id string) (Record, bool) {
	storeMu.Lock()
	defer storeMu.Unlock()

	cleanupExpiredLocked()

	record, ok := store[id]
	if !ok {
		return Record{}, false
	}
	if record.RemotePath != "" {
		return record, true
	}
	if _, err := os.Stat(record.Path); err != nil {
		delete(store, id)
		return Record{}, false
	}
	return record, true
}

func PublicURL(c *gin.Context, id string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	host = strings.TrimSpace(strings.Split(host, ",")[0])

	return fmt.Sprintf("%s://%s/v1/files/%s", scheme, host, neturl.PathEscape(id))
}

func ToPublic(record Record, c *gin.Context) PublicFile {
	return PublicFile{
		ID:          record.ID,
		Name:        record.Name,
		URL:         PublicURL(c, record.ID),
		ContentType: record.ContentType,
		Size:        record.Size,
	}
}

func ensureStoreDir() (string, error) {
	if storeDir != "" {
		return storeDir, nil
	}

	dir := filepath.Join(os.TempDir(), "claude2api_generated_files")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create generated file dir: %w", err)
	}
	storeDir = dir
	return storeDir, nil
}

func cleanupExpiredLocked() {
	now := time.Now()
	for id, record := range store {
		if now.Before(record.ExpiresAt) {
			continue
		}
		if record.RemotePath != "" {
			_ = openlist.DeleteFile(record.RemotePath)
		} else if record.Path != "" {
			_ = os.Remove(record.Path)
		}
		delete(store, id)
	}
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "download.bin"
	}

	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\r", "_",
		"\n", "_",
		"\t", "_",
	)
	name = replacer.Replace(name)
	if name == "" {
		return "download.bin"
	}
	return name
}
