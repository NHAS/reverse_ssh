package commands

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

type watch struct {
}

func (w *watch) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	messages := make(chan string)

	var joinId string
	if !line.IsSet("l") {
		joinId = observers.Join.Register(func(m []string) {

			messages <- fmt.Sprintf("-> %s joined", strings.Join(m, " "))
		})
	}

	var leaveId string
	if !line.IsSet("j") {
		leaveId = observers.Leave.Register(func(m []string) {
			messages <- fmt.Sprintf("<- %s left", strings.Join(m, " "))
		})
	}

	term, isTerm := tty.(*terminal.Terminal)
	if isTerm {
		term.EnableRaw()
	}

	go func() {

		b := make([]byte, 1)
		tty.Read(b)

		observers.Leave.Deregister(joinId)
		observers.Join.Deregister(leaveId)

		close(messages)
	}()

	fmt.Fprintf(tty, "Watching clients...\n\r")
	for m := range messages {
		fmt.Fprintf(tty, "%s %s\n\r", time.Now().Format("2006/01/02 15:04:05"), m)
	}

	if isTerm {
		term.DisableRaw()
	}

	return nil
}

func (W *watch) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (w *watch) Help(explain bool) string {
	if explain {
		return "Watches controllable client connections"
	}

	return terminal.MakeHelpText(
		"watch [OPTIONS]",
		"Watch shows joining and leaving of clients",
		"\t-j\tPrint joins only",
		"\t-l\tPrint leaves only",
	)
}

func Watch(log logger.Logger) *watch {
	return &watch{}
}
