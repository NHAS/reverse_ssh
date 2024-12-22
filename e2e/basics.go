package main

import (
	"os"
)

func basics(serverLog *os.File) {
	conditionExec("version", Version, 0, "", 0)
	conditionExec("ls", "No RSSH clients connected", 1, "", 0)
	conditionExec("who", user, 0, "", 0)
}
