package main

import (
	"errors"
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

var (
	ErrHelp        = errors.New("help error")
	ErrFingerPrint = errors.New("fingerprint")
)

func parse(args []string) (config.Config, bool, error) {
	options, err := terminal.ParseLineValidFlags(strings.Join(args, " "), 0, map[string]bool{
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
		return config.Config{}, false, err
	}

	if options.IsSet("h") || options.IsSet("help") {
		return config.Config{}, false, ErrHelp
	}

	var cfg = config.Config{
		DataDir: ".",
		Timeout: 5,
		// set as default internal label
		ConsoleLabel: internal.ConsoleLabel,
	}
	if options.IsSet("config") {
		configFile, err := options.GetArgString("config")
		if err != nil {
			return config.Config{}, false, err
		}

		cfg, err = config.Load(configFile)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("unable to load specified config file: %q, %w", configFile, err)
		}
	}

	dataDir, err := options.GetArgString("datadir")
	if err == nil {
		dataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("couldn't resolve supplied datadir path: %v", err)
		}

		dataDirStat, err := os.Stat(dataDir)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("Could not stat datadir %s - does it exist and have correct permissions?", dataDir)
		}

		if !dataDirStat.IsDir() {
			return config.Config{}, false, fmt.Errorf("Specified datadir %s is not a directory", dataDir)
		}

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
			return config.Config{}, false, err
		}
		logger.SetLogLevel(urg)
		cfg.LogLevel = logLevel
	}

	if options.IsSet("fingerprint") {
		private, err := server.CreateOrLoadServerKeys(filepath.Join(cfg.DataDir, "id_ed25519"))
		if err != nil {
			return config.Config{}, false, fmt.Errorf("failed to create or load server keys: %w", err)
		}

		fmt.Println(internal.FingerprintSHA256Hex(private.PublicKey()))
		return config.Config{}, false, ErrFingerPrint
	}

	if len(options.Arguments) < 1 {
		return config.Config{}, false, errors.New("no server listen address")
	}

	potentialListenAddress := options.Arguments[len(options.Arguments)-1].Value()
	if _, _, err = net.SplitHostPort(potentialListenAddress); err == nil {
		cfg.ListenAddress = potentialListenAddress
	}

	if _, _, err = net.SplitHostPort(cfg.ListenAddress); err != nil {
		return config.Config{}, false, fmt.Errorf("No valid listening addresses specified: cli: %q cfg: %q, err: %w", potentialListenAddress, cfg.ListenAddress, err)
	}

	if timeoutString, err := options.GetArgString("timeout"); err == nil {
		timeout, err := strconv.Atoi(timeoutString)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("Unable to convert %q to int", timeoutString)
		}

		if timeout < 0 {
			return config.Config{}, false, errors.New("Timeout cannot be below 0")
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
		cfg.ConsoleLabel = strings.TrimSpace(potentialConsoleLabel)
	}

	internal.ConsoleLabel = cfg.ConsoleLabel

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
	// if we werent supplied a cli external address, but the config has populated one
	if err != nil && cfg.ExternalAddress == "" {

		autogeneratedConnectBack = true

		cfg.ExternalAddress = cfg.ListenAddress

		//Special case :3232 as an example, which listens on all interfaces
		//However we need to have a valid address for the link command, so we get the first interface
		host, port, err := net.SplitHostPort(cfg.ListenAddress)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("unable to determine host and port parts from listen address: %q, %w", cfg.ListenAddress, err)
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
	return cfg, autogeneratedConnectBack, nil
}

func main() {

	cfg, autogeneratedConnectBack, err := parse(os.Args)
	if err != nil {
		if err != ErrHelp && err != ErrFingerPrint {
			log.Println(err)
		}
		printHelp()
		return
	}

	server.Run(cfg, autogeneratedConnectBack)
}
