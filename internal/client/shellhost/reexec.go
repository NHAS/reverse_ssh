//go:build !windows
// +build !windows

package shellhost

func ShellHost(exec string, args ...string) {
	//Dummy for linux and other platforms
}
