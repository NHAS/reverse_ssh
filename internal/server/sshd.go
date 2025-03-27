package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/handlers"
	"github.com/NHAS/reverse_ssh/internal/server/observers"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

type Options struct {
	AllowList []*net.IPNet
	DenyList  []*net.IPNet
	Comment   string

	Owners []string
}

func readPubKeys(path string) (m map[string]Options, err error) {
	authorizedKeysBytes, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("failed to load file %s, err: %v", path, err)
	}

	keys := bytes.Split(authorizedKeysBytes, []byte("\n"))
	m = map[string]Options{}

	for i, key := range keys {
		key = bytes.TrimSpace(key)
		if len(key) == 0 {
			continue
		}

		pubKey, comment, options, _, err := ssh.ParseAuthorizedKey(key)
		if err != nil {
			return m, fmt.Errorf("unable to parse public key. %s line %d. Reason: %s", path, i+1, err)
		}

		var opts Options
		opts.Comment = comment

		for _, o := range options {
			parts := strings.Split(o, "=")
			if len(parts) >= 2 {
				switch parts[0] {
				case "from":
					deny, allow := ParseFromDirective(parts[1])
					opts.AllowList = append(opts.AllowList, allow...)
					opts.DenyList = append(opts.DenyList, deny...)
				case "owner":
					opts.Owners = ParseOwnerDirective(parts[1])
				}

			}
		}

		m[string(ssh.MarshalAuthorizedKey(pubKey))] = opts
	}

	return
}

func ParseOwnerDirective(owners string) []string {

	unquoted, err := strconv.Unquote(owners)
	if err != nil {
		return nil
	}

	return strings.Split(unquoted, ",")
}

func ParseFromDirective(addresses string) (deny, allow []*net.IPNet) {
	list := strings.Trim(addresses, "\"")

	directives := strings.Split(list, ",")
	for _, directive := range directives {
		if len(directive) > 0 {
			switch directive[0] {
			case '!':
				directive = directive[1:]
				newDenys, err := ParseAddress(directive)
				if err != nil {
					log.Println("Unable to add !", directive, " to denylist: ", err)
					continue
				}
				deny = append(deny, newDenys...)
			default:
				newAllowOnlys, err := ParseAddress(directive)
				if err != nil {
					log.Println("Unable to add ", directive, " to allowlist: ", err)
					continue
				}

				allow = append(allow, newAllowOnlys...)

			}
		}
	}

	return
}

func ParseAddress(address string) (cidr []*net.IPNet, err error) {
	if len(address) > 0 && address[0] == '*' {
		_, all, _ := net.ParseCIDR("0.0.0.0/0")
		_, allv6, _ := net.ParseCIDR("::/0")
		cidr = append(cidr, all, allv6)
		return
	}

	_, mask, err := net.ParseCIDR(address)
	if err == nil {
		cidr = append(cidr, mask)
		return
	}

	ip := net.ParseIP(address)
	if ip == nil {
		var newcidr net.IPNet
		newcidr.IP = ip
		newcidr.Mask = net.CIDRMask(32, 32)

		if ip.To4() == nil {
			newcidr.Mask = net.CIDRMask(128, 128)
		}

		cidr = append(cidr, &newcidr)
		return
	}

	addresses, err := net.LookupIP(address)
	if err != nil {
		return nil, err
	}

	for _, address := range addresses {
		var newcidr net.IPNet
		newcidr.IP = address
		newcidr.Mask = net.CIDRMask(32, 32)

		if address.To4() == nil {
			newcidr.Mask = net.CIDRMask(128, 128)
		}

		cidr = append(cidr, &newcidr)
	}

	if len(addresses) == 0 {
		return nil, errors.New("Unable to find domains for " + address)
	}

	return
}

var ErrKeyNotInList = errors.New("key not found")

func CheckAuth(keysPath string, publicKey ssh.PublicKey, src net.IP, insecure bool) (*ssh.Permissions, error) {

	keys, err := readPubKeys(keysPath)
	if err != nil {
		return nil, ErrKeyNotInList
	}

	var opt Options
	if !insecure {
		var ok bool
		opt, ok = keys[string(ssh.MarshalAuthorizedKey(publicKey))]
		if !ok {
			return nil, ErrKeyNotInList
		}

		for _, deny := range opt.DenyList {
			if deny.Contains(src) {
				return nil, fmt.Errorf("not authorized ip on deny list")
			}
		}

		safe := len(opt.AllowList) == 0
		for _, allow := range opt.AllowList {
			if allow.Contains(src) {
				safe = true
				break
			}
		}

		if !safe {
			return nil, fmt.Errorf("not authorized not on allow list")
		}
	}

	return &ssh.Permissions{
		// Record the public key used for authentication.
		Extensions: map[string]string{
			"comment":   opt.Comment,
			"pubkey-fp": internal.FingerprintSHA1Hex(publicKey),
			"owners":    strings.Join(opt.Owners, ","),
		},
	}, nil

}

