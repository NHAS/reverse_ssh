package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func proxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {
	a := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(a, &drtMsg)
	if err != nil {
		log.Println(err)
		return
	}

	connection, requests, err := newChannel.Accept()
	defer connection.Close()
	go func() {
		for r := range requests {
			log.Println("Got req: ", r)
		}
	}()

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", drtMsg.Raddr, drtMsg.Rport))
	if err != nil {
		log.Println(err)
		return
	}
	defer tcpConn.Close()

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		io.Copy(connection, tcpConn)
		wg.Done()
	}()
	go func() {
		io.Copy(tcpConn, connection)
		wg.Done()
	}()

	wg.Wait()
}

//This basically handles exactly like a SSH server would
func shellChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}

	var ptyreq internal.PtyReq
PtyListener:
	for req := range requests {
		switch req.Type {
		case "pty-req":
			ptyreq, _ = internal.ParsePtyReq(req.Payload)

			req.Reply(true, nil)
			break PtyListener
		}
	}

	path := ""
	if len(shells) == 0 {
		term := terminal.NewTerminal(connection, "> ")
		fmt.Fprintln(term, "Unable to determine shell to execute")
		for {
			line, err := term.ReadLine()
			if err != nil {
				log.Println("Unable to handle input")
				return
			}

			if stats, err := os.Stat(line); !os.IsExist(err) || stats.IsDir() {
				fmt.Fprintln(term, "Unsuitable selection: ", err)
				continue
			}
			path = line
			break

		}
	} else {
		path = shells[0]
	}

	// Fire up a shell for this session
	shell := exec.Command(path)
	shell.Env = os.Environ()
	shell.Env = append(shell.Env, "TERM="+ptyreq.Term)
	// Prepare teardown function
	close := func() {
		connection.Close() // Not a fan of this
		if shell.Process != nil {
			_, err := shell.Process.Wait()
			if err != nil {
				log.Printf("Failed to exit bash (%s)", err)
			}
		}

		log.Printf("Session closed")
	}

	// Allocate a terminal for this channel
	log.Print("Creating pty...")
	shellf, err := pty.Start(shell)
	if err != nil {
		log.Printf("Could not start pty (%s)", err)
		close()
		return
	}

	//pipe session to bash and visa-versa
	var once sync.Once
	go func() {
		io.Copy(connection, shellf)
		once.Do(close)
	}()
	go func() {
		io.Copy(shellf, connection)
		once.Do(close)
	}()

	internal.SetWinsize(shellf.Fd(), ptyreq.Columns, ptyreq.Rows)

	for req := range requests {
		log.Println("Got request: ", req.Type)
		switch req.Type {
		case "shell":
			// We only accept the default shell
			// (i.e. no command in the Payload)
			if len(req.Payload) == 0 {
				req.Reply(true, nil)
			}

		case "window-change":
			w, h := internal.ParseDims(req.Payload)
			internal.SetWinsize(shellf.Fd(), w, h)
		}
	}

}
