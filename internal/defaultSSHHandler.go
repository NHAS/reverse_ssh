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

				log.Println("Got request: ", req.Type)
				switch req.Type {
				case "shell":
					// We only accept the default shell
					// (i.e. no command in the Payload)
					req.Reply(len(req.Payload) == 0, nil)
				case "pty-req":

					//Ignoring the error here as we are not fully parsing the payload, leaving the unmarshal func a bit confused (thus returning an error)
					ptyReqData, _ := ParsePtyReq(req.Payload)
					dh.terminal.SetSize(int(ptyReqData.Columns), int(ptyReqData.Rows))

					dh.user.PtyReq = *req

					req.Reply(true, nil)
				case "window-change":
					w, h := ParseDims(req.Payload)
					dh.terminal.SetSize(int(w), int(h))

					dh.user.LastWindowChange = *req
				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}

		}
	}()
}
