//go:build !windows

package client

func additionalHeaders(_ string, req []string) []string {
	// linux doesnt support using win auth just yet
	return req
}
