package handlers

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/client/connection"
	"golang.org/x/crypto/ssh"
)

type remoteforward struct {
	Listener net.Listener
	User     *connection.Session
}

var (
	currentRemoteForwardsLck sync.RWMutex
	currentRemoteForwards    = map[internal.RemoteForwardRequest]remoteforward{}
)

func GetServerRemoteForwards() (out []string) {
	currentRemoteForwardsLck.RLock()
	defer currentRemoteForwardsLck.RUnlock()

	for a, c := range currentRemoteForwards {
		if c.User == nil {
			out = append(out, a.String())
		}
	}

	return out
}

func StopRemoteForward(rf internal.RemoteForwardRequest) error {
	currentRemoteForwardsLck.Lock()
	defer currentRemoteForwardsLck.Unlock()

	if _, ok := currentRemoteForwards[rf]; !ok {
		return fmt.Errorf("unable to find remote forward request")
	}

	currentRemoteForwards[rf].Listener.Close()
	delete(currentRemoteForwards, rf)

	log.Println("Stopped listening on: ", rf.BindAddr, rf.BindPort)

	return nil
}

func StartRemoteForward(session *connection.Session, r *ssh.Request, sshConn ssh.Conn) {

	var rf internal.RemoteForwardRequest
	err := ssh.Unmarshal(r.Payload, &rf)
	if err != nil {
		r.Reply(false, []byte(fmt.Sprintf("Unable to open remote forward: %s", err.Error())))
		return
	}
	l, err := net.Listen("tcp", net.JoinHostPort(rf.BindAddr, fmt.Sprintf("%d", rf.BindPort)))
	if err != nil {
		r.Reply(false, []byte(fmt.Sprintf("Unable to open remote forward: %s", err.Error())))
		return
	}
	defer l.Close()

	defer StopRemoteForward(rf)

	if session != nil {
		session.Lock()
		session.SupportedRemoteForwards[rf] = true
		session.Unlock()
	}

	//https://datatracker.ietf.org/doc/html/rfc4254
	responseData := []byte{}
	if rf.BindPort == 0 {
		port := uint32(l.Addr().(*net.TCPAddr).Port)
		responseData = ssh.Marshal(port)
		rf.BindPort = port
	}
	r.Reply(true, responseData)

	log.Println("Started listening on: ", l.Addr())

	currentRemoteForwardsLck.Lock()

	currentRemoteForwards[rf] = remoteforward{
		Listener: l,
		User:     session,
	}
	currentRemoteForwardsLck.Unlock()

	for {

		proxyCon, err := l.Accept()
		if err != nil {
			return
		}
		go handleData(rf, proxyCon, sshConn)
	}

}

func handleData(rf internal.RemoteForwardRequest, proxyCon net.Conn, sshConn ssh.Conn) error {

	log.Println("Accepted new connection: ", proxyCon.RemoteAddr())

	originatorAddress, originatorPort, err := net.SplitHostPort(proxyCon.LocalAddr().String())
	if err != nil {
		return err
	}

	originatorPortInt, err := strconv.ParseInt(originatorPort, 10, 32)
	if err != nil {
		return err
	}

	drtMsg := internal.ChannelOpenDirectMsg{

		Raddr: originatorAddress,
		Rport: uint32(originatorPortInt),

		Laddr: rf.BindAddr,
		Lport: rf.BindPort,
	}

	log.Printf("formed drtMsg: %+v", drtMsg)

	b := ssh.Marshal(&drtMsg)

	destination, reqs, err := sshConn.OpenChannel("forwarded-tcpip", b)
	if err != nil {
		log.Println("Opening forwarded-tcpip channel to server failed: ", err)

		return err
	}
	defer destination.Close()

	go ssh.DiscardRequests(reqs)

	log.Println("Forwarded-tcpip channel request sent and accepted")

	go func() {
		defer destination.Close()
		defer proxyCon.Close()
		io.Copy(destination, proxyCon)

	}()

	defer proxyCon.Close()
	_, err = io.Copy(proxyCon, destination)

	return err
}
