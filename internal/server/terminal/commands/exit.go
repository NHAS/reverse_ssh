package commands

import "io"

func Exit(args ...string) error {
	return io.EOF
}
