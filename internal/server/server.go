package server

import (
	"log"

	"github.com/NHAS/reverse_ssh/pkg/mux"
)

func Run(addr, privateKeyPath string, authorizedKeys string, connectBackAddress string, insecure bool) {

	m, err := mux.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s (%s)", addr, err)
	}
	defer m.Close()

	log.Printf("Listening on %s\n", addr)

	if len(connectBackAddress) == 0 {
		connectBackAddress = addr
	}

	go StartWebServer(m.HTTP(), connectBackAddress, "../")

	StartSSHServer(m.SSH(), privateKeyPath, insecure, authorizedKeys)

}
