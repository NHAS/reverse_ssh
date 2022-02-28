package server

import (
	"log"
)

func Run(addr, privateKeyPath string, authorizedKeys string, connectBackAddress string, insecure bool) {

	var m Multiplexer
	err := m.Listen("tcp", addr, MultiplexerConfig{
		SSH:  true,
		HTTP: true,
	})
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
