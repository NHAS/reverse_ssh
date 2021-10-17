package commands

import (
	"fmt"
	"io"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/commands/constants"
	"github.com/NHAS/reverse_ssh/pkg/table"
	"golang.org/x/crypto/ssh"
)

type list struct {
	controllableClients *sync.Map
}

func (l *list) Run(tty io.ReadWriter, args ...string) error {

	if len(args) == 1 {
		t, _ := table.NewTable("Target", "Hostname", "IP Address", "Sys Info")

		v, ok := l.controllableClients.Load(args[0])
		if !ok {
			return fmt.Errorf("unknown client host")
		}

		client := v.(ssh.Conn)

		_, sysInfo, err := client.SendRequest("info", true, nil)
		//This will happen on connection failure, rather than error gathering system information
		if err != nil {
			sysInfo = []byte(err.Error())
		}

		t.AddValues(client.User(), client.RemoteAddr().String(), string(sysInfo))

		t.Fprint(tty)
		return nil
	}

	t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address")

	l.controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		sc := value.(ssh.Conn)

		t.AddValues(fmt.Sprintf("%s", idStr), sc.User(), sc.RemoteAddr().String())

		return true
	})

	t.Fprint(tty)

	return nil
}

func (l *list) Expect(sections []string) []string {
	if len(sections) == 1 {
		return []string{constants.RemoteId}
	}

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
