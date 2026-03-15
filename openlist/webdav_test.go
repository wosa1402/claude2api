package openlist

import "testing"

func TestEscapePath(t *testing.T) {
	got := escapePath("/claude2api/2026/03/15/hello world.txt")
	want := "/claude2api/2026/03/15/hello%20world.txt"
	if got != want {
		t.Fatalf("unexpected escaped path: %s", got)
	}
}

func TestBuildURL(t *testing.T) {
	got, err := buildURL("https://openlist.example.com/dav/", "/claude2api/file.zip")
	if err != nil {
		t.Fatalf("buildURL returned error: %v", err)
	}
	want := "https://openlist.example.com/dav/claude2api/file.zip"
	if got != want {
		t.Fatalf("unexpected url: %s", got)
	}
}
