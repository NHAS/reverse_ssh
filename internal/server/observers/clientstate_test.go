package observers

import (
	"strings"
	"testing"
	"time"
)

func TestClientStateJSONIncludesOptionalTransportFields(t *testing.T) {
	payload, err := ClientState{
		Status:               "connected",
		ID:                   "id1",
		IP:                   "198.51.100.10:0",
		HostName:             "host",
		Version:              "SSH-test",
		Timestamp:            time.Unix(0, 0).UTC(),
		Transport:            "wss",
		PublicKeyFingerprint: "fp",
		ProxySourceIP:        "192.0.2.10",
	}.Json()
	if err != nil {
		t.Fatal(err)
	}

	body := string(payload)
	for _, want := range []string{
		`"Transport":"wss"`,
		`"PublicKeyFingerprint":"fp"`,
		`"ProxySourceIP":"192.0.2.10"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("payload missing %s: %s", want, body)
		}
	}
}

func TestClientStateJSONOmitsEmptyOptionalTransportFields(t *testing.T) {
	payload, err := ClientState{
		Status:    "connected",
		ID:        "id1",
		IP:        "198.51.100.10:0",
		HostName:  "host",
		Version:   "SSH-test",
		Timestamp: time.Unix(0, 0).UTC(),
	}.Json()
	if err != nil {
		t.Fatal(err)
	}

	body := string(payload)
	for _, field := range []string{"Transport", "PublicKeyFingerprint", "ProxySourceIP"} {
		if strings.Contains(body, field) {
			t.Fatalf("payload should omit %s: %s", field, body)
		}
	}
}
