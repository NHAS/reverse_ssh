package commands

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"golang.org/x/crypto/ssh"
)

func List(controllableClients *sync.Map, term *terminal.Terminal) terminal.TerminalFunctionCallback {
	return func(args ...string) error {
		controllableClients.Range(func(idStr interface{}, value interface{}) bool {
			fmt.Fprintf(term, "%s, client version: %s\n",
				idStr,
				value.(ssh.Conn).ClientVersion(),
			)
			return true
		})

		return nil
	}
}
