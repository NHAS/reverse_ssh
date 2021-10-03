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
	clientSysinfo       map[string]string
}

func (l *list) Run(term *terminal.Terminal, args ...string) error {

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address", "Sys Info")

	l.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		sc := value.(ssh.Conn)
		sysInfo, infoSet := l.clientSysinfo[idStr.(string)]
		if !infoSet {
			sysInfo = "Unknown"
		}
		t.AddValues(fmt.Sprintf("%s", idStr), sc.User(), sc.RemoteAddr().String(), sysInfo)

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

func List(controllableClients *sync.Map, clientSysinfo map[string]string) *list {
	return &list{controllableClients, clientSysinfo}
}
