package client

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/NHAS/reverse_ssh/internal/client/keys"
	"github.com/NHAS/reverse_ssh/pkg/mux"
)

type HTTPConn struct {
	ID      string
	address string

	done chan interface{}

	readBuffer *mux.SyncBuffer

	// Cache buster for middleware proxies
	start int

	client *http.Client
}

func NewHTTPConn(address string, connector func() (net.Conn, error)) (*HTTPConn, error) {

	result := &HTTPConn{
		done:       make(chan interface{}),
		readBuffer: mux.NewSyncBuffer(8096),
		address:    address,
		start:      mathrand.Int(),
	}

	result.client = &http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return connector()
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	s, err := keys.GetPrivateKey()
	if err != nil {
		return nil, err
	}

	publicKeyBytes := s.PublicKey().Marshal()

	resp, err := result.client.Head(address + "/push?key=" + hex.EncodeToString(publicKeyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s/push?key=%s, err: %s", address, hex.EncodeToString(publicKeyBytes), err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect {
		return nil, fmt.Errorf("server refused to open a session for us: expected %d got %d", http.StatusTemporaryRedirect, resp.StatusCode)
	}

	found := false
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "NID" {
			result.ID = cookie.Value
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("server did not send an ID")
	}

	go result.startReadLoop()

	return result, nil
}

func (c *HTTPConn) startReadLoop() {
	for {
		select {
		case <-c.done:

			return
		default:
		}

		resp, err := c.client.Get(c.address + "/push/" + strconv.Itoa(c.start) + "?id=" + c.ID)
		if err != nil {
			log.Println("error getting data: ", err)
			c.Close()
			return
		}

		_, err = io.Copy(c.readBuffer, resp.Body)
		if err != nil {
			log.Println("error copying data: ", err)
			c.Close()
			return
		}

		resp.Body.Close()

		// Cache buster for middleware proxies
		c.start++

		time.Sleep(10 * time.Millisecond)

	}
}

func (c *HTTPConn) Read(b []byte) (n int, err error) {
	select {
	case <-c.done:
		return 0, io.EOF
	default:
	}

	n, err = c.readBuffer.BlockingRead(b)

	return
}

func (c *HTTPConn) Write(b []byte) (n int, err error) {
	select {
	case <-c.done:
		return 0, io.EOF
	default:
	}

	resp, err := c.client.Post(c.address+"/push?id="+c.ID, "application/octet-stream", bytes.NewBuffer(b))
	if err != nil {
		c.Close()
		return 0, err
	}
	resp.Body.Close()

	return len(b), nil
}
func (c *HTTPConn) Close() error {

	c.readBuffer.Close()

	select {
	case <-c.done:
		return nil
	default:
		close(c.done)
	}

	return nil
}

func (c *HTTPConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Zone: ""}
}

func (c *HTTPConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Zone: ""}
}

func (c *HTTPConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *HTTPConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *HTTPConn) SetWriteDeadline(t time.Time) error {
	return nil
}
