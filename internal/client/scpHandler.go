package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func scpChannel(user *users.User, newChannel ssh.NewChannel, l logger.Logger) {
	connection, requests, err := newChannel.Accept()
	if err != nil {
		l.Warning("Could not accept channel (%s)\n", err)
		return
	}
	defer connection.Close()
	go ssh.DiscardRequests(requests)

	var scpInfo internal.Scp

	err = ssh.Unmarshal(newChannel.ExtraData(), &scpInfo)
	if err != nil {
		l.Warning("Unable to unmarshal scpInfo (%s)\n", err)
		return
	}

	l.Info("Mode: %s %s\n", scpInfo.Mode, scpInfo.Path)
	switch scpInfo.Mode {
	case "-t":
		err = to(scpInfo.Path, connection)
		if err != nil {
			l.Warning("Error copying to: %s\n", err)
			internal.ScpError(fmt.Sprintf("error: %s", err), connection)
		}
	case "-f":
		err = from(scpInfo.Path, connection)
		if err != nil {
			l.Warning("Error copying from: %s\n", err)
			internal.ScpError(fmt.Sprintf("error: %s", err), connection)
		}
	default:
		l.Warning("Unknown mode.")
	}

}

func readProtocolControl(connection ssh.Channel) (string, uint32, uint64, string, error) {
	control, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		log.Println(err)
		return "", 0, 0, "", err
	}

	_, err = connection.Write([]byte{0})
	if err != nil {
		return "", 0, 0, "", err
	}

	if len(control) > 0 && control[0] == 'E' {
		return "exit", 0, 0, "", nil
	}

	parts := strings.SplitN(control, " ", 3)
	if len(parts) != 3 {
		return "", 0, 0, "", errors.New("Protocol error: " + control)
	}

	mode, _ := strconv.ParseInt(parts[0][1:], 8, 32)
	size, _ := strconv.ParseInt(parts[1], 10, 64)
	filename := parts[len(parts)-1]
	filename = filename[:len(filename)-1]

	switch parts[0][0] {
	case 'D':
		return "dir", uint32(mode), uint64(size), filename, nil
	case 'C':
		return "file", uint32(mode), uint64(size), filename, nil
	default:
		return "", 0, 0, "", errors.New(fmt.Sprintf("Unknown mode: %s", strings.TrimSpace(control)))
	}
}

func readFile(connection ssh.Channel, path string, mode uint32, size uint64) error {

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, fs.FileMode(mode))
	if err != nil {

		return err
	}
	defer f.Close()

	b := make([]byte, 1024)
	var nn uint64
	for {

		readsize := (size - nn) % 1024
		if readsize == 0 {
			readsize = 1024
		}

		n, err := connection.Read(b[:readsize])
		if err != nil {
			return err
		}

		nn += uint64(n)

		nf, err := f.Write(b[:n])
		if err != nil {

			return err
		}

		if nf != n {
			return err
		}

		if nn == size {
			break
		}
	}

	status, _ := readAck(connection)
	if status != 0 {
		return errors.New("ACK bad")
	}

	_, err = connection.Write([]byte{0})

	return err
}

func readDirectory(connection ssh.Channel, path string, mode uint32) error {

	err := os.Mkdir(path, os.FileMode(mode))
	if err != nil && !os.IsExist(err) {
		return err
	}

	for {
		t, mode, size, filename, err := readProtocolControl(connection)

		if err != nil {
			return err
		}

		newPath := filepath.Join(path, filename)

		switch t {
		case "dir":
			err = readDirectory(connection, newPath, mode)
			if err != nil {
				return err
			}
		case "file":
			err = readFile(connection, newPath, mode, size)
			if err != nil {
				return err
			}
		case "exit":
			return nil
		}
	}

}

func to(tocreate string, connection ssh.Channel) error {

	_, err := connection.Write([]byte{0})
	if err != nil {
		return err
	}

	t, mode, size, filename, err := readProtocolControl(connection)
	if err != nil {
		return err
	}

	switch t {
	case "dir":
		return readDirectory(connection, tocreate, mode)
	case "file":
		pathinfo, err := os.Stat(tocreate)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		var path string = tocreate
		if pathinfo != nil && pathinfo.IsDir() {
			path = filepath.Join(tocreate, filename)
		}

		return readFile(connection, path, mode, size)
	default:
		return errors.New("Unknown file type")
	}

}

func from(todownload string, connection ssh.Channel) error {
	clientReady, _ := readAck(connection)
	if clientReady != 0 {
		return errors.New("Client didnt ready up")
	}

	fileinfo, err := os.Stat(todownload)
	if err != nil {
		return errors.New(fmt.Sprintf("File not found: %s", err))
	}

	if fileinfo.Mode().IsRegular() {
		log.Println("Transfering single file")
		err = scpTransferFile(todownload, fileinfo, connection)
	}

	if fileinfo.IsDir() {
		log.Println("Transferring directory")
		err = scpTransferDirectory(todownload, fileinfo, connection)
	}

	connection.Write([]byte("E\n"))
	success, _ := readAck(connection)
	if success != 0 {
		return errors.New("Final end failed")
	}

	return err
}

func scpTransferDirectory(path string, mode fs.FileInfo, connection ssh.Channel) error {
	_, err := connection.Write([]byte(fmt.Sprintf("D%#o 1 %s\n", mode.Mode().Perm(), filepath.Base(path))))
	if err != nil {
		return err
	}

	success, _ := readAck(connection)
	if success != 0 {
		return errors.New("Creating directory failed")
	}

	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	files, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			err := scpTransferDirectory(filepath.Join(path, file.Name()), file, connection)
			if err != nil {
				return err
			}
			continue
		}

		err := scpTransferFile(filepath.Join(path, file.Name()), file, connection)
		if err != nil {
			return err
		}
	}

	_, err = connection.Write([]byte("E\n"))

	return err
}

func readAck(conn ssh.Channel) (int, error) {

	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err != nil {
		return -1, err
	}

	return int(buf[0]), nil

}

func scpTransferFile(path string, fi fs.FileInfo, connection ssh.Channel) error {

	_, err := fmt.Fprintf(connection, "C%04o %d %s\n", fi.Mode().Perm(), fi.Size(), filepath.Base(path))
	if err != nil {
		return err
	}

	readyToAcceptFile, _ := readAck(connection)
	if readyToAcceptFile != 0 {
		return errors.New("Client unable to receive new file")
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	defer func() {
		connection.Write([]byte{0})
		failedToFinish, _ := readAck(connection)
		if failedToFinish != 0 {
			log.Println("Unable to finish up file")
		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		nn, err := connection.Write(buf[:n])
		if nn < n {
			return errors.New("Not able to do full write, connection is bad")
		}

		if err != nil {
			return err
		}

		if n < 1024 {
			return nil
		}
	}
}
