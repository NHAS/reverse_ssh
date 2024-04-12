package mux

import (
	"bytes"
	"io"
	"sync"
)

type SyncBuffer struct {
	sync.Mutex
	*bytes.Buffer
	waitingRead  chan interface{}
	waitingWrite chan interface{}

	closed chan interface{}

	isClosed bool
}

func (sb *SyncBuffer) BlockingRead(p []byte) (n int, err error) {
	sb.Lock()
	defer func() {

		select {
		// clear the write waiting buffer
		case <-sb.waitingWrite:
		default:
			// Dont queue if nothing is waiting for us
		}

		sb.Unlock()
	}()

	if sb.isClosed {
		return 0, io.EOF
	}

	n, err = sb.Buffer.Read(p)
	if err == io.EOF {
		sb.Unlock()

		die := false
		select {
		case <-sb.waitingRead:
		case <-sb.closed:
			die = true
		}

		sb.Lock()

		if die {
			return 0, io.EOF
		}

		return sb.Buffer.Read(p)
	}

	return
}

func (sb *SyncBuffer) Read(p []byte) (n int, err error) {
	sb.Lock()
	defer func() {

		select {
		// clear the write waiting buffer
		case <-sb.waitingWrite:
		default:
			// Dont queue if nothing is waiting for us
		}

		sb.Unlock()
	}()

	if sb.isClosed {
		return 0, io.EOF
	}

	return sb.Buffer.Read(p)
}

func (sb *SyncBuffer) BlockingWrite(p []byte) (n int, err error) {
	sb.Lock()
	defer func() {

		select {
		// notify any blocked reads that it can now continue
		case sb.waitingRead <- true:
		default:
			// dont wait for notify if nothing is waiting
		}
		sb.Unlock()
	}()

	if sb.isClosed {
		return 0, io.EOF
	}

	if sb.Buffer.Len()+len(p) > 8096 {
		sb.Unlock()
		// If the buffer has grown too big, the client is probably gone, malicious or slow so wait until we have a read to clear the buffer

		die := false
		select {
		case sb.waitingWrite <- true:
		case <-sb.closed:
			die = true
		}

		sb.Lock()

		if die {
			return 0, io.EOF
		}
	}

	return sb.Buffer.Write(p)
}

func (sb *SyncBuffer) Write(p []byte) (n int, err error) {
	sb.Lock()
	defer func() {

		select {
		// notify any blocked reads that it can now continue
		case sb.waitingRead <- true:
		default:
			// dont wait for notify if nothing is waiting
		}
		sb.Unlock()
	}()

	if sb.isClosed {
		return 0, io.EOF
	}

	return sb.Buffer.Write(p)
}

func (sb *SyncBuffer) Len() int {
	sb.Lock()
	defer sb.Unlock()
	return sb.Buffer.Len()
}

func (sb *SyncBuffer) Reset() {
	sb.Lock()
	defer sb.Unlock()
	sb.Buffer.Reset()
}

func (sb *SyncBuffer) Close() error {

	if sb.isClosed {
		return nil
	}

	select {
	case <-sb.closed:
	default:

		close(sb.closed)
	}

	sb.Lock()
	sb.isClosed = true
	sb.Unlock()
	return nil
}

func NewSyncBuffer() *SyncBuffer {
	return &SyncBuffer{
		Buffer:       bytes.NewBuffer(nil),
		waitingRead:  make(chan interface{}),
		waitingWrite: make(chan interface{}),
		closed:       make(chan interface{}),
	}
}
