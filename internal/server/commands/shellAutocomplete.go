package commands

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

type shellAutocomplete struct {
}

const completion = `
_RSSHCLIENTSCOMPLETION()
{
    local cur=${COMP_WORDS[COMP_CWORD]}
    COMPREPLY=( $(compgen -W "$(ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 autocomplete --clients)" -- $cur) )
}

_RSSHFUNCTIONSCOMPLETIONS()
{
    local cur=${COMP_WORDS[COMP_CWORD]}
    COMPREPLY=( $(compgen -W "$(ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 help -l)" -- $cur) )
}

complete -F _RSSHFUNCTIONSCOMPLETIONS ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 

complete -F _RSSHCLIENTSCOMPLETION ssh -J REPLACEMEWITH_JUMPHOST_THE_REAL_SERVER_NAME_6e020f45-6d31-4c98-af4d-0ba75b48b664

complete -F _RSSHCLIENTSCOMPLETION ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 exec 
complete -F _RSSHCLIENTSCOMPLETION ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 connect 
complete -F _RSSHCLIENTSCOMPLETION ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 listen -c 
complete -F _RSSHCLIENTSCOMPLETION ssh REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458 kill `

func (k *shellAutocomplete) ValidArgs() map[string]string {
	return map[string]string{
		"clients":          "Return a list of client ids",
		"shell-completion": "Generate bash completion to put in .bashrc/.zshrc with optional server name (will use rssh as server name if not set)",
	}
}

func (k *shellAutocomplete) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	if line.IsSet("clients") {
		clients, err := user.SearchClients("")
		if err != nil {
			return nil
		}

		for id, conn := range clients {
			keyId := conn.Permissions.Extensions["pubkey-fp"]
			if conn.Permissions.Extensions["comment"] != "" {
				keyId = conn.Permissions.Extensions["comment"]
			}

			fmt.Fprintf(tty, "%s\n%s\n%s\n%s\n", id, keyId, users.NormaliseHostname(conn.User()), conn.RemoteAddr().String())

		}

		return nil
	}

	if line.IsSet("shell-completion") {
		originalServerName, err := line.GetArgString("shell-completion")
		if err != nil {
			originalServerName = "rssh"
		}

		// We have to preserve the original server name, even if it has the port for the jump host command
		serverConsoleAddress := originalServerName

		host, port, err := net.SplitHostPort(originalServerName)
		if err == nil {
			// the server name had a port in it, so we need to make the console comands into ssh servername -p port command
			// rather than the conventional ssh blah command
			serverConsoleAddress = host + " -p " + port
		}

		res := strings.ReplaceAll(completion, "REPLACEMEWITH_THE_REAL_SERVER_NAME_4259e892-f7ca-4428-afb0-9af135ce9458", serverConsoleAddress)
		res = strings.ReplaceAll(res, "REPLACEMEWITH_JUMPHOST_THE_REAL_SERVER_NAME_6e020f45-6d31-4c98-af4d-0ba75b48b664", originalServerName)

		fmt.Fprintln(tty, res)
		return nil
	}

	return nil
}

func (k *shellAutocomplete) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (k *shellAutocomplete) Help(explain bool) string {
	if explain {
		return "Generate bash/zsh autocompletion, or match clients and return list of ids"
	}

	return terminal.MakeHelpText(k.ValidArgs(),
		"autocomplete",
	)
}
