//go:build windows

package client

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const winINETInternetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// detectSystemProxyConfig reads the WinINET proxy configuration from the
// current user's registry hive (HKCU\...\Internet Settings) and returns proxy
// candidates usable by Connect/GetProxyDetails.
//
// Only the manual ProxyServer value is honoured. ProxyEnable=0 or an absent
// ProxyEnable disables primary proxy selection, but any ProxyServer entries
// are still returned so Run() can try them as fallbacks after a failed direct
// connection. AutoConfigURL (PAC) and AutoDetect (WPAD) are intentionally not
// consulted in this implementation.
//
// An empty candidate list with nil error means "no proxy configured".
func detectSystemProxyConfig() (systemProxyConfig, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, winINETInternetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return systemProxyConfig{}, fmt.Errorf("open HKCU\\%s: %w", winINETInternetSettingsKey, err)
	}
	defer k.Close()

	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil && err != registry.ErrNotExist {
		return systemProxyConfig{}, fmt.Errorf("read ProxyEnable: %w", err)
	}

	server, _, err := k.GetStringValue("ProxyServer")
	if err == registry.ErrNotExist {
		return systemProxyConfig{}, nil
	}
	if err != nil {
		return systemProxyConfig{}, fmt.Errorf("read ProxyServer: %w", err)
	}

	return systemProxyConfig{
		enabled:    enable == 1,
		candidates: parseWinINETProxyString(server),
	}, nil
}

// DetectSystemProxy returns the first usable system proxy for raw SSH-style
// connections. New code should use detectSystemProxyConfig and choose an
// ordering based on the target transport.
func DetectSystemProxy() (string, error) {
	config, err := detectSystemProxyConfig()
	if err != nil {
		return "", err
	}
	if !config.enabled {
		return "", nil
	}

	ordered := orderProxyCandidatesForTransport(config.candidates, "ssh")
	if len(ordered) == 0 {
		return "", nil
	}

	return ordered[0], nil
}
