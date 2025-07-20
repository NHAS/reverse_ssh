package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

func linkTests() {
	log.Println("Starting link tests")

	conditionExec("link --name linuxbin", "linuxbin", 0, "", 0)
	conditionExec("link --goos linux --shared-object --name sharedlinux", "sharedlinux", 0, "", 0)

	conditionExec("link --goos windows --name windowsbin", "windowsbin", 0, "", 0)
	conditionExec("link --goos windows --shared-object --name windowsdll", "windowsdll", 0, "", 0)

	conditionExec("link --name versionbin --version-string nootnootnootnoot1", "", 0, "", 0)

	resp, err := http.Get("http://" + listenAddr + "/linuxbin")
	if err != nil {
		log.Fatal("failed to fetch linuxbin: ", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatal("should have returned 200 for created linux binary, instead got: ", resp.Status)
	}

	resp, err = http.Get("http://" + listenAddr + "/windowsdll")
	if err != nil {
		log.Fatal("failed to fetch windowsdll: ", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatal("should have returned 200 for created windowsdll, instead got: ", resp.Status)
	}

	resp, err = http.Get("http://" + listenAddr + "/versionbin")
	if err != nil {
		log.Fatal("failed to fetch versionstring binary: ", err)
	}
	if resp.StatusCode != 200 {
		log.Fatal("should have returned 200 for created versionstring , instead got: ", resp.Status)
	}

	wholeBinary, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("failed to read binary: ", err)
	}

	if !bytes.Contains(wholeBinary, []byte("nootnootnootnoot1")) {
		log.Fatal("the version string was not set within the binary")
	}

	resp.Body.Close()

	log.Println("Ending link tests")

}
