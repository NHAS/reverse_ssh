package main

import (
	"log"
	"net/http"
)

func linkTests() {
	log.Println("Starting link tests")

	conditionExec("link --name linuxbin", "linuxbin", 0, "", 0)
	conditionExec("link --goos linux --shared-object --name sharedlinux", "sharedlinux", 0, "", 0)

	conditionExec("link --goos windows --name windowsbin", "windowsbin", 0, "", 0)
	conditionExec("link --goos windows --shared-object --name windowsdll", "windowsdll", 0, "", 0)

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

	log.Println("Ending link tests")

}
