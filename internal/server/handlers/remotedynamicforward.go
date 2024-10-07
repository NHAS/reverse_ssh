package handlers

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

// A different kind of handler, this is allowed to open ports on the actual server itself
// This is so when on a machine with strict av that has a regular ssh client you can do `ssh -R port rssh.server` and get a proxiable port into the network
func RemoteDynamicForward(sshConn ssh.Conn, reqs <-chan *ssh.Request, log logger.Logger) {
	defer sshConn.Close()
	clientClosed := make(chan bool)
	for r := range reqs {

		switch r.Type {

		case "tcpip-forward":

			go func(req *ssh.Request) {
				var rf internal.RemoteForwardRequest

				err := ssh.Unmarshal(req.Payload, &rf)
				if err != nil {
					log.Warning("failed to unmarshal remote forward request: %s", err)
					req.Reply(false, []byte("Unable to open remote forward"))
					return
				}

				// Ignore rf.BindAddr, helps us mitigate malicious clients
				l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", rf.BindPort))
				if err != nil {
					log.Warning("failed to listen for remote forward request: %s", err)
					req.Reply(false, []byte("Unable to open remote forward"))
					return
				}

				log.Info("Opened remote forward port on server: 127.0.0.1:%d", rf.BindPort)

				go func() {
					<-clientClosed
					l.Close()
				}()
				defer l.Close()

				req.Reply(true, nil)

				for {

					proxyCon, err := l.Accept()
					if err != nil {
						if !strings.Contains(err.Error(), "use of a closed") {
							log.Warning("failed to accept tcp connection: %s", err)
						}
						return
					}
					go handleData(rf, proxyCon, sshConn)
				}

			}(r)
		default:

			log.Info("Client %s sent unknown proxy request type: %s", sshConn.RemoteAddr(), r.Type)

			if err := r.Reply(false, nil); err != nil {
				log.Info("Sending reply encountered an error: %s", err)
				sshConn.Close()
			}
		}
	}

	clientClosed <- true

	log.Info("Proxy client %s ended", sshConn.RemoteAddr())

}

func handleData(rf internal.RemoteForwardRequest, proxyCon net.Conn, sshConn ssh.Conn) error {

	originatorAddress := proxyCon.LocalAddr().String()
	var originatorPort uint32

	for i := len(originatorAddress) - 1; i > 0; i-- {
		if originatorAddress[i] == ':' {

			e, err := strconv.ParseInt(originatorAddress[i+1:], 10, 32)
			if err != nil {
				sshConn.Close()
				return fmt.Errorf("failed to parse port number: %s", err)
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
