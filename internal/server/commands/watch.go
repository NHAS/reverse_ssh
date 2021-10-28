package commands

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server/clients"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type Watch struct {
}

func (w *Watch) Run(tty io.ReadWriter, args ...string) error {

	updatePeriod := 5
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err == nil && n > 0 {
			updatePeriod = n
		}
	}

	for {
		moveto(tty, 0, 0)
		erase_display(tty, 0)
		textcolor_normal(tty)
		textcolor_fg(tty, 7) // Text white

		t, _ := table.NewTable("Targets", "ID", "Hostname", "IP Address")
		for id, conn := range clients.GetAll() {
			t.AddValues(id, conn.User(), conn.RemoteAddr().String())
		}

		out := t.OutputStrings()
		for _, line := range out {
			tty.Write([]byte(line + "\n"))
			//move(tty, 1, 1) // Move down one, once
		}

		time.Sleep(time.Duration(updatePeriod) * time.Second)
	}

	return nil
}

func erase_line(tty io.Writer, n int) {
	fmt.Fprintf(tty, "%c[%dK", 0x1B, n)
}

func erase_display(tty io.Writer, n int) {
	fmt.Fprintf(tty, "%c[%dJ", 0x1B, n)
}

func moveto(tty io.Writer, x int, y int) {

	// clamp the X coordinate.
	if x < 0 {
		x = 0
	}

	// clamp the Y coordinate.
	if y < 0 {
		y = 0
	}

	fmt.Fprintf(tty, "%c[%d;%dH", 0x1B, y, x)
}

func move(tty io.Writer, which int, n int) {

	movement := []byte{'A', 'B', 'C', 'D'}
	if which > len(movement) && which < 0 {
		panic("Direction wasnt in map")
	}

	fmt.Fprintf(tty, "%c[%d%c", 0x1B, n, movement[which])
}

func textcolor_normal(tty io.Writer) {
	fmt.Fprintf(tty, "%c[22m", 0x1B)
}

func textcolor_fg(tty io.Writer, fg int) {

	/* Command is the control command to the terminal */
	fmt.Fprintf(tty, "\033[%dm", fg+30)
}

func (w *Watch) Help(explain bool) string {
	if explain {
		return "Print currently connected clients in a pretty table. (default update every 5 seconds)"
	}

	return makeHelpText(
		"watch [TIME]",
	)
}
