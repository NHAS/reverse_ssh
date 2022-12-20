package webserver

import (
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
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
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
		Handler:      createHandler(),
	}

	log.Println("Started Web Server")
	webserverOn = true

	log.Fatal(srv.Serve(webListener))

}

func createHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/download", download)
	mux.HandleFunc("/build", build)
	return mux
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
