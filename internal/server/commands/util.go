package commands

import (
	"log"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/terminal"
)

func makeHelpText(lines ...string) (s string) {
	for _, v := range lines {
		s += v + "\n"
	}

	return s
}

//This lives here due to cyclic import hell
type WindowSizeChangeHandler struct {
	cancel   chan bool
	user     *internal.User
	terminal *terminal.Terminal
}

func NewWindowSizeChangeHandler(u *internal.User, term *terminal.Terminal) *WindowSizeChangeHandler {

	return &WindowSizeChangeHandler{
		cancel:   make(chan bool),
		user:     u,
		terminal: term,
	}
}

func (dh *WindowSizeChangeHandler) Stop() {
	dh.cancel <- true
}

func (dh *WindowSizeChangeHandler) Start() {

	go func() {
		for {
			select {
			case <-dh.cancel:

				return
			case req := <-dh.user.ShellRequests:
				if req == nil { // Channel has closed, so therefore end this default handler
					return
				}

				switch req.Type {

				case "window-change":
					w, h := internal.ParseDims(req.Payload)
					dh.terminal.SetSize(int(w), int(h))

					dh.user.Pty.Columns = w
					dh.user.Pty.Rows = h

				default:
					log.Println("Handled unknown request type in default handler: ", req.Type)
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}

		}
	}()
}
