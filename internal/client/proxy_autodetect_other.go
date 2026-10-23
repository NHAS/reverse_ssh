//go:build !windows

package client

// DetectSystemProxy is a no-op on non-Windows platforms. WinINET-style proxy
// auto-detection is not available; users on other platforms should rely on
// the standard *_PROXY environment variables, which Run() already consults.
func DetectSystemProxy() (string, error) {
	return "", nil
}

func detectSystemProxyConfig() (systemProxyConfig, error) {
	return systemProxyConfig{}, nil
}
