package data

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"gorm.io/gorm"
)

type Webhook struct {
	gorm.Model
	URL      string
	CheckTLS bool
}

func CreateWebhook(newUrl string, checktls bool) (string, error) {
	u, err := url.Parse(newUrl)
	if err != nil {
		return "", err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("only http and https schemes are supported: supplied scheme: " + u.Scheme)
	}

	addresses, err := net.LookupIP(u.Hostname())
	if err != nil {
		return "", fmt.Errorf("unable to lookup hostname '%s': %s", u.Hostname(), err)
	}

	if len(addresses) == 0 {
		return "", fmt.Errorf("no addresses found for '%s': %s", u.Hostname(), err)
	}

	// Create a new Webhook instance
	webhook := Webhook{
		URL:      newUrl,
		CheckTLS: checktls,
	}

	// Add the webhook to the database
	if err := db.Create(&webhook).Error; err != nil {
		return "", fmt.Errorf("failed to create webhook in the database: %s", err)
	}

	return u.String(), nil
}

func GetAllWebhooks() ([]Webhook, error) {
	var webhooks []Webhook
	if err := db.Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

func DeleteWebhook(url string) error {
	return db.Where("url = ?", url).Delete(&Webhook{}).Error
}
