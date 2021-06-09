package internal

import (
	"log"

	"github.com/NHAS/reverse_ssh/internal/server/terminal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
)

type DefaultSSHHandler struct {
	cancel   chan bool
	user     *users.User
	terminal *terminal.Terminal
}

func NewDefaultHandler(u *users.User, term *terminal.Terminal) *DefaultSSHHandler {

	return &DefaultSSHHandler{
		cancel:   make(chan bool),
		user:     u,
		terminal: term,
	}
}

func (dh *DefaultSSHHandler) Stop() {
	dh.cancel <- true
}

func (dh *DefaultSSHHandler) Start() {

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
					w, h := ParseDims(req.Payload)
					dh.terminal.SetSize(int(w), int(h))

					dh.user.LastWindowChange = *req
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
