package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/NHAS/reverse_ssh/internal/terminal"
)

func TestParseTrustedProxyCIDRs(t *testing.T) {
	line := terminal.ParseLine("--trusted-proxy-cidr 192.0.2.0/24,198.51.100.10 --trusted-proxy-cidr 2001:db8::1", 0)

	got, err := parseTrustedProxyCIDRs(line)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"192.0.2.0/24", "198.51.100.10/32", "2001:db8::1/128"}
	if len(got) != len(want) {
		t.Fatalf("got %d CIDRs, want %d", len(got), len(want))
	}
	gotStrings := make([]string, 0, len(got))
	for _, cidr := range got {
		gotStrings = append(gotStrings, cidr.String())
	}
	sort.Strings(gotStrings)
	sort.Strings(want)
	for i := range want {
		if gotStrings[i] != want[i] {
			t.Fatalf("CIDR %d = %q, want %q", i, gotStrings[i], want[i])
		}
	}
}

func TestParseTrustedProxyCIDRsUnset(t *testing.T) {
	line := terminal.ParseLine(":2222", 0)

	got, err := parseTrustedProxyCIDRs(line)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestParseTrustedProxyCIDRsInvalid(t *testing.T) {
	_, err := parseTrustedProxyCIDRValues([]string{"not-an-ip"})
	if err == nil {
		t.Fatal("expected invalid CIDR error")
	}
	if !strings.Contains(err.Error(), "invalid --trusted-proxy-cidr") {
		t.Fatalf("error = %q", err)
	}
}
