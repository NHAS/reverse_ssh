package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

type proxy struct {
	user                *internal.User
	controllableClients *sync.Map
	currentlyConnected  string
	modeAutoComplete    *trie.Trie
}

func (p *proxy) Run(tty io.ReadWriter, args ...string) error {

	if len(args) < 1 {
		return fmt.Errorf(p.Help(false))
	}

	switch args[0] {
	case "status":
		if p.currentlyConnected == "" {
			return fmt.Errorf("Disconnected")
		}

		fmt.Fprintf(tty, "Connected to %s\n", p.currentlyConnected)

	case "disconnect":
		fmt.Fprintf(tty, "Disconnected from %s\n", p.currentlyConnected)

		p.user.ProxyConnection = nil
		p.currentlyConnected = ""

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
		p.currentlyConnected = args[1]

		fmt.Fprintf(tty, "Connected: %s\n", p.currentlyConnected)
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
		return []string{constants.RemoteId}
	default:
		return nil
	}
}

func (p *proxy) Help(explain bool) string {
	if explain {
		return "Set or stop proxy connection to controlled remote host."
	}

	return makeHelpText(
		"proxy disconnect",
		"proxy status",
		"proxy connect <remote_id>",
	)
}

func Proxy(user *internal.User, controllableClients *sync.Map) *proxy {
	return &proxy{
		user:                user,
		controllableClients: controllableClients,
		modeAutoComplete:    trie.NewTrie("disconnect", "connect", "status"),
	}
}