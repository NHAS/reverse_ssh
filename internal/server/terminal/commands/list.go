package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/pkg/table"
	"golang.org/x/crypto/ssh"
)

type list struct {
	controllableClients *sync.Map
}

func (l *list) Run(term *terminal.Terminal, args ...string) error {

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address")

	l.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		sc := value.(ssh.Conn)
		t.AddValues(fmt.Sprintf("%s", idStr), sc.User(), sc.RemoteAddr().String())

		return true
	})

	t.Fprint(term)

	return nil
}

func (l *list) Expect(sections []string) []string {
	return nil
}

func (l *list) Help(explain bool) string {
	if explain {
		return "List connected controllable hosts."
	}

	return makeHelpText(
		"ls",
	)
}

func List(controllableClients *sync.Map) *list {
	return &list{controllableClients: controllableClients}
}
