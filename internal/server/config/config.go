package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Insecure              bool
	OpenProxy             bool
	TLS                   bool
	TLSCert               string
	TLSKey                string
	ListenAddress         string
	ExternalAddress       string
	Timeout               int
	EnableClientDownloads bool
	DataDir               string
	Log_Level             string
	Console_Label         string
	WebServer             bool // deprecated, kept for backwards compatability
}

func Load(filePath string) (*Config, error) {
	cfg := &Config{}

	configPath := filepath.Join(filePath)
	_, err := os.Stat(configPath)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(configPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = toml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	return cfg, err
}
