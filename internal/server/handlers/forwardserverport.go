package handlers

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/multiplexer"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

var (
	currentRemoteForwardsLck sync.RWMutex
	currentRemoteForwards    = map[string]string{}
	remoteForwards           = map[string]ssh.Channel{}
)

type chanAddress struct {
	Port uint32
	IP   string
}

func (c *chanAddress) Network() string {
	return "remote_forward_tcp"
}

func (c *chanAddress) String() string {
	return net.JoinHostPort(c.IP, fmt.Sprintf("%d", c.Port))
}

type chanConn struct {
	channel    ssh.Channel
	localAddr  chanAddress
	remoteAddr chanAddress
}

func (c *chanConn) Read(b []byte) (n int, err error) {
	return c.channel.Read(b)
}

func (c *chanConn) Write(b []byte) (n int, err error) {
	return c.channel.Write(b)
}

func (c *chanConn) Close() error {
	return c.channel.Close()
}

func (c *chanConn) LocalAddr() net.Addr {
	return &c.localAddr
}

func (c *chanConn) RemoteAddr() net.Addr {
	return &c.remoteAddr
}

func (c *chanConn) SetDeadline(t time.Time) error {
	return errors.New("not implemented on a channel")
}

func (c *chanConn) SetReadDeadline(t time.Time) error {
	return errors.New("not implemented on a channel")

}

func (c *chanConn) SetWriteDeadline(t time.Time) error {
	return errors.New("not implemented on a channel")

}

func channelToConn(channel ssh.Channel, drtMsg internal.ChannelOpenDirectMsg) net.Conn {

	return &chanConn{
		channel: channel,
		localAddr: chanAddress{
			Port: drtMsg.Lport,
			IP:   drtMsg.Raddr,
		},
		remoteAddr: chanAddress{
			Port: drtMsg.Rport,
			IP:   drtMsg.Raddr,
		},
	}
}

func ServerPortForward(clientId string) func(_ string, _ *users.User, newChannel ssh.NewChannel, log logger.Logger) {
	return func(_ string, _ *users.User, newChannel ssh.NewChannel, log logger.Logger) {
		a := newChannel.ExtraData()

		var drtMsg internal.ChannelOpenDirectMsg
		err := ssh.Unmarshal(a, &drtMsg)
		if err != nil {
			log.Warning("Unable to unmarshal proxy %s", err)
			newChannel.Reject(ssh.ResourceShortage, "Unable to unmarshal proxy")
			return
		}

		connection, requests, err := newChannel.Accept()
		if err != nil {
			newChannel.Reject(ssh.ResourceShortage, "nope")
			log.Warning("Unable to accept new channel %s", err)
			return
		}

		go func() {
			for req := range requests {
				if req.WantReply {
					req.Reply(false, nil)
				}
			}

			StopRemoteForward(clientId)
		}()

		currentRemoteForwardsLck.Lock()
		remoteForwards[clientId] = connection
		currentRemoteForwards[clientId] = net.JoinHostPort(drtMsg.Raddr, fmt.Sprintf("%d", drtMsg.Rport))
		currentRemoteForwardsLck.Unlock()

		multiplexer.ServerMultiplexer.QueueConn(channelToConn(connection, drtMsg))

	}
}

func StopRemoteForward(clientId string) {
	currentRemoteForwardsLck.Lock()
	defer currentRemoteForwardsLck.Unlock()

	if remoteForwards[clientId] != nil {
		remoteForwards[clientId].Close()
	}

	delete(remoteForwards, clientId)
	delete(currentRemoteForwards, clientId)
}
