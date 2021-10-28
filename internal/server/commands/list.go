package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/pkg/table"
	"golang.org/x/crypto/ssh"
)

type list struct {
	controllableClients *sync.Map
}

func (l *list) Run(tty io.ReadWriter, args ...string) error {

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address")

	l.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		sc := value.(ssh.Conn)

		t.AddValues(fmt.Sprintf("%s", idStr), sc.User(), sc.RemoteAddr().String())

		return true
	})

	t.Fprint(tty)

	return nil
}
func (l *list) Help(explain bool) string {
	if explain {
		return "List connected controllable hosts."
	}

	return makeHelpText(
		"ls",
		"ls <remote_id>",
	)
}

func List(controllableClients *sync.Map) *list {
	return &list{controllableClients}
}
