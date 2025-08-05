package keys

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

//go:embed private_key
var privateKey string

func GetPrivateKey() (ssh.Signer, error) {
	sshPriv, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		log.Println("Unable to load embedded private key: ", err)
		bs, err := internal.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}

		sshPriv, err = ssh.ParsePrivateKey(bs)
		if err != nil {
			return nil, err
		}
	}

	return sshPriv, nil
}

func SetPrivateKey(key string) error {
	_, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return fmt.Errorf("private key invalid: %w", err)
	}

	privateKey = key
	return nil
}

func AuthorisedKeysLine() (string, error) {
	priv, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("private key invalid: %w", err)
	}

	return string(ssh.MarshalAuthorizedKey(priv.PublicKey())), nil

}
