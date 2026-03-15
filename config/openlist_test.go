package config

import "testing"

func TestNormalizeOpenListDirectory(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: "/claude2api"},
		{name: "nested", input: "exports/files", want: "/exports/files"},
		{name: "windows", input: "\\exports\\files\\", want: "/exports/files"},
		{name: "root", input: "/", want: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeOpenListDirectory(tc.input); got != tc.want {
				t.Fatalf("NormalizeOpenListDirectory(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
