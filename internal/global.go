package internal

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"golang.org/x/crypto/ssh"
)

type ChannelOpenDirectMsg struct {
	Raddr string
	Rport uint32
	Laddr string
	Lport uint32
}

func GeneratePrivateKey() ([]byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// Convert a generated ed25519 key into a PEM block so that the ssh library can ingest it, bit round about tbh
	bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	privatePem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: bytes,
		},
	)

	return privatePem, nil
}

func FingerprintSHA256Hex(pubKey ssh.PublicKey) string {
	sha256sum := sha256.Sum256(pubKey.Marshal())
	fingerPrint := hex.EncodeToString(sha256sum[:])
	return fingerPrint
}

func SendRequest(req ssh.Request, sshChan ssh.Channel) (bool, error) {
	return sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
}

type PtyReq struct {
	Term          string
	Columns, Rows uint32
	Width, Height uint32
}

// =======================

func ParsePtyReq(req []byte) (out PtyReq, err error) {

	err = ssh.Unmarshal(req, &out)
	return out, err
}

// ParseDims extracts terminal dimensions (width x height) from the provided buffer.
func ParseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// ======================

type ChannelHandler func(user *users.User, newChannel ssh.NewChannel)

func RegisterChannelCallbacks(user *users.User, chans <-chan ssh.NewChannel, handlers map[string]ChannelHandler) error {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		t := newChannel.ChannelType()

		if callBack, ok := handlers[t]; ok {
			go callBack(user, newChannel)
			continue
		}

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Printf("Client %s (%s) sent invalid channel type '%s'\n", user.ServerConnection.RemoteAddr(), user.ServerConnection.ClientVersion(), t)
	}

	users.RemoveUser(user.IdString)

	return fmt.Errorf("connection terminated")
}
