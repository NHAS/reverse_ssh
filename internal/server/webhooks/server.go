package webhooks

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	"net/http"
	"net/url"

	"github.com/NHAS/reverse_ssh/internal/server/observers"
)

var m sync.RWMutex
var recipients map[string]bool = make(map[string]bool)

var configPath string

func StartWebhooks(config string) {
	configPath = config

	_, err := os.Stat(configPath)
	if err != nil {
		err := ioutil.WriteFile(configPath, []byte("{}"), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}

	activeWebhooks, err := ioutil.ReadFile(configPath)

	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(activeWebhooks, &recipients)
	if err != nil {
		log.Fatal(err)
	}

	messages := make(chan interface{})

	observers.ConnectionState.Register(func(message interface{}) {
		messages <- message
	})

	go func() {
		for msg := range messages {

			go func(msg interface{}) {
				data, err := json.Marshal(msg)
				if err != nil {
					log.Println("Invalid websocket message: ", err)
					return
				}

				m.RLock()
				for r, tlsCheck := range recipients {

					tr := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: !tlsCheck},
					}

					client := http.Client{
						Timeout:   2 * time.Second,
						Transport: tr,
					}

					buff := bytes.NewBuffer(data)
					_, err := client.Post(r, "application/json", buff)
					if err != nil {
						log.Printf("Error sending webhook '%s': %s\n", r, err)
					}
				}
				m.RUnlock()
			}(msg)

		}
	}()
}

func Add(newUrl string, checkTLS bool) (string, error) {
	u, err := url.Parse(newUrl)
	if err != nil {
		return "", err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("only http and https schemes are supported: supplied scheme: " + u.Scheme)
	}

	addresses, err := net.LookupIP(u.Hostname())
	if err != nil {
		return "", fmt.Errorf("Was unable to lookup hostname '%s': %s", u.Hostname(), err)
	}

	if len(addresses) == 0 {
		return "", fmt.Errorf("No addresses found for '%s': %s", u.Hostname(), err)
	}

	u.Path = path.Join(u.Path, "rssh_webhook")

	m.Lock()
	recipients[u.String()] = checkTLS
	m.Unlock()

	saveConfig()

	return u.String(), nil
}

func GetAll() []string {
	m.RLock()
	defer m.RUnlock()

	r := []string{}
	for l := range recipients {
		r = append(r, l)
	}

	sort.Strings(r)

	return r
}

func Remove(url string) error {
	m.Lock()
	defer m.Unlock()

	if _, ok := recipients[url]; !ok {
		return errors.New("Url not found")
	}

	delete(recipients, url)

	saveConfig()

	return nil
}

func saveConfig() {
	m.RLock()
	defer m.RUnlock()

	activeWebhooks, _ := json.Marshal(&recipients)
	ioutil.WriteFile(configPath, activeWebhooks, 0644)
}
