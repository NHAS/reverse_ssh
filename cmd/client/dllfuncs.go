//go:build windows && cgo && cshared

package main

import "C"

//export VoidFunc
func VoidFunc() {
	settings, _ := makeInitialSettings()

	Run(settings)
}

//export OnProcessAttach
func OnProcessAttach() {
	settings, _ := makeInitialSettings()
	Run(settings)
}
