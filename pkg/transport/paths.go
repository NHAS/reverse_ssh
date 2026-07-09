package transport

import "strings"

const (
	DefaultWSPath   = "/ws"
	DefaultPushPath = "/push"
)

func NormalizePath(path, fallback string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = fallback
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func JoinPushPath(pushPath, suffix string) string {
	pushPath = NormalizePath(pushPath, DefaultPushPath)
	suffix = strings.TrimLeft(suffix, "/")
	if suffix == "" {
		return pushPath
	}
	return pushPath + "/" + suffix
}