func registerChannelCallbacks(connectionDetails string, user *users.User, chans <-chan ssh.NewChannel, log logger.Logger, handlers map[string]func(connectionDetails string, user *users.User, newChannel ssh.NewChannel, log logger.Logger)) error {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		t := newChannel.ChannelType()
		log.Info("Handling channel: %s", t)
		if callBack, ok := handlers[t]; ok {
			go callBack(connectionDetails, user, newChannel, log)
			continue
		}

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Warning("Sent an invalid channel type %q", t)
	}

	return fmt.Errorf("connection terminated")
}

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true
	}
	return false
}

func StartSSHServer(sshListener net.Listener, privateKey ssh.Signer, insecure, openproxy bool, dataDir string, timeout int) {
	//Taken from the server example, authorized keys are required for controllers
	adminAuthorizedKeysPath := filepath.Join(dataDir, "authorized_keys")
	authorizedControlleeKeysPath := filepath.Join(dataDir, "authorized_controllee_keys")
	authorizedProxyKeysPath := filepath.Join(dataDir, "authorized_proxy_keys")

	downloadsDir := filepath.Join(dataDir, "downloads")
	if _, err := os.Stat(downloadsDir); err != nil && os.IsNotExist(err) {
		os.Mkdir(downloadsDir, 0700)
		log.Println("Created downloads directory (", downloadsDir, ")")
	}

	usersKeysDir := filepath.Join(dataDir, "keys")
	if _, err := os.Stat(usersKeysDir); err != nil && os.IsNotExist(err) {
		os.Mkdir(usersKeysDir, 0700)
		log.Println("Created user keys directory (", usersKeysDir, ")")
	}

	if _, err := os.Stat(adminAuthorizedKeysPath); err != nil && os.IsNotExist(err) && isDirEmpty(usersKeysDir) {
		log.Println("WARNING: authorized_keys file does not exist in server directory, and no user keys are registered. You will not be able to log in to this server!")
	}

	config := &ssh.ServerConfig{
		ServerVersion: "SSH-2.0-OpenSSH_8.0",
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

			remoteIp := getIP(conn.RemoteAddr().String())
			// from forwradserverport.go, effectively when pivoting and exposing the server port we have to just trust whatever structure the client gives us for our remote/local addresses,
			// we dont want someone being able to bypass ip allow lists, so mark it as untrusted
			isUntrustWorthy := conn.RemoteAddr().Network() == "remote_forward_tcp"

			if remoteIp == nil {
				return nil, fmt.Errorf("not authorized %q, could not parse IP address %s", conn.User(), conn.RemoteAddr())
			}

			// Check administrator keys first, they can impersonate users
			perm, err := CheckAuth(adminAuthorizedKeysPath, key, remoteIp, false)
			if err == nil && !isUntrustWorthy {
				perm.Extensions["type"] = "user"
				perm.Extensions["privilege"] = "5"

				return perm, err
			}
			if err != ErrKeyNotInList {
				err = fmt.Errorf("admin with supplied username (%s) denied login: %s", strconv.QuoteToGraphic(conn.User()), err)
				if isUntrustWorthy {
					err = fmt.Errorf("admin (%s) denied login: cannot connect admins via pivoted server port (may result in allow list bypass)", strconv.QuoteToGraphic(conn.User()))
				}
				return nil, err
			}

			// Stop path traversal
			authorisedKeysPath := filepath.Join(usersKeysDir, filepath.Join("/", filepath.Clean(conn.User())))
			perm, err = CheckAuth(authorisedKeysPath, key, remoteIp, false)
			if err == nil && !isUntrustWorthy {
				perm.Extensions["type"] = "user"
				perm.Extensions["privilege"] = "0"

				return perm, err
			}

			if err != ErrKeyNotInList {
				err = fmt.Errorf("user (%s) denied login: %s", strconv.QuoteToGraphic(conn.User()), err)
				if isUntrustWorthy {
					err = fmt.Errorf("user (%s) denied login: cannot connect users via pivoted server port (may result in allow list bypass)", strconv.QuoteToGraphic(conn.User()))
				}

				return nil, err
			}

			// not going to check isUntrustWorthy down here as these are often the reason we're pivoting into a place anyway

			//If insecure mode, then any unknown client will be connected as a controllable client.
			//The server effectively ignores channel requests from controllable clients.
			perms, err := CheckAuth(authorizedControlleeKeysPath, key, remoteIp, insecure)
			if err == nil {
				perms.Extensions["type"] = "client"
				return perms, err
			}

			if err != ErrKeyNotInList {

				return nil, fmt.Errorf("client was denied login: %s", err)
			}

			perms, err = CheckAuth(authorizedProxyKeysPath, key, remoteIp, insecure || openproxy)
			if err == nil {

				perms.Extensions["type"] = "proxy"
				return perms, err
			}

			if err != ErrKeyNotInList {
				return nil, fmt.Errorf("proxy was denied login: %s", err)
			}

			return nil, fmt.Errorf("not authorized %q, potentially you might want to enable --insecure mode", conn.User())
		},
	}

	config.AddHostKey(privateKey)

	observers.ConnectionState.Register(func(c observers.ClientState) {
		var arrowDirection = "<-"
		if c.Status == "disconnected" {
			arrowDirection = "->"
		}

		f, err := os.OpenFile(filepath.Join(dataDir, "watch.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Println("unable to open watch log for writing:", err)
		}
		defer f.Close()

		if _, err := f.WriteString(fmt.Sprintf("%s %s %s (%s %s) %s %s\n", c.Timestamp.Format("2006/01/02 15:04:05"), arrowDirection, c.HostName, c.IP, c.ID, c.Version, c.Status)); err != nil {
			log.Println(err)
		}

	})

	// Accept all connections
	for {
		conn, err := sshListener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}

		go acceptConn(conn, config, timeout, dataDir)
	}
}

