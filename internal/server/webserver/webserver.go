package webserver

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var connectBack string
var projectRoot string
var webserverOn bool

func Start(webListener net.Listener, connectBackAddress, projRoot string) {
	projectRoot = projRoot
	connectBack = connectBackAddress

	err := startBuildManager("./cache")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", buildAndServe(projRoot, connectBackAddress, validPlatforms, validArchs))

	log.Println("Started Web Server")
	webserverOn = true
	log.Fatal(http.Serve(webListener, nil))

}

func buildAndServe(project, connectBackAddress string, validPlatforms, validArchs map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(req.URL.Path[1:], "/")

		if len(parts) == 0 { // This should never happen
			http.Error(w, "Error", 501)
			return
		}

		f, err := Get(parts[len(parts)-1])
		if err != nil {
			http.NotFound(w, req)
			return
		}

		file, err := os.Open(f.Path)
		if err != nil {
			http.Error(w, "Error: "+err.Error(), 501)
			return
		}
		defer file.Close()

		filename := filepath.Base(f.Path)
		if f.Goos == "windows" {
			filename += ".exe"
		}

		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", "application/octet-stream")

		_, err = io.Copy(w, file)
		if err != nil {
			http.Error(w, "Error: "+err.Error(), 501)
			return
		}
	}
}
