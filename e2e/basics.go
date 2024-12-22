package main

import "log"

func basics() {
	log.Println("Starting basic tests")
	conditionExec("version", Version, 0, "", 0)
	conditionExec("ls", "No RSSH clients connected", 1, "", 0)
	conditionExec("who", user, 0, "", 0)
	log.Println("Finished basic tests")
}
