package handlers

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

var CurrentRemoteForwards = map[string]net.Listener{}

func StartRemoteForward(r *ssh.Request, sshConn ssh.Conn) {

	var rf internal.RemoteForwardRequest

	err := ssh.Unmarshal(r.Payload, &rf)
	if err != nil {
		log.Println(err)
		r.Reply(false, []byte(fmt.Sprintf("Unable to open remote forward: %s", err.Error())))
		return
	}

	address := fmt.Sprintf("%s:%d", rf.BindAddr, rf.BindPort)

	if _, ok := CurrentRemoteForwards[address]; ok && rf.BindPort != 0 {
		r.Reply(true, nil)
	}

	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Println(err)
		r.Reply(false, []byte(fmt.Sprintf("Unable to open remote forward: %s", err.Error())))
		return
	}
	defer l.Close()

	//https://datatracker.ietf.org/doc/html/rfc4254
	responseData := []byte{}
	if rf.BindPort == 0 {
		port := uint32(l.Addr().(*net.TCPAddr).Port)
		responseData = ssh.Marshal(port)
		rf.BindPort = port
	}
	r.Reply(true, responseData)

	address = fmt.Sprintf("%s:%d", rf.BindAddr, rf.BindPort)

	log.Println("Started listening on: ", address)

	CurrentRemoteForwards[address] = l

	for {

		proxyCon, err := l.Accept()
		if err != nil {
			return
		}
		go handleData(rf, proxyCon, sshConn)
	}

}

func handleData(rf internal.RemoteForwardRequest, proxyCon net.Conn, sshConn ssh.Conn) error {
	defer proxyCon.Close()

	log.Println("Accepted new connection: ", proxyCon.RemoteAddr())

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

	_, err = io.Copy(proxyCon, destination)
	return err
}
