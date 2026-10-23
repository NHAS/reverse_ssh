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
	"github.com/NHAS/reverse_ssh/internal/server/config"
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
	fmt.Println("\t--config\t\tLoad toml config file")

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
		"config":                  true,
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

	var cfg = config.Config{
		DataDir: ".",
		Timeout: 5,
	}
	if options.IsSet("config") {
		configFile, err := options.GetArgString("config")
		if err != nil {
			log.Fatal(err)
		}

		cfg, err = config.Load(configFile)
		if err != nil {
			log.Fatalf("unable to load specified config file: %q, %v", configFile, err)
		}
	}

	dataDir, err := options.GetArgString("datadir")
	if err == nil {
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

		cfg.DataDir = dataDir
	}

	logLevel, err := options.GetArgString("log-level")
	if err != nil {
		var ok bool
		logLevel, ok = os.LookupEnv("RSSH_LOG_LEVEL")
		if !ok && cfg.LogLevel != "" {
			logLevel = cfg.LogLevel
		}
	}

	if logLevel != "" {
		urg, err := logger.StrToUrgency(logLevel)
		if err != nil {
			log.Fatal(err)
		}
		logger.SetLogLevel(urg)
		cfg.LogLevel = logLevel
	}

	if options.IsSet("fingerprint") {
		private, err := server.CreateOrLoadServerKeys(filepath.Join(cfg.DataDir, "id_ed25519"))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(internal.FingerprintSHA256Hex(private.PublicKey()))
		return
	}

	if len(options.Arguments) < 1 {
		printHelp()
		fmt.Println("\nMissing listening address")
		return
	}

	potentialListenAddress := options.Arguments[len(options.Arguments)-1].Value()
	if _, _, err = net.SplitHostPort(potentialListenAddress); err == nil {
		cfg.ListenAddress = potentialListenAddress
	}

	if _, _, err = net.SplitHostPort(cfg.ListenAddress); err != nil {
		log.Fatalf("No valid to a valid listening addresses specified: cli: %q cfg: %q", potentialListenAddress, cfg.ListenAddress)
	}

	if timeoutString, err := options.GetArgString("timeout"); err == nil {
		timeout, err := strconv.Atoi(timeoutString)
		if err != nil {
			fmt.Printf("Unable to convert %q to int\n", timeoutString)
			printHelp()
			return
		}

		if timeout < 0 {
			fmt.Printf("Timeout cannot be below 0\n")
			printHelp()
			return
		}

		if timeout == 0 {
			log.Println("Timeout/keepalives disabled, this may cause issues if you are connected to a client and it disconnects")
		}

		cfg.Timeout = timeout
	}

	if options.IsSet("insecure") {
		cfg.Insecure = true
	}

	if options.IsSet("openproxy") {
		cfg.OpenProxy = true
	}

	potentialConsoleLabel, err := options.GetArgString("console-label")
	if err != nil || potentialConsoleLabel == "" {
		var ok bool
		envLabel, ok := os.LookupEnv("RSSH_CONSOLE_LABEL")
		if ok && envLabel != "" {
			potentialConsoleLabel = strings.TrimSpace(envLabel)
		}
	}

	if potentialConsoleLabel != "" {
		internal.ConsoleLabel = strings.TrimSpace(potentialConsoleLabel)
	}

	if options.IsSet("tls") {
		cfg.TLS = true
	}

	tlscert, err := options.GetArgString("tlscert")
	if err == nil {
		cfg.TLSCert = tlscert
	}

	tlskey, err := options.GetArgString("tlskey")
	if err == nil {
		cfg.TLSKey = tlskey
	}

	if options.IsSet("webserver") || options.IsSet("enable-client-downloads") {
		cfg.EnableClientDownloads = true
	}

	if options.IsSet("webserver") {
		log.Println("[WARNING] --webserver is deprecated, use --enable-client-downloads")
	}

	connectBackAddress, err := options.GetArgString("external_address")
	if err == nil {
		cfg.ExternalAddress = connectBackAddress
	}

	autogeneratedConnectBack := false
	if err != nil {
		autogeneratedConnectBack = true

		cfg.ExternalAddress = cfg.ListenAddress

		//Special case :3232 as an example, which listens on all interfaces
		//However we need to have a valid address for the link command, so we get the first interface
		host, port, err := net.SplitHostPort(cfg.ListenAddress)
		if err != nil {
			log.Fatalf("unable to determine host and port parts from listen address: %q, %v", cfg.ListenAddress, err)
		}

		if len(host) == 0 {

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
						cfg.ExternalAddress = strings.Split(addrs[0].String(), "/")[0] + ":" + port
						break
					}
				}
			}
		}

	}

	log.Println("Connect back: ", cfg.ExternalAddress)

	server.Run(cfg, autogeneratedConnectBack)
}
