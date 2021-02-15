package internal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/crypto/ssh"
)

type ChannelHandler func(sshConn ssh.Conn, newChannel ssh.NewChannel)

type ChannelOpenDirectMsg struct {
	Raddr string
	Rport uint32
	Laddr string
	Lport uint32
}

func FingerprintSHA256Hex(pubKey ssh.PublicKey) string {
	sha256sum := sha256.Sum256(pubKey.Marshal())
	fingerPrint := hex.EncodeToString(sha256sum[:])
	return fingerPrint
}

func RegisterChannelCallbacks(sshConn ssh.Conn, chans <-chan ssh.NewChannel, handlers map[string]ChannelHandler) error {
	// Service the incoming Channel channel in go routine

	for newChannel := range chans {
		t := newChannel.ChannelType()

		if callBack, ok := handlers[t]; ok {
			go callBack(sshConn, newChannel)
			continue
		}

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Printf("Client %s (%s) sent invalid channel type '%s'\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), t)
	}

	return fmt.Errorf("connection terminated")
}

func SendRequest(req ssh.Request, sshChan ssh.Channel) (bool, error) {
	return sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
}

// =======================

// ParseDims extracts terminal dimensions (width x height) from the provided buffer.
func ParseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// ======================

// Winsize stores the Height and Width of a terminal.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

// SetWinsize sets the size of the given pty.
func SetWinsize(fd uintptr, w, h uint32) {
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

// Borrowed from https://github.com/creack/termios/blob/master/win/win.go
