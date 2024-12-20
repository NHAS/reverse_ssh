//go:build !windows

package client

func additionalHeaders(proxy string, req []string) []string {
	// linux doesnt support using win auth just yet
	return req
}
