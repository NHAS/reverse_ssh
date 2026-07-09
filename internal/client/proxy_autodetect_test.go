package client

import (
	"reflect"
	"testing"
)

func TestParseWinINETProxyString(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantURLs []string
		wantFor  []string
	}{
		{"empty", "", nil, nil},
		{"whitespace only", "   ", nil, nil},
		{"single host:port", "127.0.0.1:8080", []string{"http://127.0.0.1:8080"}, []string{""}},
		{"single with whitespace", "  proxy.local:3128  ", []string{"http://proxy.local:3128"}, []string{""}},
		{"per-scheme http and https", "http=10.0.0.1:8080;https=10.0.0.1:8443", []string{"http://10.0.0.1:8080", "http://10.0.0.1:8443"}, []string{"http", "https"}},
		{"per-scheme socks", "socks=10.0.0.1:1080", []string{"socks5://10.0.0.1:1080"}, []string{"socks"}},
		{"per-scheme socks fallback", "ftp=10.0.0.1:21;socks=10.0.0.1:1080", []string{"socks5://10.0.0.1:1080"}, []string{"socks"}},
		{"per-scheme only ftp ignored", "ftp=10.0.0.1:21", nil, nil},
		{"per-scheme malformed segment", "http=;https=10.0.0.1:8443", []string{"http://10.0.0.1:8443"}, []string{"https"}},
		{"per-scheme uppercase keys", "HTTP=10.0.0.1:8080", []string{"http://10.0.0.1:8080"}, []string{"http"}},
		{"secure alias", "secure=10.0.0.1:8443", []string{"http://10.0.0.1:8443"}, []string{"https"}},
		{"per-scheme stray semicolons", ";;http=10.0.0.1:8080;;", []string{"http://10.0.0.1:8080"}, []string{"http"}},
		{"explicit http url is preserved", "http=http://proxy.local:8080", []string{"http://proxy.local:8080"}, []string{"http"}},
		{"explicit https proxy url is preserved", "https=https://secure-proxy.local:8443", []string{"https://secure-proxy.local:8443"}, []string{"https"}},
		{"explicit socks5 url is preserved", "socks=socks5://socks.local:1080", []string{"socks5://socks.local:1080"}, []string{"socks"}},
		{"whitespace separated entries", "http=10.0.0.1:8080 https=10.0.0.2:8443", []string{"http://10.0.0.1:8080", "http://10.0.0.2:8443"}, []string{"http", "https"}},
		{"ordered generic fallback list", "proxy1.local:8080;proxy2.local:8080", []string{"http://proxy1.local:8080", "http://proxy2.local:8080"}, []string{"", ""}},
		{"duplicate scheme entries are preserved", "http=proxy1.local:8080;http=proxy2.local:8080", []string{"http://proxy1.local:8080", "http://proxy2.local:8080"}, []string{"http", "http"}},
		{"direct entries ignored", "DIRECT;http=proxy.local:8080", []string{"http://proxy.local:8080"}, []string{"http"}},
		{"unsupported explicit socks4 ignored", "socks=socks4://socks.local:1080;http=proxy.local:8080", []string{"http://proxy.local:8080"}, []string{"http"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWinINETProxyString(tc.in)
			if gotURLs := candidateURLs(got); !reflect.DeepEqual(gotURLs, tc.wantURLs) {
				t.Errorf("urls = %#v, want %#v", gotURLs, tc.wantURLs)
			}
			if gotFor := candidateSchemes(got); !reflect.DeepEqual(gotFor, tc.wantFor) {
				t.Errorf("schemes = %#v, want %#v", gotFor, tc.wantFor)
			}
		})
	}
}

func TestOrderProxyCandidatesForTransport(t *testing.T) {
	candidates := parseWinINETProxyString("generic.local:8080;http=http.local:8080;https=https.local:8443;socks=socks.local:1080")

	cases := []struct {
		name      string
		transport string
		want      []string
	}{
		{
			name:      "http transport prefers http entry",
			transport: "http",
			want:      []string{"http://http.local:8080", "http://generic.local:8080", "http://https.local:8443", "socks5://socks.local:1080"},
		},
		{
			name:      "ws transport prefers http entry",
			transport: "ws",
			want:      []string{"http://http.local:8080", "http://generic.local:8080", "http://https.local:8443", "socks5://socks.local:1080"},
		},
		{
			name:      "https transport prefers secure entry",
			transport: "https",
			want:      []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name:      "wss transport prefers secure entry",
			transport: "wss",
			want:      []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name:      "raw ssh transport prefers generic entry",
			transport: "ssh",
			want:      []string{"http://generic.local:8080", "http://https.local:8443", "http://http.local:8080", "socks5://socks.local:1080"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := orderProxyCandidatesForTransport(candidates, tc.transport)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("orderProxyCandidatesForTransport(..., %q) = %#v, want %#v", tc.transport, got, tc.want)
			}
		})
	}
}

