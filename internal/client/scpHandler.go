package client

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"golang.org/x/crypto/ssh"
)

func scpChannel(user *users.User, newChannel ssh.NewChannel) {
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	defer connection.Close()
	go ssh.DiscardRequests(requests)

	var scpInfo internal.Scp

	err = ssh.Unmarshal(newChannel.ExtraData(), &scpInfo)
	if err != nil {
		log.Printf("Unable to unmarshal scpInfo (%s)", err)
		return
	}

	log.Println("Mode: ", scpInfo.Mode, scpInfo.Path)
	switch scpInfo.Mode {
	case "-t":
		to(scpInfo.Path, connection)
	case "-f":
		from(scpInfo.Path, connection)
	default:
		log.Println("Unknown mode.")
	}

	connection.Write([]byte("E\n"))

}

func to(tocreate string, connection ssh.Channel) {

	connection.Write([]byte{0})

	buff := make([]byte, 1024)
	for {
		n, err := connection.Read(buff)
		if err != nil {
			return
		}

		fmt.Printf("'%s'\n", string(buff[:n]))
	}

}

func from(todownload string, connection ssh.Channel) {
	fileinfo, err := os.Stat(todownload)
	if err != nil {
		internal.ScpError(fmt.Sprintf("error: %s", err), connection)
		log.Println("File not found")
		return
	}

	if fileinfo.Mode().IsRegular() {
		log.Println("Transfering single file")
		log.Println(scpTransferFile(todownload, fileinfo, connection))
	}

	if fileinfo.IsDir() {

		err = filepath.Walk(todownload, func(path string, info fs.FileInfo, err error) error {

			if info.IsDir() {
				err := scpTransferDirectory(path, info, connection)
				if err != nil {
					return err
				}
				return nil
			}

			scpTransferFile(path, info, connection)

			return nil
		})

		if err != nil {
			log.Println(err)
		}

		return

	}

}

func scpTransferDirectory(path string, mode fs.FileInfo, connection ssh.Channel) error {
	connection.Write([]byte(fmt.Sprintf("D%#o 1 %s\n", mode.Mode().Perm(), filepath.Base(path))))

	success, _ := readAck(connection)
	if success != 0 {
		return errors.New("Creating directory failed")
	}

	return nil
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

	clientReady, _ := readAck(connection)
	if clientReady != 0 {
		return errors.New("Client didnt ready up")
	}

	_, err := fmt.Fprintf(connection, "C%#o %d %s\n", fi.Mode(), fi.Size(), filepath.Base(path))
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

	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		log.Printf("Writing: %s\n", string(buf[:n]))
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
	return err
}
