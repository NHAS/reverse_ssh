package client

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/NHAS/reverse_ssh/pkg/mux"
)

type HTTPConn struct {
	queryPath string
	ID        string
	address   string

	done chan interface{}

	readBuffer *mux.SyncBuffer

	client *http.Client
}

func NewHTTPConn(address string, connector func() (net.Conn, error)) (*HTTPConn, error) {

	result := &HTTPConn{
		done:       make(chan interface{}),
		readBuffer: mux.NewSyncBuffer(8096),
		address:    address,
	}

	result.client = &http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return connector()
			},
			MaxConnsPerHost:   1,
			MaxIdleConns:      -1,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := result.client.Head(address + "/download")
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect {
		return nil, errors.New("server refused to open a session for us")
	}

	if resp.Header.Get("Location") == "" {
		return nil, errors.New("server sent invalid query location")
	}

	result.queryPath = resp.Header.Get("Location")

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

		resp, err := c.client.Get(c.address + "/download?item=" + c.ID)
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
	log.Println("br ", n, err)

	return
}

func (c *HTTPConn) Write(b []byte) (n int, err error) {
	select {
	case <-c.done:
		return 0, io.EOF
	default:
	}

	resp, err := c.client.Post(c.address+"/download?item="+c.ID, "application/octet-stream", bytes.NewBuffer(b))
	if err != nil {
		c.Close()
		return 0, err
	}
	resp.Body.Close()

	log.Println("bw: ", len(b))

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
	return &net.IPAddr{IP: net.IPv4(127, 0, 0, 1), Zone: ""}
}

func (c *HTTPConn) RemoteAddr() net.Addr {
	return &net.IPAddr{IP: net.IPv4(127, 0, 0, 1), Zone: ""}
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
