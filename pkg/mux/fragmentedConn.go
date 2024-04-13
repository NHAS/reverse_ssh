package mux

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"time"
)

var ErrClosed = errors.New("fragment collector has been closed")

const maxBuffer = 8096

type fragmentedConnection struct {
	done chan interface{}

	readBuffer  *SyncBuffer
	writeBuffer *SyncBuffer

	localAddr  net.Addr
	remoteAddr net.Addr

	isDead *time.Timer

	onClose func()
}

func NewFragmentCollector(localAddr net.Addr, remoteAddr net.Addr, onClosed func()) (*fragmentedConnection, string, error) {

	fc := &fragmentedConnection{
		done: make(chan interface{}),

		readBuffer:  NewSyncBuffer(maxBuffer),
		writeBuffer: NewSyncBuffer(maxBuffer),
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		onClose:     onClosed,
	}

	// Since the polling rate for a client is 10 ms, if they havent talked to us in any sense in 2 seconds they're dead
	fc.isDead = time.AfterFunc(2*time.Second, func() {
		fc.Close()
	})

	randomData := make([]byte, 16)
	_, err := rand.Read(randomData)
	if err != nil {
		return nil, "", err
	}

	id := hex.EncodeToString(randomData)

	return fc, id, nil
}

func (fc *fragmentedConnection) IsAlive() {
	fc.isDead.Reset(2 * time.Second)
}

func (fc *fragmentedConnection) Read(b []byte) (n int, err error) {

	select {
	case <-fc.done:
		return 0, io.EOF
	default:

	}

	n, err = fc.readBuffer.BlockingRead(b)

	return
}

func (fc *fragmentedConnection) Write(b []byte) (n int, err error) {

	select {
	case <-fc.done:
		return 0, io.EOF
	default:

	}

	n, err = fc.writeBuffer.BlockingWrite(b)

	return
}

func (fc *fragmentedConnection) Close() error {

	fc.writeBuffer.Close()
	fc.readBuffer.Close()

	select {
	case <-fc.done:
	default:
		close(fc.done)
		fc.onClose()
	}

	return nil
}

func (fc *fragmentedConnection) LocalAddr() net.Addr {
	return fc.localAddr
}

func (fc *fragmentedConnection) RemoteAddr() net.Addr {
	return fc.remoteAddr
}

func (fc *fragmentedConnection) SetDeadline(t time.Time) error {
	return nil
}

func (fc *fragmentedConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (fc *fragmentedConnection) SetWriteDeadline(t time.Time) error {
	return nil
}
