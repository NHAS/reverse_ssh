package main

import (
	"os"

	"golang.org/x/crypto/ssh"
)

func basics(session *ssh.Session, serverLog *os.File) {
	condition(session, "version", Version, nil, "", 0)
	condition(session, "ls", "No RSSH clients connected", nil, "", 0)
	condition(session, "who", user, nil, "", 0)
}