func TestOrderProxyCandidatesDeduplicatesURLs(t *testing.T) {
	candidates := parseWinINETProxyString("http=proxy.local:8080;https=proxy.local:8080;socks=socks.local:1080")
	got := orderProxyCandidatesForTransport(candidates, "https")
	want := []string{"http://proxy.local:8080", "socks5://socks.local:1080"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("orderProxyCandidatesForTransport dedupe = %#v, want %#v", got, want)
	}
}

func TestSelectAutoDetectedProxies(t *testing.T) {
	candidates := parseWinINETProxyString("generic.local:8080;http=http.local:8080;https=https.local:8443;socks=socks.local:1080")

	cases := []struct {
		name           string
		config         systemProxyConfig
		configured     string
		wantProxy      string
		wantFallbacks  []string
		wantOrderedAll []string
	}{
		{
			name: "enabled system proxy becomes primary when no proxy is configured",
			config: systemProxyConfig{
				enabled:    true,
				candidates: candidates,
			},
			wantProxy:      "http://https.local:8443",
			wantFallbacks:  []string{"http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
			wantOrderedAll: []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name: "disabled system proxy remains fallback after direct connection",
			config: systemProxyConfig{
				enabled:    false,
				candidates: candidates,
			},
			wantProxy:      "",
			wantFallbacks:  []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
			wantOrderedAll: []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name: "explicit proxy wins over enabled system proxy",
			config: systemProxyConfig{
				enabled:    true,
				candidates: candidates,
			},
			configured:     "manual.local:8080",
			wantProxy:      "manual.local:8080",
			wantFallbacks:  []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
			wantOrderedAll: []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name: "disabled system proxy remains fallback after explicit proxy",
			config: systemProxyConfig{
				enabled:    false,
				candidates: candidates,
			},
			configured:     "manual.local:8080",
			wantProxy:      "manual.local:8080",
			wantFallbacks:  []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
			wantOrderedAll: []string{"http://https.local:8443", "http://generic.local:8080", "http://http.local:8080", "socks5://socks.local:1080"},
		},
		{
			name: "no candidates keeps direct connection without fallbacks",
			config: systemProxyConfig{
				enabled: true,
			},
			wantProxy: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectAutoDetectedProxies(tc.config, "https", tc.configured)
			if got.proxyAddr != tc.wantProxy {
				t.Errorf("proxyAddr = %q, want %q", got.proxyAddr, tc.wantProxy)
			}
			if !reflect.DeepEqual(got.fallbackProxies, tc.wantFallbacks) {
				t.Errorf("fallbackProxies = %#v, want %#v", got.fallbackProxies, tc.wantFallbacks)
			}
			if !reflect.DeepEqual(got.orderedProxies, tc.wantOrderedAll) {
				t.Errorf("orderedProxies = %#v, want %#v", got.orderedProxies, tc.wantOrderedAll)
			}
		})
	}
}

func TestDedupeProxyFallbacks(t *testing.T) {
	cases := []struct {
		name      string
		primary   string
		fallbacks []string
		want      []string
	}{
		{
			name:      "drops configured proxy duplicate after normalization",
			primary:   "http://proxy.local:8080",
			fallbacks: []string{"proxy.local:8080", "http://other.local:8080"},
			want:      []string{"http://other.local:8080"},
		},
		{
			name:      "drops duplicate default port forms",
			primary:   "http://proxy.local",
			fallbacks: []string{"proxy.local:80", "http://other.local"},
			want:      []string{"http://other.local"},
		},
		{
			name:      "deduplicates fallback list without primary proxy",
			fallbacks: []string{"proxy.local:8080", "http://proxy.local:8080", "socks5://socks.local:1080"},
			want:      []string{"proxy.local:8080", "socks5://socks.local:1080"},
		},
		{
			name:      "keeps invalid proxies once so caller can log parse error",
			fallbacks: []string{"://bad", "://bad", "http://proxy.local:8080"},
			want:      []string{"://bad", "http://proxy.local:8080"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeProxyFallbacks(tc.primary, tc.fallbacks)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeProxyFallbacks(%q, %#v) = %#v, want %#v", tc.primary, tc.fallbacks, got, tc.want)
			}
		})
	}
}

func candidateURLs(candidates []proxyCandidate) []string {
	var urls []string
	for _, candidate := range candidates {
		urls = append(urls, candidate.proxyURL)
	}
	return urls
}

func candidateSchemes(candidates []proxyCandidate) []string {
	var schemes []string
	for _, candidate := range candidates {
		schemes = append(schemes, candidate.forScheme)
	}
	return schemes
}
