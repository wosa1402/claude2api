package core

import (
	"claude2api/genfile"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type conversationFileRef struct {
	Path     string
	Name     string
	MimeType string
}

var errNoConversationFiles = errors.New("no conversation files found")

func (c *Client) ExportConversationFiles(conversationID string, gc *gin.Context) ([]genfile.PublicFile, error) {
	refs, err := c.listConversationFiles(conversationID)
	if err != nil {
		if errors.Is(err, errNoConversationFiles) {
			return nil, nil
		}
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}

	files := make([]genfile.PublicFile, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	var lastErr error

	for _, ref := range refs {
		if ref.Path == "" {
			continue
		}
		if _, ok := seen[ref.Path]; ok {
			continue
		}
		seen[ref.Path] = struct{}{}

		data, contentType, filename, err := c.DownloadConversationFile(conversationID, ref.Path)
		if err != nil {
			lastErr = err
			continue
		}

		if filename == "" {
			filename = ref.Name
		}
		if filename == "" {
			filename = path.Base(ref.Path)
		}
		if contentType == "" {
			contentType = ref.MimeType
		}

		record, err := genfile.Store(filename, contentType, data)
		if err != nil {
			lastErr = err
			continue
		}
		files = append(files, genfile.ToPublic(record, gc))
	}

	if len(files) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return files, nil
}

func (c *Client) DownloadConversationFile(conversationID string, filePath string) ([]byte, string, string, error) {
	if c.orgID == "" {
		return nil, "", "", errors.New("organization ID not set")
	}
	if strings.TrimSpace(filePath) == "" {
		return nil, "", "", errors.New("empty conversation file path")
	}

	url := fmt.Sprintf(
		"https://claude.ai/api/organizations/%s/conversations/%s/wiggle/download-file?path=%s",
		c.orgID,
		conversationID,
		neturl.QueryEscape(filePath),
	)

	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("https://claude.ai/chat/%s", conversationID)).
		SetHeader("accept", "*/*").
		Get(url)
	if err != nil {
		return nil, "", "", fmt.Errorf("download conversation file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("download conversation file status %d", resp.StatusCode)
	}

	filename := filenameFromDisposition(resp.Header.Get("content-disposition"))
	contentType := resp.Header.Get("content-type")
	if contentType == "" {
		contentType = http.DetectContentType(resp.Bytes())
	}

	return resp.Bytes(), contentType, filename, nil
}

func (c *Client) listConversationFiles(conversationID string) ([]conversationFileRef, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		refs, err := c.listConversationFilesOnce(conversationID)
		if err == nil && len(refs) > 0 {
			return refs, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errNoConversationFiles
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return nil, lastErr
}

func (c *Client) listConversationFilesOnce(conversationID string) ([]conversationFileRef, error) {
	if c.orgID == "" {
		return nil, errors.New("organization ID not set")
	}

	url := fmt.Sprintf(
		"https://claude.ai/api/organizations/%s/chat_conversations/%s?tree=True&rendering_mode=messages&render_all_tools=true&consistency=eventual",
		c.orgID,
		conversationID,
	)
	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("https://claude.ai/chat/%s", conversationID)).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch conversation tree: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch conversation tree status %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("parse conversation tree: %w", err)
	}

	messages, ok := payload["chat_messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return nil, errors.New("conversation tree has no chat_messages")
	}

	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		if asString(message["sender"]) != "assistant" {
			continue
		}
		return extractConversationFileRefs(message), nil
	}

	return nil, errors.New("assistant message not found in conversation tree")
}

func extractConversationFileRefs(message map[string]interface{}) []conversationFileRef {
	contentItems, ok := message["content"].([]interface{})
	if !ok || len(contentItems) == 0 {
		return nil
	}

	refs := make([]conversationFileRef, 0)

	for _, rawItem := range contentItems {
		item, ok := rawItem.(map[string]interface{})
		if !ok || asString(item["type"]) != "tool_result" {
			continue
		}
		for _, rawContent := range asSlice(item["content"]) {
			part, ok := rawContent.(map[string]interface{})
			if !ok || asString(part["type"]) != "local_resource" {
				continue
			}
			filePath := asString(part["file_path"])
			if filePath == "" {
				continue
			}
			refs = append(refs, conversationFileRef{
				Path:     filePath,
				Name:     asString(part["name"]),
				MimeType: asString(part["mime_type"]),
			})
		}
	}
	if len(refs) > 0 {
		return refs
	}

	for _, rawItem := range contentItems {
		item, ok := rawItem.(map[string]interface{})
		if !ok || asString(item["type"]) != "tool_use" {
			continue
		}

		name := asString(item["name"])
		input := asMap(item["input"])
		switch name {
		case "present_files":
			for _, rawPath := range asSlice(input["filepaths"]) {
				filePath := asString(rawPath)
				if filePath == "" {
					continue
				}
				refs = append(refs, conversationFileRef{
					Path: filePath,
					Name: path.Base(filePath),
				})
			}
		case "create_file":
			filePath := asString(input["path"])
			if filePath == "" {
				continue
			}
			refs = append(refs, conversationFileRef{
				Path: filePath,
				Name: path.Base(filePath),
			})
		}
	}

	return refs
}

func formatConversationFilesText(files []genfile.PublicFile) string {
	if len(files) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\nDownload files:\n")
	for _, file := range files {
		b.WriteString("- ")
		b.WriteString(file.Name)
		b.WriteString(": ")
		b.WriteString(file.URL)
		b.WriteByte('\n')
	}
	return b.String()
}

func filenameFromDisposition(disposition string) string {
	if disposition == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return ""
	}
	if filename := strings.TrimSpace(params["filename*"]); filename != "" {
		filename = strings.TrimPrefix(filename, "utf-8''")
		if decoded, err := neturl.QueryUnescape(filename); err == nil {
			return decoded
		}
		return filename
	}
	return strings.TrimSpace(params["filename"])
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}
