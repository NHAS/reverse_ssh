package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
)

func StartWebServer(webListener net.Listener, connectBackAddress, projectRoot string) {

	clientSource := filepath.Join(projectRoot, "/cmd/client")
	info, err := os.Stat(clientSource)
	if err != nil || !info.IsDir() {
		log.Println("Webserver is enabled, but the server doesnt appear to be in the {project_root}/bin")
		log.Fatal("Cant find client source directory, ending")
	}

	cmd := exec.Command("go", "tool", "dist", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("Unable to run the go compiler to get a list of compilation targets: ", err)
	}

	platformAndArch := bytes.Split(output, []byte("\n"))

	validPlatforms := make(map[string]bool)
	validArchs := make(map[string]bool)

	for _, line := range platformAndArch {
		parts := bytes.Split(line, []byte("/"))
		if len(parts) == 2 {
			validPlatforms[string(parts[0])] = true
			validArchs[string(parts[1])] = true

		}
	}

	http.HandleFunc("/", buildAndServe(projectRoot, connectBackAddress, validPlatforms, validArchs))

	log.Fatal(http.Serve(webListener, nil))
}

func buildAndServe(project, connectBackAddress string, validPlatforms, validArchs map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(req.URL.Path[1:], "/")

		filename, err := internal.RandomString(16)
		if err != nil {
			http.Error(w, "Error", 501)
			return
		}

		cmd := exec.Command("go", "build", fmt.Sprintf("-ldflags=-X main.destination=%s", connectBackAddress), "-o", filename, filepath.Join(project, "/cmd/client"))
		cmd.Env = append(cmd.Env, os.Environ()...)

		if len(connectBackAddress) != 0 {
			cmd.Env = append(cmd.Env)
		}

		requestedWindows := false
		if len(parts) > 0 {

			if _, ok := validPlatforms[parts[0]]; !ok {

				http.Error(w, "Invalid platform", 501)
				return
			}
			cmd.Env = append(cmd.Env, "GOOS="+parts[0])
			requestedWindows = parts[0] == "windows"
		}

		if len(parts) > 1 {

			if _, ok := validArchs[parts[1]]; !ok {
				http.Error(w, "Invalid architecture", 501)
				return

			}
			cmd.Env = append(cmd.Env, "GOARCH="+parts[1])
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			http.Error(w, "Error: "+err.Error()+"\n"+string(output), 501)
			return
		}
		defer os.Remove(filename)

		file, err := os.Open(filename)
		if err != nil {
			http.Error(w, "Error: "+err.Error(), 501)
			return
		}
		defer file.Close()

		if requestedWindows || (runtime.GOOS == "windows" && len(parts) < 1) {
			filename = filename + ".exe"
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
