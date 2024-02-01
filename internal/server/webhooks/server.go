package webhooks

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"time"

	"net/http"

	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/internal/server/observers"
)

func StartWebhooks() {

	messages := make(chan observers.ClientState)

	observers.ConnectionState.Register(func(message observers.ClientState) {
		messages <- message
	})

	go func() {
		for msg := range messages {

			go func(msg observers.ClientState) {

				fullBytes, err := msg.Json()
				if err != nil {
					log.Println("Bad webhook message: ", err)
					return
				}

				wrapper := struct {
					Full string
					Text string `json:"text"`
				}{
					Full: string(fullBytes),
					Text: msg.Summary(),
				}

				webhookMessage, _ := json.Marshal(wrapper)

				recipients, err := data.GetAllWebhooks()
				if err != nil {
					log.Println("error fetching webhooks: ", err)
					return
				}

				for _, webhook := range recipients {

					tr := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: webhook.CheckTLS},
					}

					client := http.Client{
						Timeout:   2 * time.Second,
						Transport: tr,
					}

					buff := bytes.NewBuffer(webhookMessage)
					_, err := client.Post(webhook.URL, "application/json", buff)
					if err != nil {
						log.Printf("Error sending webhook '%s': %s\n", webhook.URL, err)
					}
				}
			}(msg)

		}
	}()
}
