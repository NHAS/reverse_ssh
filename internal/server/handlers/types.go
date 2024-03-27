package handlers

import (
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type ChannelHandler func(connectionDetails string, user *users.User, newChannel ssh.NewChannel, log logger.Logger)
