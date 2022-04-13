package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/webserver"
	"github.com/NHAS/reverse_ssh/pkg/mux"
	"golang.org/x/crypto/ssh"
)

func CreateOrLoadServerKeys(privateKeyPath string) (ssh.Signer, error) {
	if privateKeyPath == "" {
		//If we have already created a private key (or there is one in the current directory) dont overwrite/create another one
		privateKeyPath = "id_ed25519"
		if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {

			privateKeyPem, err := internal.GeneratePrivateKey()
			if err != nil {
				return nil, fmt.Errorf("Unable to generate private key, and no private key specified: %s", err)
			}

			err = ioutil.WriteFile(privateKeyPath, privateKeyPem, 0600)
			if err != nil {
				return nil, fmt.Errorf("Unable to write private key to disk: %s", err)
			}
		}

	}

	privateBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to load private key (%s): %s", privateKeyPath, err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse private key: %s", err)
	}

	return private, nil
}

func Run(addr, privateKeyPath string, authorizedKeys string, connectBackAddress string, insecure, enabledWebserver bool) {

	m, err := mux.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s (%s)", addr, err)
	}
	defer m.Close()

	log.Printf("Listening on %s\n", addr)

	s, err := filepath.Abs(privateKeyPath)
	if err != nil {
		log.Fatalf("Unable to make absolute path from private key path [%s]: %s", privateKeyPath, err)
	}

	private, err := CreateOrLoadServerKeys(privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Loading private key from: %s (%s)\n", privateKeyPath, s)

	log.Println("Server key fingerprint: ", internal.FingerprintSHA256Hex(private.PublicKey()))

	if enabledWebserver {
		if len(connectBackAddress) == 0 {
			connectBackAddress = addr
		}
		go webserver.Start(m.HTTP(), connectBackAddress, "../", private.PublicKey())

	}

	StartSSHServer(m.SSH(), private, insecure, authorizedKeys)

}
