package handlers

import (
	"github.com/NHAS/reverse_ssh/internal/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type ChannelHandler func(user *users.User, newChannel ssh.NewChannel, log logger.Logger)
