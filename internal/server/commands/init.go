package commands

import (
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

var allCommands = map[string]terminal.Command{
	"ls":       &list{},
	"help":     &help{},
	"exit":     &exit{},
	"connect":  &connect{},
	"kill":     &kill{},
	"rc":       &scripting{},
	"proxy":    &proxy{},
	"rforward": &remoteForward{},
}

func GetCommands(user *internal.User, connection ssh.Channel, requests <-chan *ssh.Request, controllableClients *sync.Map, log logger.Logger) map[string]terminal.Command {

	var o = map[string]terminal.Command{
		"ls":       List(controllableClients),
		"help":     Help(),
		"exit":     Exit(),
		"connect":  Connect(user, controllableClients, nil, log, nil, nil),
		"kill":     Kill(controllableClients, log),
		"rc":       RC(user, controllableClients),
		"proxy":    Proxy(user, controllableClients),
		"rforward": RemoteForward(user, controllableClients, log),
	}

	return o
}
