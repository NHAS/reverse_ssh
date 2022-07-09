package webserver

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/webserver/shellscripts"
	"golang.org/x/crypto/ssh"
)

var (
	defaultConnectBack string
	defaultFingerPrint string
	projectRoot        string
	webserverOn        bool
)

func Start(webListener net.Listener, connectBackAddress, projRoot string, publicKey ssh.PublicKey) {
	projectRoot = projRoot
	defaultConnectBack = connectBackAddress
	defaultFingerPrint = internal.FingerprintSHA256Hex(publicKey)

	err := startBuildManager("./cache")
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      buildAndServe(projRoot, connectBackAddress, validPlatforms, validArchs),
	}

	log.Println("Started Web Server")
	webserverOn = true

	log.Fatal(srv.Serve(webListener))

}

func buildAndServe(project, connectBackAddress string, validPlatforms, validArchs map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(req.URL.Path[1:], "/")

		log.Printf("[%s] INFO Web Server got hit:  %s\n", req.RemoteAddr, req.URL.Path)
		if len(parts) == 0 { // This should never happen
			http.Error(w, "Error", 501)
			return
		}

		filename := filepath.Base(req.URL.Path)
		linkExtension := filepath.Ext(filename)

		filename = strings.TrimSuffix(filename, linkExtension)

		f, err := Get(filename)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if linkExtension != "" {

			host, port := getHostnameAndPort(defaultConnectBack)

			output, err := shellscripts.MakeTemplate(shellscripts.Args{
				OS:       f.Goos,
				Arch:     f.Goarch,
				Name:     filename,
				Host:     host,
				Port:     port,
				Protocol: "http",
			}, linkExtension[1:])
			if err != nil {
				http.NotFound(w, req)
				log.Printf("[%s] ERROR failed to get extension:  %v\n", req.RemoteAddr, err)

				return
			}

			w.Header().Set("Content-Disposition", "attachment; filename="+filename+linkExtension)
			w.Header().Set("Content-Type", "application/octet-stream")

			w.Write(output)

			return
		}

		file, err := os.Open(f.Path)
		if err != nil {
			http.Error(w, "Error: "+err.Error(), 501)
			return
		}
		defer file.Close()

		var extension string

		switch f.FileType {
		case "shared-object":
			if f.Goos != "windows" {
				extension = ".so"
			} else if f.Goos == "windows" {
				extension = ".dll"
			}
		case "executable":
			if f.Goos == "windows" {
				extension = ".exe"
			}
		default:

		}

		w.Header().Set("Content-Disposition", "attachment; filename="+filename+extension)
		w.Header().Set("Content-Type", "application/octet-stream")

		_, err = io.Copy(w, file)
		if err != nil {
			return
		}
	}
}

func getHostnameAndPort(address string) (host, port string) {
	for i := len(address) - 1; i > 0; i-- {
		if address[i] == ':' {
			host = address[:i]
			if i < len(address) {
				port = address[i+1:]
			}
			return
		}
	}

	return
}
