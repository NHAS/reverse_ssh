package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var client = &http.Client{}

func Request(url, method string, params interface{}) ([]byte, error) {
	bytesData, err := json.Marshal(params)
	if err != nil {
		return []byte{}, err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(bytesData))
	if err != nil {
		return []byte{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)
		return body, err
	} else {
		return []byte{}, fmt.Errorf("unknown auth error")
	}

}

func Put(url string, params interface{}) ([]byte, error) {
	return Request(url, "PUT", params)
}

func Post(url string, params interface{}) ([]byte, error) {
	return Request(url, "POST", params)
}
