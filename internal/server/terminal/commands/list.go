package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"golang.org/x/crypto/ssh"
)

type list struct {
	controllableClients *sync.Map
}

func (l *list) Run(term *terminal.Terminal, args ...string) error {
	l.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		fmt.Fprintf(term, "%s, client version: %s\n",
			idStr,
			value.(ssh.Conn).ClientVersion(),
		)
		return true
	})

	return nil
}

func (l *list) Expect(sections []string) []string {
	return nil
}

func List(controllableClients *sync.Map) *list {
	return &list{controllableClients: controllableClients}
}
