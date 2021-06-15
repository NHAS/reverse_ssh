package internal

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
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

func FingerprintSHA1Hex(pubKey ssh.PublicKey) string {
	shasum := sha1.Sum(pubKey.Marshal())
	fingerPrint := hex.EncodeToString(shasum[:])
	return fingerPrint
}

func FingerprintSHA256Hex(pubKey ssh.PublicKey) string {
	shasum := sha256.Sum256(pubKey.Marshal())
	fingerPrint := hex.EncodeToString(shasum[:])
	return fingerPrint
}

func SendRequest(req ssh.Request, sshChan ssh.Channel) (bool, error) {
	return sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
}

type PtyReq struct {
	Term          string
	Columns, Rows uint32
	Width, Height uint32
	Modes         string
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

type ChannelHandler func(user *users.User, newChannel ssh.NewChannel, log logger.Logger)

func RegisterChannelCallbacks(user *users.User, chans <-chan ssh.NewChannel, log logger.Logger, handlers map[string]ChannelHandler) error {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		t := newChannel.ChannelType()

		if callBack, ok := handlers[t]; ok {
			go callBack(user, newChannel, log)
			continue
		}

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Ulogf(logger.WARN, "Sent an invalid channel type '%s'\n", t)
	}

	users.RemoveUser(user.IdString)

	return fmt.Errorf("connection terminated")
}

func DiscardChannels(sshConn ssh.Conn, chans <-chan ssh.NewChannel, log logger.Logger) {
	for newChannel := range chans {
		t := newChannel.ChannelType()

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Ulogf(logger.INFO, "Sent channel request to discarded channel handler '%s'\n", t)
	}

}

func FileExists(path string) bool {
	s, err := os.Stat(path)
	return err == nil && s.Mode().IsRegular()
}
