package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

type proxy struct {
	user                *users.User
	controllableClients *sync.Map

	modeAutoComplete *trie.Trie
}

func (p *proxy) Run(term *terminal.Terminal, args ...string) error {

	if len(args) < 1 {
		helpText := "proxy disconnect\n"
		helpText += "proxy connect <remote_id>"
		return fmt.Errorf(helpText)
	}

	switch args[0] {
	case "disconnect":
		p.user.ProxyConnection = nil
	case "connect":
		if len(args) != 2 {
			return fmt.Errorf("Not enough arguments to connect to a proxy host.\nproxy connect <remote_id>")
		}

		cc, ok := p.controllableClients.Load(args[1])
		if !ok {
			return fmt.Errorf("Unknown connection host")
		}

		controlClient := cc.(ssh.Conn)

		p.user.ProxyConnection = controlClient
	default:
		return fmt.Errorf("Invalid subcommand %s", args[0])
	}

	return nil
}

func (p *proxy) Expect(sections []string) []string {
	if len(sections) == 1 {
		return p.modeAutoComplete.PrefixMatch(sections[0])
	}

	switch sections[0] {
	case "connect":
		return []string{RemoteId}
	default:
		return nil
	}
}

func Proxy(user *users.User, controllableClients *sync.Map) *proxy {
	return &proxy{
		user:                user,
		controllableClients: controllableClients,
		modeAutoComplete:    trie.NewTrie("disconnect", "connect"),
	}
}
