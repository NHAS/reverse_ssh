//go:build windows
// +build windows

package shellhost

import (
	"log"
	"os"
)

func ShellHost(exe string) {
	err := run(exe)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
