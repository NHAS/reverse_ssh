package client

import "testing"

func TestHTTPConnURLsUseDefaultPushPath(t *testing.T) {
	c := &HTTPConn{
		ID:       "session",
		address:  "https://example.internal",
		pushPath: "/push",
		start:    7,
	}

	if got := c.initURL("abc123"); got != "https://example.internal/push?key=abc123" {
		t.Fatalf("initURL = %q", got)
	}
	if got := c.readURL(); got != "https://example.internal/push/7?id=session" {
		t.Fatalf("readURL = %q", got)
	}
	if got := c.writeURL(); got != "https://example.internal/push?id=session" {
		t.Fatalf("writeURL = %q", got)
	}
}

func TestHTTPConnURLsUseCustomPushPath(t *testing.T) {
	c := &HTTPConn{
		ID:       "session",
		address:  "https://example.internal",
		pushPath: "/api/poll",
		start:    9,
	}

	if got := c.initURL("abc123"); got != "https://example.internal/api/poll?key=abc123" {
		t.Fatalf("initURL = %q", got)
	}
	if got := c.readURL(); got != "https://example.internal/api/poll/9?id=session" {
		t.Fatalf("readURL = %q", got)
	}
	if got := c.writeURL(); got != "https://example.internal/api/poll?id=session" {
		t.Fatalf("writeURL = %q", got)
	}
}
