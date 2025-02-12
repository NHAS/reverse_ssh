package commands

import (
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/server/webserver"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/NHAS/reverse_ssh/internal/terminal/autocomplete"
	"github.com/NHAS/reverse_ssh/pkg/logger"
	"github.com/NHAS/reverse_ssh/pkg/table"
)

type link struct {
}

var spaceMatcher = regexp.MustCompile(`[\s]+`)

func (l *link) ValidArgs() map[string]string {

	r := map[string]string{
		"s":                 "Set homeserver address, defaults to server --external_address if set, or server listen address if not",
		"l":                 "List currently active download links",
		"r":                 "Remove download link",
		"C":                 "Comment to add as the public key (acts as the name)",
		"goos":              "Set the target build operating system (default runtime GOOS)",
		"goarch":            "Set the target build architecture (default runtime GOARCH)",
		"goarm":             "Set the go arm variable (not set by default)",
		"name":              "Set the link download url/filename (default random characters)",
		"proxy":             "Set connect proxy address to bake it",
		"tls":               "Use TLS as the underlying transport",
		"ws":                "Use plain http websockets as the underlying transport",
		"wss":               "Use TLS websockets as the underlying transport",
		"stdio":             "Use stdin and stdout as transport, will disable logging, destination after stdio:// is ignored",
		"http":              "Use http polling as the underlying transport",
		"https":             "Use https polling as the underlying transport",
		"shared-object":     "Generate shared object file",
		"fingerprint":       "Set RSSH server fingerprint will default to server public key",
		"garble":            "Use garble to obfuscate the binary (requires garble to be installed)",
		"upx":               "Use upx to compress the final binary (requires upx to be installed)",
		"lzma":              "Use lzma compression for smaller binary at the cost of overhead at execution (requires upx flag to be set)",
		"no-lib-c":          "Compile client without glibc",
		"sni":               "When TLS is in use, set a custom SNI for the client to connect with",
		"working-directory": "Set download/working directory for automatic script (i.e doing curl https://<url>.sh)",
		"raw-download":      "Download over raw TCP, outputs bash downloader rather than http",
		"use-kerberos":      "Instruct client to try and use kerberos ticket when using a proxy",
		"log-level":         "Set default output logging levels, [INFO,WARNING,ERROR,FATAL,DISABLED]",
		"ntlm-proxy-creds":  "Set NTLM proxy credentials in format DOMAIN\\USER:PASS",
	}

	// Add duplicate flags for owners
	addDuplicateFlags("Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson", r, "owners", "o")

	return r
}

