//go:build !windows

package client

func addHostKerberosHeaders(_ string, req []string) []string {
	// linux doesnt support using kerberos host auth just yet
	return req
}
