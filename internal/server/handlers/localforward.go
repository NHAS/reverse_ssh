package handlers

import (
	"io"
	"io/ioutil"
	"net"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func LocalForward(controllableClients *sync.Map) internal.ChannelHandler {

	return func(user *internal.User, newChannel ssh.NewChannel, log logger.Logger) {
		proxyTarget := newChannel.ExtraData()

		var drtMsg internal.ChannelOpenDirectMsg
		err := ssh.Unmarshal(proxyTarget, &drtMsg)
		if err != nil {
			log.Warning("Unable to unmarshal proxy destination: %s", err)
			return
		}

		log.Info("%v", drtMsg.Raddr)

		if c, ok := controllableClients.Load(drtMsg.Raddr); ok {

			target := c.(ssh.Conn)

			go func() {
				connection, requests, err := newChannel.Accept()
				if err != nil {
					newChannel.Reject(ssh.ConnectionFailed, err.Error())
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)

				config := &ssh.ServerConfig{
					PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
						return &ssh.Permissions{}, nil
					},
				}

				privateBytes, err := ioutil.ReadFile("id_ed25519")
				if err != nil {
					log.Fatal("Failed to load private key (%s): %s", "id_25519", err)
				}

				private, err := ssh.ParsePrivateKey(privateBytes)
				if err != nil {
					log.Fatal("Failed to parse private key: %s", err)
				}
				config.AddHostKey(private)

				p1, p2 := net.Pipe()
				go io.Copy(connection, p2)
				go io.Copy(p2, connection)
				conn, chans, reqs, err := ssh.NewServerConn(p1, config)
				if err != nil {
					log.Fatal("%s", err.Error())
				}
				defer conn.Close()

				go func() {
					for r := range reqs {
						ok, ra, _ := target.SendRequest(r.Type, r.WantReply, r.Payload)
						if r.WantReply {
							r.Reply(ok, ra)
						}
					}
				}()

				for c := range chans {
					go func(reqChan ssh.NewChannel) {
						newChan, newReq, err := target.OpenChannel(reqChan.ChannelType(), reqChan.ExtraData())
						if err != nil {
							reqChan.Reject(ssh.ConnectionFailed, err.Error())
							log.Warning("Could not accept channel: %s", err)
							return
						}
						defer newChan.Close()

						targetChannel, targetReqs, err := reqChan.Accept()
						if err != nil {
							reqChan.Reject(ssh.ConnectionFailed, err.Error())
							log.Warning("Could not accept channel: %s", err)
							return
						}
						defer targetChannel.Close()

						go io.Copy(newChan, targetChannel)

						go io.Copy(targetChannel, newChan)

						go func() {
							for r := range newReq {

								ok, err := targetChannel.SendRequest(r.Type, r.WantReply, r.Payload)
								if err != nil {
									r.Reply(false, []byte(err.Error()))
								}
								r.Reply(ok, nil)
							}
						}()

						for r := range targetReqs {

							ok, err := newChan.SendRequest(r.Type, r.WantReply, r.Payload)
							if err != nil {
								r.Reply(false, []byte(err.Error()))
							}

							err = r.Reply(ok, nil)
						}

					}(c)
				}

			}()

			return
		}

		if user.ProxyConnection == nil {
			newChannel.Reject(ssh.Prohibited, "no remote location to forward traffic to")
			return
		}

		proxyDest, proxyRequests, err := user.ProxyConnection.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, err.Error())
			return
		}
		defer proxyDest.Close()

		connection, requests, err := newChannel.Accept()
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, err.Error())
			return
		}
		defer connection.Close()

		go ssh.DiscardRequests(requests)

		log.Info("Human client proxying to: %s:%d", drtMsg.Raddr, drtMsg.Rport)

		go ssh.DiscardRequests(proxyRequests)

		go func() {
			defer proxyDest.Close()
			defer connection.Close()

			io.Copy(connection, proxyDest)
		}()

		io.Copy(proxyDest, connection)

		log.Info("ENDED: %s:%d", drtMsg.Raddr, drtMsg.Rport)

	}
}
