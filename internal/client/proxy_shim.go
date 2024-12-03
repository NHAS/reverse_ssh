//go:build !windows

package client

func AdditionalHeaders(proxy string, req []string) []string {
	// linux doesnt support using win auth just yet
	return req
}
