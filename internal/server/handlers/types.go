package handlers

import (
	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type ChannelHandler func(user *internal.User, newChannel ssh.NewChannel, log logger.Logger)
