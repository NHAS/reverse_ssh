package handlers

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func LogToConsole(newChannel ssh.NewChannel, log logger.Logger) {

	consoleChannel, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ResourceShortage, err.Error())
		return
	}
	defer consoleChannel.Close()

	log.Info("switching log output to ssh channel")

	Console.ToChannel(consoleChannel)

	ssh.DiscardRequests(requests)

	log.Info("finished log -> channel")
}

type console struct {
	sync.Mutex

	currentLogFile *os.File
	currentChannel ssh.Channel

	readPipe, writePipe *os.File

	systemStdout *os.File
	systemStderr *os.File

	exit chan bool
}

func (c *console) ToChannel(channel ssh.Channel) error {

	c.Lock()
	defer c.Unlock()

	c.close()

	var err error

	c.currentChannel = channel

	mw := io.MultiWriter(c.systemStdout, c.currentChannel)
	log.SetOutput(mw)

	c.readPipe, c.writePipe, err = os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to make pipe: %s", err)
	}

	// replace stdout,stderr with pipe writer | all writes to stdout, stderr will go through pipe instead (fmt.print, log)
	os.Stdout = c.writePipe
	os.Stderr = c.writePipe

	go c.copy(mw)

	return nil
}

func (c *console) copy(mw io.Writer) {
	io.Copy(mw, c.readPipe)

	c.exit <- true
}

func (c *console) Close() error {
	c.Lock()
	defer c.Unlock()

	return c.close()
}

func (c *console) close() error {
	if c.currentLogFile != nil {
		c.currentLogFile.Close()
		c.currentLogFile = nil
	}

	if c.currentChannel != nil {
		c.currentChannel.Close()
		c.currentChannel = nil
	}

	if c.readPipe != nil {
		c.readPipe.Close()
		c.readPipe = nil
	}

	if c.writePipe != nil {
		c.writePipe.Close()
		c.writePipe = nil
	}

	select {
	case <-c.exit: // wait for copier to exit before moving over
	case <-time.After(2 * time.Second):
		return fmt.Errorf("log file writing thread didnt end after 2 seconds. Probably a bug")
	}

	os.Stderr = c.systemStderr
	os.Stdout = c.systemStdout

	log.SetOutput(c.systemStdout)

	log.Println("finished copying and resetting")

	return nil
}

func (c *console) ToFile(path string) error {
	c.Lock()
	defer c.Unlock()

	var err error

	c.close()

	c.currentLogFile, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file to dump logs into: %s", err)
	}

	mw := io.MultiWriter(c.systemStdout, c.currentLogFile)
	log.SetOutput(mw)

	c.readPipe, c.writePipe, err = os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to make pipe: %s", err)
	}

	// replace stdout,stderr with pipe writer | all writes to stdout, stderr will go through pipe instead (fmt.print, log)
	os.Stdout = c.writePipe
	os.Stderr = c.writePipe

	go c.copy(mw)

	return nil
}

func NewConsole() *console {
	return &console{
		exit:         make(chan bool, 1),
		systemStdout: os.Stdout,
		systemStderr: os.Stderr,
	}
}

var Console = NewConsole()
