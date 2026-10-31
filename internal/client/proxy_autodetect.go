package client

import (
	"strings"
	"unicode"
)

type proxyCandidate struct {
	forScheme string
	proxyURL  string
}

type systemProxyConfig struct {
	enabled    bool
	candidates []proxyCandidate
}

type autoProxySelection struct {
	proxyAddr       string
	fallbackProxies []string
	orderedProxies  []string
}

// parseWinINETProxyString parses the value of the WinINET ProxyServer
// registry value. Common formats:
//
//	"host:port"                                  — single proxy for all schemes
//	"http=h:p;https=h:p;ftp=h:p;socks=h:p"       — per-scheme list
//	"http=http://h:p https=socks5://s:p"         — explicit proxy schemes
//
// Entries can be separated by semicolons or whitespace. ftp= entries are
// ignored because the client never makes FTP requests. If a per-scheme entry
// omits an explicit proxy URL scheme, Windows' HTTP/Secure fields are treated
// as HTTP proxy endpoints, while the SOCKS field is treated as SOCKS5.
func parseWinINETProxyString(raw string) []proxyCandidate {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var candidates []proxyCandidate
	for _, part := range splitWinINETProxyList(raw) {
		forScheme, proxySpec := splitWinINETProxyEntry(part)
		proxyURL := normalizeWinINETProxySpec(forScheme, proxySpec)
		if proxyURL == "" {
			continue
		}

		if forScheme == "ftp" {
			continue
		}

		candidates = append(candidates, proxyCandidate{
			forScheme: forScheme,
			proxyURL:  proxyURL,
		})
	}

	return candidates
}

func splitWinINETProxyList(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || unicode.IsSpace(r)
	})
}

func splitWinINETProxyEntry(entry string) (forScheme, proxySpec string) {
	kv := strings.SplitN(strings.TrimSpace(entry), "=", 2)
	if len(kv) != 2 {
		return "", strings.TrimSpace(entry)
	}

	forScheme = strings.ToLower(strings.TrimSpace(kv[0]))
	if forScheme == "secure" {
		forScheme = "https"
	}

	return forScheme, strings.TrimSpace(kv[1])
}

func normalizeWinINETProxySpec(forScheme, proxySpec string) string {
	proxySpec = strings.TrimSpace(proxySpec)
	if proxySpec == "" {
		return ""
	}

	lowerSpec := strings.ToLower(proxySpec)
	if lowerSpec == "direct" || lowerSpec == "direct://" {
		return ""
	}

	if scheme, ok := explicitProxyScheme(proxySpec); ok {
		switch scheme {
		case "http", "https", "socks", "socks5":
			return proxySpec
		default:
			return ""
		}
	}

	if forScheme == "socks" {
		return "socks5://" + proxySpec
	}

	return "http://" + proxySpec
}

func explicitProxyScheme(proxySpec string) (string, bool) {
	index := strings.Index(proxySpec, "://")
	if index <= 0 {
		return "", false
	}

	return strings.ToLower(proxySpec[:index]), true
}

func orderProxyCandidatesForTransport(candidates []proxyCandidate, transport string) []string {
	var preference []string

	switch transport {
	case "http", "ws":
		preference = []string{"http", "", "https", "socks"}
	case "https", "tls", "wss":
		preference = []string{"https", "", "http", "socks"}
	default:
		preference = []string{"", "https", "http", "socks"}
	}

	var ordered []string
	seen := map[string]bool{}

	for _, scheme := range preference {
		for _, candidate := range candidates {
			if candidate.forScheme != scheme || seen[candidate.proxyURL] {
				continue
			}

			ordered = append(ordered, candidate.proxyURL)
			seen[candidate.proxyURL] = true
		}
	}

	return ordered
}

func selectAutoDetectedProxies(config systemProxyConfig, transport, configuredProxy string) autoProxySelection {
	ordered := orderProxyCandidatesForTransport(config.candidates, transport)
	selection := autoProxySelection{
		proxyAddr:      configuredProxy,
		orderedProxies: ordered,
	}

	if config.enabled && configuredProxy == "" && len(ordered) > 0 {
		selection.proxyAddr = ordered[0]
		selection.fallbackProxies = ordered[1:]
		return selection
	}

	selection.fallbackProxies = ordered
	return selection
}

func dedupeProxyFallbacks(primary string, fallbacks []string) []string {
	seen := map[string]bool{}
	if key := proxyDedupeKey(primary); key != "" {
		seen[key] = true
	}

	var deduped []string
	for _, fallback := range fallbacks {
		key := proxyDedupeKey(fallback)
		if key == "" || seen[key] {
			continue
		}

		deduped = append(deduped, fallback)
		seen[key] = true
	}

	return deduped
}

func proxyDedupeKey(proxy string) string {
	normalized, err := GetProxyDetails(proxy)
	if err == nil {
		return normalized
	}

	return strings.TrimSpace(proxy)
}
