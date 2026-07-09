package mux

import (
	"testing"

	"github.com/NHAS/reverse_ssh/pkg/mux/protocols"
)

func TestClassifyHTTPRequestDefaultPaths(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   protocols.Type
	}{
		{name: "websocket", header: "GET /ws HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.Websockets},
		{name: "poll head", header: "HEAD /push?key=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "poll get", header: "GET /push/123?id=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "poll post", header: "POST /push?id=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "download", header: "GET /payload.exe HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTPDownload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyHTTPRequest([]byte(tt.header), "", ""); got != tt.want {
				t.Fatalf("classifyHTTPRequest = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyHTTPRequestCustomPaths(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   protocols.Type
	}{
		{name: "websocket", header: "GET /socket HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.Websockets},
		{name: "poll head", header: "HEAD /api/poll?key=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "poll get", header: "GET /api/poll/123?id=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "poll post", header: "POST /api/poll?id=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTP},
		{name: "default websocket becomes download", header: "GET /ws HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTPDownload},
		{name: "default push becomes download", header: "HEAD /push?key=abc HTTP/1.1\r\nHost: example\r\n\r\n", want: protocols.HTTPDownload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyHTTPRequest([]byte(tt.header), "/socket", "/api/poll"); got != tt.want {
				t.Fatalf("classifyHTTPRequest = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyHTTPRequestIgnoresQueryString(t *testing.T) {
	got := classifyHTTPRequest([]byte("HEAD /api/poll?key=abc HTTP/1.1\r\n"), "/socket", "/api/poll")
	if got != protocols.HTTP {
		t.Fatalf("classifyHTTPRequest = %q, want %q", got, protocols.HTTP)
	}
}