func (l *link) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {

	if toList, ok := line.Flags["l"]; ok {
		t, _ := table.NewTable("Active Files", "Url", "Client Callback", "Log Level", "GOOS", "GOARCH", "Version", "Type", "Hits", "Size")

		files, err := data.ListDownloads(strings.Join(toList.ArgValues(), " "))
		if err != nil {
			return err
		}

		ids := []string{}
		for id := range files {
			ids = append(ids, id)
		}

		sort.Strings(ids)

		for _, id := range ids {
			file := files[id]

			t.AddValues("http://"+path.Join(webserver.DefaultConnectBack, id), file.CallbackAddress, file.LogLevel, file.Goos, file.Goarch+file.Goarm, file.Version, file.FileType, fmt.Sprintf("%d", file.Hits), fmt.Sprintf("%.2f MB", file.FileSize))
		}

		t.Fprint(tty)

		return nil

	}

	if toRemove, ok := line.Flags["r"]; ok {
		if len(toRemove.Args) == 0 {
			fmt.Fprintf(tty, "No argument supplied\n")

			return nil
		}

		files, err := data.ListDownloads(strings.Join(toRemove.ArgValues(), " "))
		if err != nil {
			return err
		}

		if len(files) == 0 {
			return errors.New("No links match")
		}

		for id := range files {
			err := data.DeleteDownload(id)
			if err != nil {
				fmt.Fprintf(tty, "Unable to remove %s: %s\n", id, err)
				continue
			}
			fmt.Fprintf(tty, "Removed %s\n", id)
		}

		return nil

	}

	buildConfig := webserver.BuildConfig{
		SharedLibrary:   line.IsSet("shared-object"),
		UPX:             line.IsSet("upx"),
		Lzma:            line.IsSet("lzma"),
		Garble:          line.IsSet("garble"),
		DisableLibC:     line.IsSet("no-lib-c"),
		UseKerberosAuth: line.IsSet("use-kerberos"),
		RawDownload:     line.IsSet("raw-download"),
	}

	var err error
	buildConfig.GOOS, err = line.GetArgString("goos")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.GOARCH, err = line.GetArgString("goarch")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.GOARM, err = line.GetArgString("goarm")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.ConnectBackAdress, err = line.GetArgString("s")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	if buildConfig.ConnectBackAdress == "" {
		buildConfig.ConnectBackAdress = webserver.DefaultConnectBack
	}

	tt := map[string]bool{
		"tls":   line.IsSet("tls"),
		"wss":   line.IsSet("wss"),
		"ws":    line.IsSet("ws"),
		"stdio": line.IsSet("stdio"),
		"http":  line.IsSet("http"),
		"https": line.IsSet("https"),
	}

	numberTrue := 0
	scheme := ""
	for i := range tt {
		if tt[i] {
			numberTrue++
			scheme = i + "://"
		}
	}

	if numberTrue > 1 {
		return errors.New("cant use tls/wss/ws/std/http/https flags together (only supports one per client)")
	}

	buildConfig.ConnectBackAdress = scheme + buildConfig.ConnectBackAdress

	buildConfig.Name, err = line.GetArgString("name")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.Comment, err = line.GetArgString("C")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.Fingerprint, err = line.GetArgString("fingerprint")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.Proxy, err = line.GetArgString("proxy")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.SNI, err = line.GetArgString("sni")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.LogLevel, err = line.GetArgString("log-level")
	if err != nil {
		if err != terminal.ErrFlagNotSet {
			return err
		}

		buildConfig.LogLevel = logger.UrgencyToStr(logger.GetLogLevel())
	} else {
		_, err := logger.StrToUrgency(buildConfig.LogLevel)
		if err != nil {
			return fmt.Errorf("could to turn log-level %q into log urgency (probably an invalid setting)", err)
		}
	}

	buildConfig.Owners, err = line.GetArgString("owners")
	if err != nil {
		if err != terminal.ErrFlagNotSet {

			return err
		}

		buildConfig.Owners, err = line.GetArgString("o")
		if err != nil && err != terminal.ErrFlagNotSet {
			return err
		}
	}

	buildConfig.WorkingDirectory, err = line.GetArgString("working-directory")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	buildConfig.NTLMProxyCreds, err = line.GetArgString("ntlm-proxy-creds")
	if err != nil && err != terminal.ErrFlagNotSet {
		return err
	}

	if spaceMatcher.MatchString(buildConfig.Owners) {
		return errors.New("owners flag cannot contain any whitespace")
	}

	url, err := webserver.Build(buildConfig)
	if err != nil {
		return err
	}

	fmt.Fprintln(tty, url)

	return nil
}

func (l *link) Expect(line terminal.ParsedLine) []string {
	if line.Section != nil {
		switch line.Section.Value() {
		case "l", "r":
			return []string{autocomplete.WebServerFileIds}
		}
	}

	return nil
}

func (e *link) Help(explain bool) string {
	if explain {
		return "Generate client binary and return link to it"
	}

	return terminal.MakeHelpText(e.ValidArgs(),
		"link [OPTIONS]",
		"Link will compile a client and serve the resulting binary on a link which is returned.",
		"This requires the web server component has been enabled.",
	)
}
