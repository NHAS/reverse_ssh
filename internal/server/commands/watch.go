package commands

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/fatih/color"
)

type watch struct {
	datadir string
}

func (w *watch) ValidArgs() map[string]string {
	return map[string]string{
		"a": "Lists all previous connection events",
		"l": "List previous n number of connection events, e.g watch -l 10 shows last 10 connections",
	}
}

func (w *watch) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

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

	if numberOfLinesStr, err := line.GetArgString("l"); err == nil {

		f, err := os.Open(filepath.Join(w.datadir, "watch.log"))
		if err != nil {
			log.Println("unable to open watch.log:", err)
			return err
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return err
		}

		numberOfLines, err := strconv.Atoi(numberOfLinesStr)
		if err != nil {
			return err
		}

		readStartIndex := info.Size()

		i := 0
	outer:
		for {
			readStartIndex -= 128
			if readStartIndex < 0 {
				readStartIndex = 0
			}

			buffer := make([]byte, 128)
			n, err := f.ReadAt(buffer, readStartIndex)
			if err != nil {
				if err == io.EOF {
					break outer
				}
				return err
			}

			for ii := n - 1; ii > 0; ii-- {
				if buffer[ii] == '\n' {
					i++
				}

				// Since we are reading backwards, we have to read towards the previous lines newline
				if i == numberOfLines+1 {
					// Ignore the previous lines new line
					readStartIndex += int64(ii) + 1
					break outer
				}
			}

			if readStartIndex == 0 {
				// If we've regressed to the file start, and not exited the loop jump out now
				break
			}
		}

		_, err = f.Seek(readStartIndex, 0)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(f)
		// optionally, resize scanner's capacity for lines over 64K, see next example
		for scanner.Scan() {
			fmt.Fprintf(tty, "%s\n\r", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		return nil
	}

	messages := make(chan string)

	observerId := observers.ConnectionState.Register(func(c observers.ClientState) {

		var arrowDirection = "<-"
		if c.Status == "disconnected" {
			arrowDirection = "->"
			messages <- fmt.Sprintf("%s %s %s (%s %s) %s %s", c.Timestamp.Format("2006/01/02 15:04:05"), arrowDirection, color.BlueString(c.HostName), c.IP, color.YellowString(c.ID), c.Version, color.RedString(c.Status))
		} else {
			messages <- fmt.Sprintf("%s %s %s (%s %s) %s %s", c.Timestamp.Format("2006/01/02 15:04:05"), arrowDirection, color.BlueString(c.HostName), c.IP, color.YellowString(c.ID), c.Version, color.GreenString(c.Status))
		}

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
		term.DisableRaw(false)
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

	return terminal.MakeHelpText(w.ValidArgs(),
		"watch [OPTIONS]",
		"Watch shows continuous connection status of clients (prints the joining and leaving of clients)",
		"Defaultly waits for new connection events",
	)
}

func Watch(datadir string) *watch {

	return &watch{datadir: datadir}
}
