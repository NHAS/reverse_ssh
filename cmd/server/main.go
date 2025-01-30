package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

func printHelp() {

	fmt.Println("usage: ", filepath.Base(os.Args[0]), "[options] listen_address")
	fmt.Println("\nOptions:")
	fmt.Println("  Data")
	fmt.Println("\t--datadir\t\tDirectory to search for keys, config files, and to store compile cache (defaults to working directory)")
	fmt.Println("  Authorisation")
	fmt.Println("\t--insecure\t\tIgnore authorized_controllee_keys file and allow any RSSH client to connect")
	fmt.Println("\t--openproxy\t\tAllow any ssh client to do a dynamic remote forward (-R) and effectively allowing anyone to open a port on localhost on the server")
	fmt.Println("  Network")
	fmt.Println("\t--tls\t\t\tEnable TLS on socket (ssh/http over TLS)")
	fmt.Println("\t--tlscert\t\tTLS certificate path")
	fmt.Println("\t--tlskey\t\tTLS key path")
	fmt.Println("\t--webserver\t\t(Depreciated) Enable webserver on the listen_address port")
	fmt.Println("\t--enable-client-downloads\t\tEnable webserver and raw TCP to download clients")
	fmt.Println("\t--external_address\tIf the external IP and port of the RSSH server is different from the listening address, set that here")
	fmt.Println("\t--timeout\t\tSet rssh client timeout (when a client is considered disconnected) defaults, in seconds, defaults to 5, if set to 0 timeout is disabled")
	fmt.Println("  Utility")
	fmt.Println("\t--fingerprint\t\tPrint fingerprint and exit. (Will generate server key if none exists)")
	fmt.Println("\t--log-level\t\tChange logging output levels (will set default log level for generated clients), [INFO,WARNING,ERROR,FATAL,DISABLED]")
	fmt.Println("\t--console-label\t\tChange console label.  (Default: catcher)")

}

func main() {

	options, err := terminal.ParseLineValidFlags(strings.Join(os.Args, " "), 0, map[string]bool{
		"insecure":                true,
		"tls":                     true,
		"tlscert":                 true,
		"tlskey":                  true,
		"external_address":        true,
		"fingerprint":             true,
		"webserver":               true, // deprecated
		"enable-client-downloads": true,
		"datadir":                 true,
		"h":                       true,
		"help":                    true,
		"timeout":                 true,
		"openproxy":               true,
		"log-level":               true,
		"console-label":           true,
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

	dataDir, err := options.GetArgString("datadir")
	if err != nil {
		dataDir = "."
	}

	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		log.Fatalf("couldn't resolve supplied datadir path: %v", err)
	}

	dataDirStat, err := os.Stat(dataDir)
	if err != nil {
		log.Fatalf("Could not stat datadir %s - does it exist and have correct permissions?", dataDir)
	}

	if !dataDirStat.IsDir() {
		log.Fatalf("Specified datadir %s is not a directory", dataDir)
	}

	log.Printf("Loading files from %s\n", dataDir)

	var (
		logLevel string
		ok       bool
	)

	logLevel, err = options.GetArgString("log-level")
	ok = err == nil
	if err != nil {
		logLevel, ok = os.LookupEnv("RSSH_LOG_LEVEL")
	}

	if ok {
		urg, err := logger.StrToUrgency(logLevel)
		if err != nil {
			log.Fatal(err)
		}
		logger.SetLogLevel(urg)
	}

	if options.IsSet("fingerprint") {
		private, err := server.CreateOrLoadServerKeys(filepath.Join(dataDir, "id_ed25519"))
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

	var timeout int = 5
	if timeoutString, err := options.GetArgString("timeout"); err == nil {
		timeout, err = strconv.Atoi(timeoutString)
		if err != nil {
			fmt.Printf("Unable to convert '%s' to int\n", timeoutString)
			printHelp()
			return
		}

		if timeout < 0 {
			fmt.Printf("Timeout cannot be below 0 (I cant believe I have to say that)\n")
			printHelp()
			return
		}

		if timeout == 0 {
			log.Println("Timeout/keepalives disabled, this may cause issues if you are connected to a client and it disconnects")
		}
	}

	insecure := options.IsSet("insecure")
	openproxy := options.IsSet("openproxy")

	potentialConsoleLabel, err := options.GetArgString("console-label")
	if err == nil {
		internal.ConsoleLabel = strings.TrimSpace(potentialConsoleLabel)
	} else {
		potentialConsoleLabel, ok := os.LookupEnv("RSSH_CONSOLE_LABEL")
		if ok {
			internal.ConsoleLabel = strings.TrimSpace(potentialConsoleLabel)
		}
	}

	tls := options.IsSet("tls")
	tlscert, _ := options.GetArgString("tlscert")
	tlskey, _ := options.GetArgString("tlskey")

	enabledDownloads := options.IsSet("webserver") || options.IsSet("enable-client-downloads")

	if options.IsSet("webserver") {
		log.Println("[WARNING] --webserver is deprecated, use --enable-client-downloads")
	}

	connectBackAddress, err := options.GetArgString("external_address")

	autogeneratedConnectBack := false
	if err != nil && enabledDownloads {
		autogeneratedConnectBack = true

		connectBackAddress = listenAddress

		//Special case where we're using :3232 as an example, which listens on all interfaces
		//However we need to have a valid address for the link command, so we get the first interface
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

	log.Println("connect back: ", connectBackAddress)

	server.Run(listenAddress, dataDir, connectBackAddress, autogeneratedConnectBack, tlscert, tlskey, insecure, enabledDownloads, tls, openproxy, timeout)
}
