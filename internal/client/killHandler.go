package client

import (
	"os"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func killChannel(user *internal.User, newChannel ssh.NewChannel, l logger.Logger) {
	l.Info("Server sent kill command, exiting...\n")
	<-time.After(3 * time.Second) // 3 seconds is less than the 5 the server will wait
	os.Exit(0)
}
