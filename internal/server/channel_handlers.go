package server

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func proxyChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	if connections[sshConn] == nil {
		newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
		return
	}

	destConn := connections[sshConn]

	proxyTarget := newChannel.ExtraData()

	var drtMsg internal.ChannelOpenDirectMsg
	err := ssh.Unmarshal(proxyTarget, &drtMsg)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Human client proxying to: %s:%d\n", drtMsg.Raddr, drtMsg.Rport)

	connection, requests, err := newChannel.Accept()
	defer connection.Close()
	go func() {
		for r := range requests {
			log.Println("Got req: ", r)
		}
	}()

	proxyDest, proxyRequests, err := destConn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	defer proxyDest.Close()
	go func() {
		for r := range proxyRequests {
			log.Println("Prox Got req: ", r)
		}
	}()

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		io.Copy(connection, proxyDest)
		wg.Done()
	}()
	go func() {
		io.Copy(proxyDest, connection)
		wg.Done()
	}()

	wg.Wait()
}

func sessionChannel(sshConn ssh.Conn, newChannel ssh.NewChannel) {

	defer log.Printf("Human client disconnected %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion())

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()

	term := terminal.NewTerminal(connection, "> ")

	term.AutoCompleteCallback = func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {

		if key == '\t' {

			parts := strings.Split(line, " ")

			searchString := ""
			if len(parts) > 0 {
				searchString = parts[len(parts)-1]
			}

			var r []string
			if len(parts) == 1 {
				r = autoCompleteCommands.PrefixMatch(searchString)
			}
			if len(parts) > 1 {
				r = autoCompleteClients.PrefixMatch(searchString)
			}

			if len(r) == 1 {
				return line + r[0], len(line + r[0]), true
			}

			for _, completion := range r {
				fmt.Fprintf(term, "%s\n", line+completion)
			}

			return "", 0, false
		}
		return "", 0, false
	}

	stop := make(chan bool)

	var ptyReq ssh.Request
	var lastWindowChange ssh.Request

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	// While we arent passing the requests directly to the remote host consume them with our terminal and store the results to send initialy to the remote on client connect
	go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop)

	//Send list of controllable remote hosts to human client
	fmt.Fprintf(term, "Connected controllable clients: \n")
	controllableClients.Range(func(idStr interface{}, value interface{}) bool {
		fmt.Fprintf(term, "%s, client version: %s\n",
			idStr,
			value.(ssh.Conn).ClientVersion(),
		)
		return true
	})

	for {
		//This will break if the user does CTRL+C or CTRL+D apparently we need to reset the whole terminal if a user does this....
		line, err := term.ReadLine()
		if err != nil {
			log.Println("Breaking")
			break
		}

		commandParts := strings.Split(line, " ")

		if len(commandParts) > 0 {

			switch commandParts[0] {
			default:
				fmt.Fprintf(term, "Unknown command: %s\n", commandParts[0])

			case "ls":
				controllableClients.Range(func(idStr interface{}, value interface{}) bool {
					fmt.Fprintf(term, "%s, client version: %s\n",
						idStr,
						value.(ssh.Conn).ClientVersion(),
					)
					return true
				})

			case "help":
				r := autoCompleteCommands.PrefixMatch("")
				fmt.Fprintln(term, "Commands: ")
				for _, completion := range r {
					fmt.Fprintf(term, "%s\n", completion)
				}

			case "exit":
				return
			case "connect":
				if len(commandParts) != 2 {
					fmt.Fprintf(term, "connect <remote machine id>\n")
					continue
				}

				c, ok := controllableClients.Load(commandParts[1])
				if !ok {
					fmt.Fprintf(term, "Unknown connection host\n")
					continue
				}

				controlClient := c.(ssh.Conn)
				//Attempt to connect to remote host and send inital pty request and screen size
				// If we cant, report and error to the clients terminal
				newSession, err := createSession(controlClient, ptyReq, lastWindowChange)
				if err == nil {
					stop <- true // Stop the default request handler

					connections[sshConn] = controlClient

					err := attachSession(term, newSession, connection, requests)
					if err != nil {
						fmt.Fprintf(term, "Error: %s", err)
						log.Println(err)
					}

					connections[sshConn] = nil

					fmt.Fprintf(term, "Session has terminated. Press any key to continue\n")
					log.Printf("Client %s (%s) has disconnected from remote host %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), controlClient.RemoteAddr(), controlClient.ClientVersion())

					go handleSSHRequests(&ptyReq, &lastWindowChange, term, requests, stop) // Re-enable the default handler if the client isnt connected to a remote host
				} else {

					fmt.Fprintf(term, "%s\n", err)
				}
			}
		}
	}

	delete(connections, sshConn)
}
