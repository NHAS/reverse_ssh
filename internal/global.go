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
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

var (
	Version      string
	ConsoleLabel string = "catcher"
)

type ShellStruct struct {
	Cmd string
}

type RemoteForwardRequest struct {
	BindAddr string
	BindPort uint32
}

func (r *RemoteForwardRequest) String() string {
	return net.JoinHostPort(r.BindAddr, fmt.Sprintf("%d", r.BindPort))
}

// https://tools.ietf.org/html/rfc4254
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

type ClientInfo struct {
	Username string
	Hostname string
	GoArch   string
	GoOS     string
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

func RandomString(length int) (string, error) {
	randomData := make([]byte, length)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(randomData), nil
}

func DiscardChannels(sshConn ssh.Conn, chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		t := newChannel.ChannelType()

		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unsupported channel type: %s", t))
		log.Printf("Client %s (%s) sent invalid channel type '%s'\n", sshConn.RemoteAddr(), sshConn.ClientVersion(), t)
	}

}
