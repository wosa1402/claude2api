package core

import "testing"

func TestExtractConversationFileRefsFromLocalResource(t *testing.T) {
	message := map[string]interface{}{
		"sender": "assistant",
		"content": []interface{}{
			map[string]interface{}{
				"type": "tool_result",
				"name": "present_files",
				"content": []interface{}{
					map[string]interface{}{
						"type":      "local_resource",
						"file_path": "/mnt/user-data/outputs/url-shortener.zip",
						"name":      "url-shortener",
						"mime_type": "application/zip",
					},
				},
			},
		},
	}

	refs := extractConversationFileRefs(message)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Path != "/mnt/user-data/outputs/url-shortener.zip" {
		t.Fatalf("unexpected path: %s", refs[0].Path)
	}
	if refs[0].MimeType != "application/zip" {
		t.Fatalf("unexpected mime type: %s", refs[0].MimeType)
	}
}

func TestExtractConversationFileRefsFallbackToPresentFiles(t *testing.T) {
	message := map[string]interface{}{
		"sender": "assistant",
		"content": []interface{}{
			map[string]interface{}{
				"type": "tool_use",
				"name": "present_files",
				"input": map[string]interface{}{
					"filepaths": []interface{}{
						"/mnt/user-data/outputs/hello.txt",
					},
				},
			},
		},
	}

	refs := extractConversationFileRefs(message)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Path != "/mnt/user-data/outputs/hello.txt" {
		t.Fatalf("unexpected path: %s", refs[0].Path)
	}
	if refs[0].Name != "hello.txt" {
		t.Fatalf("unexpected name: %s", refs[0].Name)
	}
}

func TestFilenameFromDisposition(t *testing.T) {
	disposition := "attachment; filename*=utf-8''url-shortener.zip"
	if got := filenameFromDisposition(disposition); got != "url-shortener.zip" {
		t.Fatalf("unexpected filename: %s", got)
	}
}
