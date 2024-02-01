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
	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/internal/server/webserver/shellscripts"
	"golang.org/x/crypto/ssh"
)

var (
	DefaultConnectBack string
	defaultFingerPrint string
	projectRoot        string
	webserverOn        bool
)

func Start(webListener net.Listener, connectBackAddress, projRoot, dataDir string, publicKey ssh.PublicKey) {
	projectRoot = projRoot
	DefaultConnectBack = connectBackAddress
	defaultFingerPrint = internal.FingerprintSHA256Hex(publicKey)

	err := startBuildManager(filepath.Join(dataDir, "cache"))
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		Handler:      buildAndServe(projRoot, connectBackAddress, validPlatforms, validArchs),
	}

	log.Println("Started Web Server")
	webserverOn = true

	log.Fatal(srv.Serve(webListener))

}

const notFound = `<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>`

func buildAndServe(project, connectBackAddress string, validPlatforms, validArchs map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		log.Printf("[%s] INFO Web Server got hit:  %s\n", req.RemoteAddr, req.URL.Path)

		filename := strings.TrimPrefix(req.URL.Path, "/")
		linkExtension := filepath.Ext(filename)

		filenameWithoutExtension := strings.TrimSuffix(filename, linkExtension)

		f, err := data.GetDownload(filename)
		if err != nil {
			f, err = data.GetDownload(filename)
			if err != nil {

				w.Header().Set("content-type", "text/html")
				w.Header().Set("server", "nginx")
				w.Header().Set("Connection", "keep-alive")

				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(notFound))
				return
			}

			if linkExtension != "" {

				host, port := getHostnameAndPort(DefaultConnectBack)

				output, err := shellscripts.MakeTemplate(shellscripts.Args{
					OS:       f.Goos,
					Arch:     f.Goarch,
					Name:     filenameWithoutExtension,
					Host:     host,
					Port:     port,
					Protocol: "http",
				}, linkExtension[1:])
				if err != nil {

					w.Header().Set("content-type", "text/html")
					w.Header().Set("server", "nginx")
					w.Header().Set("Connection", "keep-alive")

					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte(notFound))
					return
				}

				w.Header().Set("Content-Disposition", "attachment; filename="+filename)
				w.Header().Set("Content-Type", "application/octet-stream")

				w.Write(output)
				return
			}
		}

		file, err := os.Open(f.FilePath)
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

		w.Header().Set("Content-Disposition", "attachment; filename="+strings.TrimSuffix(filename, extension)+extension)
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
