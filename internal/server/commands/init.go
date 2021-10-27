package commands

import (
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func GetCommands(user *internal.User, connection ssh.Channel, requests <-chan *ssh.Request, controllableClients *sync.Map, log logger.Logger) map[string]terminal.Command {

	o := make(map[string]terminal.Command)

	o["ls"] = List(controllableClients)
	o["help"] = Help()
	o["exit"] = Exit()
	o["connect"] = Connect(user, controllableClients, nil, log, nil, nil)
	o["kill"] = Kill(controllableClients, log)
	o["rc"] = RC(user, controllableClients)
	o["proxy"] = Proxy(user, controllableClients)
	o["rforward"] = RemoteForward(user, controllableClients, log)

	return o
}
