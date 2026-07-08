package mux

import (
	"net"
	"testing"
	"time"
)

func TestBufferedConnReadUsesPrefixWithoutBlocking(t *testing.T) {
	request := []byte("GET /main.sh HTTP/1.0\r\nHost: example\r\n\r\n")
	blocking := &blockingConn{readStarted: make(chan struct{})}
	bc := &bufferedConn{prefix: append([]byte(nil), request...), conn: blocking}

	buf := make([]byte, 4096)
	type readResult struct {
		n   int
		err error
	}
	done := make(chan readResult, 1)
	go func() {
		n, err := bc.Read(buf)
		done <- readResult{n: n, err: err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("read failed: %v", result.err)
		}
		if result.n != len(request) {
			t.Fatalf("read %d bytes, want %d", result.n, len(request))
		}
	case <-blocking.readStarted:
		t.Fatal("bufferedConn should not read from underlying conn while prefix has data")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("bufferedConn.Read blocked waiting for underlying conn")
	}
}

type blockingConn struct {
	readStarted chan struct{}
}

func (c *blockingConn) Read([]byte) (int, error) {
	close(c.readStarted)
	select {}
}

func (c *blockingConn) Write([]byte) (int, error)        { return 0, nil }
func (c *blockingConn) Close() error                     { return nil }
func (c *blockingConn) LocalAddr() net.Addr              { return nil }
func (c *blockingConn) RemoteAddr() net.Addr             { return nil }
func (c *blockingConn) SetDeadline(time.Time) error      { return nil }
func (c *blockingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *blockingConn) SetWriteDeadline(time.Time) error { return nil }
