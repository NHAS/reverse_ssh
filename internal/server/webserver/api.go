package webserver

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/webserver/shellscripts"
)

func download(w http.ResponseWriter, req *http.Request) {

	log.Printf("[%s] INFO Web Server got hit:  %s\n", req.RemoteAddr, req.URL.Path)

	filename := strings.TrimPrefix(req.URL.Path, "download/")
	filename = strings.TrimPrefix(filename, "filename")
	linkExtension := filepath.Ext(filename)

	filenameWithoutExtension := strings.TrimSuffix(filename, linkExtension)

	f, err := Get(filename)
	if err != nil {
		f, err = Get(filenameWithoutExtension)
		if err != nil {
			http.NotFound(w, req)
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
				http.NotFound(w, req)
				return
			}

			w.Header().Set("Content-Disposition", "attachment; filename="+filename)
			w.Header().Set("Content-Type", "application/octet-stream")

			w.Write(output)
			return
		}
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

	w.Header().Set("Content-Disposition", "attachment; filename="+strings.TrimSuffix(filename, extension)+extension)
	w.Header().Set("Content-Type", "application/octet-stream")

	_, err = io.Copy(w, file)
	if err != nil {
		return
	}
}
func build(w http.ResponseWriter, req *http.Request) {
	goos := req.URL.Query().Get("goos")
	goarch := req.URL.Query().Get("goarch")
	username := req.URL.Query().Get("username")
	password := req.URL.Query().Get("password")
	filename := username + "_" + goos + "_" + goarch
	Build(goos, goarch, "", "", filename, false, false, false, username, password)
}
