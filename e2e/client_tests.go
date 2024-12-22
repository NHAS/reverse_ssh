package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

func clientTests() {
	log.Println("Starting client tests")
	kill := runPrecompiledClient()
	defer kill()

	conditionExec("ls", "127.0.0.1:", 0, "", 0)
	conditionExec("exec -y * echo hello", "hello", 0, "", 0)

	log.Println("Ending client tests")

}

func runPrecompiledClient() func() {
	cmd := exec.Command("./client", "--foreground", "-d", listenAddr)

	r, w, err := os.Pipe()
	if err != nil {
		log.Fatal("failed to create for client pipe: ", err)
	}

	cmd.Stdout = io.MultiWriter(os.Stdout, w)
	cmd.Stderr = io.MultiWriter(os.Stdout, w)

	err = cmd.Start()
	if err != nil {
		log.Fatal("failed to start client:", err)
	}
	serverLog = r

	time.Sleep(1 * time.Second)

	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
}