func getIP(ip string) net.IP {
	for i := len(ip) - 1; i > 0; i-- {
		if ip[i] == ':' {
			return net.ParseIP(strings.Trim(strings.Trim(ip[:i], "]"), "["))
		}
	}

	return nil
}

func acceptConn(c net.Conn, config *ssh.ServerConfig, timeout int, dataDir string) {

	//Initially set the timeout high, so people who type in their ssh key password can actually use rssh
	realConn := &internal.TimeoutConn{Conn: c, Timeout: time.Duration(timeout) * time.Minute}

	// Before use, a handshake must be performed on the incoming net.Conn.
	sshConn, chans, reqs, err := ssh.NewServerConn(realConn, config)
	if err != nil {
		log.Printf("Failed to handshake (%s)", err.Error())
		return
	}

	clientLog := logger.NewLog(sshConn.RemoteAddr().String())

	if timeout > 0 {
		//If we are using timeouts
		//Set the actual timeout much lower to whatever the user specifies it as (defaults to 5 second keepalive, 10 second timeout)
		realConn.Timeout = time.Duration(timeout*2) * time.Second

		go func() {
			for {
				_, _, err = sshConn.SendRequest("keepalive-rssh@golang.org", true, []byte(fmt.Sprintf("%d", timeout)))
				if err != nil {
					clientLog.Info("Failed to send keepalive, assuming client has disconnected")
					sshConn.Close()
					return
				}
				time.Sleep(time.Duration(timeout) * time.Second)
			}
		}()
	}

	switch sshConn.Permissions.Extensions["type"] {
	case "user":

		// sshUser.User is used here as CreateOrGetUser can be passed a nil sshConn
		user, connectionDetails, err := users.CreateOrGetUser(sshConn.User(), sshConn)
		if err != nil {
			sshConn.Close()
			log.Println(err)
			return
		}

		// Since we're handling a shell, local and remote forward, so we expect
		// channel type of "session" or "direct-tcpip"
		go func() {

			err = registerChannelCallbacks(connectionDetails, user, chans, clientLog, map[string]func(connectionDetails string, user *users.User, newChannel ssh.NewChannel, log logger.Logger){
				"session":      handlers.Session(dataDir),
				"direct-tcpip": handlers.LocalForward,
			})
			clientLog.Info("User disconnected: %s", err.Error())

			users.DisconnectUser(sshConn)
		}()

		clientLog.Info("New User SSH connection, version %s", sshConn.ClientVersion())

		// Discard all global out-of-band Requests, except for the tcpip-forward
		go ssh.DiscardRequests(reqs)

	case "client":

		id, username, err := users.AssociateClient(sshConn)
		if err != nil {
			clientLog.Error("Unable to add new client %s", err)

			sshConn.Close()
			return
		}

		go func() {
			go ssh.DiscardRequests(reqs)

			err = registerChannelCallbacks("", nil, chans, clientLog, map[string]func(_ string, user *users.User, newChannel ssh.NewChannel, log logger.Logger){
				"rssh-download":   handlers.Download(dataDir),
				"forwarded-tcpip": handlers.ServerPortForward(id),
			})

			clientLog.Info("SSH client disconnected")
			users.DisassociateClient(id, sshConn)

			observers.ConnectionState.Notify(observers.ClientState{
				Status:    "disconnected",
				ID:        id,
				IP:        sshConn.RemoteAddr().String(),
				HostName:  username,
				Version:   string(sshConn.ClientVersion()),
				Timestamp: time.Now(),
			})
		}()

		clientLog.Info("New controllable connection from %s with id %s", color.BlueString(username), color.YellowString(id))

		observers.ConnectionState.Notify(observers.ClientState{
			Status:    "connected",
			ID:        id,
			IP:        sshConn.RemoteAddr().String(),
			HostName:  username,
			Version:   string(sshConn.ClientVersion()),
			Timestamp: time.Now(),
		})

	case "proxy":
		clientLog.Info("New remote dynamic forward connected: %s", sshConn.ClientVersion())

		go internal.DiscardChannels(sshConn, chans)
		go handlers.RemoteDynamicForward(sshConn, reqs, clientLog)

	default:
		sshConn.Close()
		clientLog.Warning("Client connected but type was unknown, terminating: %s", sshConn.Permissions.Extensions["type"])
	}
}
