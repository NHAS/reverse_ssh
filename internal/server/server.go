package server

import (
	"log"
)

func Run(addr, privateKeyPath string, authorizedKeys string, connectBackAddress string, insecure bool, webserver bool) {

	var m Multiplexer
	err := m.Listen("tcp", addr, MultiplexerConfig{
		SSH:  true,
		HTTP: webserver,
	})
	if err != nil {
		log.Fatalf("Failed to listen on %s (%s)", addr, err)
	}
	defer m.Close()

	log.Printf("Listening on %s\n", addr)

	if webserver {
		go StartWebServer(m.HTTP(), connectBackAddress, "../")
	}

	StartSSHServer(m.SSH(), privateKeyPath, insecure, authorizedKeys)

}
