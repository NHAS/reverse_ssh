package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[--key] [--authorizedkeys] listen_address")
	fmt.Println("\t\taddress\tThe network address the server will listen on")
	fmt.Println("\t\t--key\tPath to the ssh private key the server will use when talking with clients")
	fmt.Println("\t\t--authorizedkeys\tPath to the authorized_keys file or a given public key that control which users can talk to the server")
	fmt.Println("\t\t--insecure\tIgnore authorized_controllee_keys and allow any controllable client to connect")
	fmt.Println("\t\t--daemonise\tGo to background")
	fmt.Println("\t\t--fingerprint\tPrint fingerprint and exit. (Will generate server key if none exists)")
	fmt.Println("\t\t--enable_webserver\tEnable multiplexed webserver on RSSH port, will automatically compile new clients on request (requires golang)")
	fmt.Println("\t\t--homeserver_address\tIf the public address of the RSSH server location is different to the listening address, change this to change the connect back host embedded within dynamically compiled clients served by the HTTP server")
	fmt.Println("\t\t--telegram_token\tTelegram bot token AND a chat id (telegram_chat_id) are required to send messages to a telegram chat")
	fmt.Println("\t\t--telegram_chat_id\tTelegram bot token (telegram_token) AND a chat id are required to send messages to a telegram chat")
}

func main() {
	privkey_path := flag.String("key", "", "Path to SSH private key, if omitted will generate a key on first use")
	flag.Bool("insecure", false, "Ignore authorized_controllee_keys and allow any controllable client to connect")
	connectBackAddress := flag.String("homeserver_address", "", "RSSH server location to embed within dynamically compiled clients")
	flag.Bool("enable_webserver", false, "Start webserver on rssh port")

	flag.Bool("daemonise", false, "Go to background")

	flag.Bool("fingerprint", false, "Print fingerprint and exit. (Will generate key if no key exists)")
	authorizedKeysPath := flag.String("authorizedkeys", "authorized_keys", "Path to authorized_keys file or a given public key, if omitted will look for an adjacent 'authorized_keys' file")

	telegramToken := flag.String("telegram_token", "", "Telegram bot token")
	telegramChatId := flag.Int("telegram_chat_id", 0, "Telegram chat id")

	flag.Usage = printHelp

	flag.Parse()

	var background, insecure, fingerprint, webserver bool

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "insecure":
			insecure = true
		case "daemonise":
			background = true
		case "fingerprint":
			fingerprint = true
		case "enable_webserver":
			webserver = true
		}
	})

	if fingerprint {
		private, err := server.CreateOrLoadServerKeys(*privkey_path)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(internal.FingerprintSHA256Hex(private.PublicKey()))
		return
	}

	if len(flag.Args()) < 1 {
		fmt.Println("Missing listening address")
		printHelp()
		return
	}

	if background {
		cmd := exec.Command(os.Args[0], flag.Args()...)
		cmd.Start()
		log.Println("Ending parent")
		return
	}

	server.Run(flag.Args()[0], *privkey_path, *authorizedKeysPath, *connectBackAddress, insecure, webserver, *telegramToken, *telegramChatId)

}
