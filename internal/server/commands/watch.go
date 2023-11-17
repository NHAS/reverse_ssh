package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/observer"
)

type watch struct {
	datadir string
}

func (w *watch) Run(tty io.ReadWriter, line terminal.ParsedLine) error {

	if line.IsSet("h") || line.IsSet("help") {
		return errors.New(w.Help(false))
	}

	if line.IsSet("a") {

		f, err := os.Open(filepath.Join(w.datadir, "watch.log"))
		if err != nil {
			log.Println("unable to open watch.log:", err)
			return err
		}

		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fmt.Fprintf(tty, "%s\n\r", sc.Text())
		}

		return sc.Err()
	}

	messages := make(chan string)

	observerId := observers.ConnectionState.Register(func(m observer.Message) {

		c := m.(observers.ClientState)

		var arrowDirection = "<-"
		if c.Status == "disconnected" {
			arrowDirection = "->"
		}

		messages <- fmt.Sprintf("%s %s %s (%s %s) %s %s", c.Timestamp.Format("2006/01/02 15:04:05"), arrowDirection, c.HostName, c.IP, c.ID, c.Version, c.Status)

	})

	term, isTerm := tty.(*terminal.Terminal)
	if isTerm {
		term.EnableRaw()
	}

	go func() {

		b := make([]byte, 1)
		tty.Read(b)

		observers.ConnectionState.Deregister(observerId)

		close(messages)
	}()

	fmt.Fprintf(tty, "Watching clients...\n\r")
	for m := range messages {
		fmt.Fprintf(tty, "%s\n\r", m)
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
		"Watch shows continuous connection status of clients (prints the joining and leaving of clients)",
		"Defaultly waits for new connection events",
		"\t-l\tList previous n number of connection events, e.g watch -l 10 shows last 10 connections",
	)
}

func Watch(datadir string) *watch {
	observers.ConnectionState.Register(func(m observer.Message) {

		c := m.(observers.ClientState)

		var arrowDirection = "<-"
		if c.Status == "disconnected" {
			arrowDirection = "->"
		}

		f, err := os.OpenFile(filepath.Join(datadir, "watch.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Println("unable to open watch log for writing:", err)
		}
		defer f.Close()

		if _, err := f.WriteString(fmt.Sprintf("%s %s %s (%s %s) %s %s\n", c.Timestamp.Format("2006/01/02 15:04:05"), arrowDirection, c.HostName, c.IP, c.ID, c.Version, c.Status)); err != nil {
			log.Println(err)
		}

	})

	return &watch{datadir: datadir}
}
