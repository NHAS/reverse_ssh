//go:build windows
// +build windows

package shellhost

import (
	"log"
	"os"
	"strconv"
)

func ShellHost(exe string, args ...string) {
	if len(args) != 2 {
		return
	}

	width, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return
	}

	height, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return
	}

	err = run(exe, int(width), int(height))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
