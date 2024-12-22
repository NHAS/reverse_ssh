package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal/server"
	"golang.org/x/crypto/ssh"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var (
	Version   string
	client    *ssh.Client
	serverLog *os.File
)

const (
	listenAddr = "127.0.0.1:3333"
	user       = "test-user"
)

func main() {
	key, err := server.CreateOrLoadServerKeys("id_e2e")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	err = os.WriteFile("authorized_keys", ssh.MarshalAuthorizedKey(key.PublicKey()), 0660)
	if err != nil {
		log.Println(err)
		os.Exit(2)
	}

	// extend here

	requiredFiles := []string{
		"server", // Your server binary
		"client", // Your client binary
		"id_e2e", // Server private key
		"authorized_keys",
	}

	missingFiles := []string{}
	for _, file := range requiredFiles {
		if !fileExists(file) {
			missingFiles = append(missingFiles, file)
		}
	}

	if len(missingFiles) > 0 {
		log.Fatalf("Missing required files: %v", missingFiles)
	}

	reset()

	kill := runServer()
	defer kill()

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: "test-user",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: Only for testing
		Timeout:         2 * time.Second,
	}

	// Connect as administrator
	client, err = ssh.Dial("tcp", listenAddr, config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// integration tests
	basics()
	clientTests()
	linkTests()

	log.Println("All passed!")
}

func conditionExec(command, expectedOutput string, exitCode int, serverLogExpected string, withIn int) {
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	output, err := session.Output(command)
	if err != nil {

		if exitError, ok := err.(*ssh.ExitError); !ok || (ok && exitCode != exitError.ExitStatus()) {
			log.Fatalf("Failed to execute command %q: %v: %q", command, err, output)
		}
	}

	wait := make(chan bool)
	if serverLog != nil && len(serverLogExpected) > 0 {
		go func() {
			output := make([]string, withIn, 0)
			check := bufio.NewScanner(serverLog)
			found := false
			for i := 0; i < withIn; i++ {
				line := check.Text()
				output = append(output, line)
				if strings.Contains(line, serverLogExpected) {
					found = false
					break
				}
			}
			close(wait)

			if !found {
				log.Fatalf("server did not output expected value. Command %q, expected %q, actual: %q", command, serverLogExpected, output)
			}

		}()
	} else {
		close(wait)
	}

	if !strings.Contains(string(output), expectedOutput) {
		log.Fatalf("expected %q for command %q, got %q", expectedOutput, command, string(output))
	}

	select {
	case <-wait:
	case <-time.After(30 * time.Second):
		log.Fatal("timeout waiting for command (server output)")
	}

}

func runServer() func() {
	cmd := exec.Command("./server", "--enable-client-downloads", listenAddr)

	r, w, err := os.Pipe()
	if err != nil {
		log.Fatal("failed to create pipe: ", err)
	}

	cmd.Stdout = io.MultiWriter(os.Stdout, w)
	cmd.Stderr = io.MultiWriter(os.Stdout, w)

	err = cmd.Start()
	if err != nil {
		log.Fatal("failed to start server:", err)
	}
	serverLog = r

	time.Sleep(1 * time.Second)

	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
}

func reset() {
	os.RemoveAll("./cache")
	os.RemoveAll("./downloads")
	os.RemoveAll("./keys")
	os.RemoveAll("./data.db")
}
