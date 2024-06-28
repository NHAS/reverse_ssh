package storage

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func Store(filename string, r io.ReadCloser) (string, error) {

	fd, err := unix.MemfdCreate("", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return StoreDisk(filename, r)
	}

	mfd := os.NewFile(uintptr(fd), "")
	_, err = io.Copy(mfd, r)
	if err != nil {
		return StoreDisk(filename, r)
	}

	return fmt.Sprintf("/proc/self/fd/%d", fd), nil
}
