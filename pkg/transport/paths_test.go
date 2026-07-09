package transport

import "testing"

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{name: "fallback", fallback: "/ws", want: "/ws"},
		{name: "adds leading slash", input: "push", fallback: "/ws", want: "/push"},
		{name: "trims trailing slash", input: "/push/", fallback: "/ws", want: "/push"},
		{name: "keeps root", input: "/", fallback: "/ws", want: "/"},
		{name: "trims whitespace", input: "  api/push/  ", fallback: "/ws", want: "/api/push"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePath(tt.input, tt.fallback); got != tt.want {
				t.Fatalf("NormalizePath(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestJoinPushPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		suffix string
		want   string
	}{
		{name: "joins suffix", path: "/custom/", suffix: "123", want: "/custom/123"},
		{name: "trims suffix slash", path: "/custom", suffix: "/123", want: "/custom/123"},
		{name: "empty suffix returns path", path: "/custom", want: "/custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinPushPath(tt.path, tt.suffix); got != tt.want {
				t.Fatalf("JoinPushPath(%q, %q) = %q, want %q", tt.path, tt.suffix, got, tt.want)
			}
		})
	}
}
