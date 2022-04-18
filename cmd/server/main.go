package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server"
	"github.com/NHAS/reverse_ssh/internal/terminal"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[options] listen_address")
	fmt.Println("\nOptions:")
	fmt.Println("  Authorisation")
	fmt.Println("\t--key\t\t\tServer SSH private key path (will be generated if not specified)")
	fmt.Println("\t--authorizedkeys\tPath to the authorized_keys file, if omitted an adjacent 'authorized_keys' file is required")
	fmt.Println("\t--insecure\t\tIgnore authorized_controllee_keys file and allow any RSSH client to connect")
	fmt.Println("  Network")
	fmt.Println("\t--webserver\tEnable webserver on the listen_address port")
	fmt.Println("\t--external_address\tIf the external IP and port of the RSSH server is different from the listening address, set that here")
	fmt.Println("  Utility")
	fmt.Println("\t--fingerprint\tPrint fingerprint and exit. (Will generate server key if none exists)")
}

func main() {

	options, err := terminal.ParseLineValidFlags(strings.Join(os.Args, " "), 0, map[string]bool{
		"key":              true,
		"authorizedkeys":   true,
		"insecure":         true,
		"external_address": true,
		"fingerprint":      true,
		"webserver":        true,
		"h":                true,
		"help":             true,
	})

	if err != nil {
		fmt.Println(err)
		printHelp()
		return
	}

	if options.IsSet("h") || options.IsSet("help") {
		printHelp()
		return
	}

	privkeyPath, err := options.GetArgString("key")
	if err != nil {
		privkeyPath = "id_ed25519"
	}

	if options.IsSet("fingerprint") {
		private, err := server.CreateOrLoadServerKeys(privkeyPath)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(internal.FingerprintSHA256Hex(private.PublicKey()))
		return
	}

	if len(options.Arguments) < 1 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	listenAddress := options.Arguments[len(options.Arguments)-1].Value()

	authorizedKeysPath, err := options.GetArgString("authorizedkeys")
	if err != nil {
		authorizedKeysPath = "authorized_keys"
	}

	insecure := options.IsSet("insecure")

	webserver := options.IsSet("webserver")
	connectBackAddress, err := options.GetArgString("external_address")

	if err != nil && webserver {

		connectBackAddress = listenAddress

		addressParts := strings.Split(listenAddress, ":")
		if len(addressParts) > 0 && len(addressParts[0]) == 0 {

			port := addressParts[1]

			ifaces, err := net.Interfaces()
			if err == nil {
				for _, i := range ifaces {

					addrs, err := i.Addrs()
					if err != nil {
						continue
					}

					if len(addrs) == 0 {
						continue
					}

					if i.Flags&net.FlagLoopback == 0 {
						connectBackAddress = strings.Split(addrs[0].String(), "/")[0] + ":" + port
						break
					}
				}
			}
		}

	}

	server.Run(listenAddress, privkeyPath, authorizedKeysPath, connectBackAddress, insecure, webserver)

}
