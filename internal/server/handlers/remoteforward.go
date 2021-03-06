package handlers

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

type remoteForward struct {
	BindAddr string
	BindPort uint32
}

//This allows for a client to forward a port to the server, e.g -R 9999
//Which would open a port on 9999 on the server localhost, that will pipe traffic to the clients network
func RemoteForward(sshConn ssh.Conn, reqs <-chan *ssh.Request) {
	defer sshConn.Close()
	clientClosed := make(chan bool)
	for r := range reqs {

		switch r.Type {

		case "tcpip-forward":

			go func() {
				var rf remoteForward

				err := ssh.Unmarshal(r.Payload, &rf)
				if err != nil {
					log.Println(err)
					r.Reply(false, []byte("Unable to open remote forward"))
					return
				}

				l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", rf.BindPort))
				if err != nil {
					log.Println(err)
					r.Reply(false, []byte("Unable to open remote forward"))
					return
				}

				go func() {
					<-clientClosed
					l.Close()
				}()
				defer l.Close()

				r.Reply(true, nil)

				for {

					proxyCon, err := l.Accept()
					if err != nil {
						if !strings.Contains(err.Error(), "use of a closed") {
							log.Println(err)
						}
						return
					}
					go handleData(rf, proxyCon, sshConn)
				}

			}()
		default:
			log.Printf("Client %s sent unknown proxy request type: %s\n", sshConn.RemoteAddr(), r.Type)
			if err := r.Reply(false, nil); err != nil {
				log.Println("Sending reply encountered an error: ", err)
				sshConn.Close()
			}
		}
	}

	clientClosed <- true

	log.Printf("Proxy client %s ended\n", sshConn.RemoteAddr())

}

func handleData(rf remoteForward, proxyCon net.Conn, sshConn ssh.Conn) error {

	originatorAddress := proxyCon.LocalAddr().String()
	var originatorPort uint32

	for i := len(originatorAddress) - 1; i > 0; i-- {
		if originatorAddress[i] == ':' {

			e, err := strconv.Atoi(originatorAddress[i+1:])
			if err != nil {
				log.Fatal(err)
			}

			originatorPort = uint32(e)
			originatorAddress = originatorAddress[:i]
			break
		}
	}

	drtMsg := internal.ChannelOpenDirectMsg{
		Raddr: rf.BindAddr,
		Rport: rf.BindPort,

		Laddr: originatorAddress,
		Lport: originatorPort,
	}

	b := ssh.Marshal(&drtMsg)

	destination, reqs, err := sshConn.OpenChannel("forwarded-tcpip", b)
	if err != nil {
		return err
	}

	go ssh.DiscardRequests(reqs)

	go func() {
		defer destination.Close()
		defer proxyCon.Close()

		io.Copy(destination, proxyCon)
	}()
	go func() {
		defer destination.Close()
		defer proxyCon.Close()

		io.Copy(proxyCon, destination)

	}()

	return nil
}
