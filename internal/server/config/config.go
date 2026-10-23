package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Insecure              bool
	OpenProxy             bool `toml:"open_proxy"`
	TLS                   bool
	TLSCert               string `toml:"tls_cert"`
	TLSKey                string `toml:"tls_key"`
	ListenAddress         string `toml:"listen_address"`
	ExternalAddress       string `toml:"external_address"`
	Timeout               int
	EnableClientDownloads bool   `toml:"enable_client_downloads"`
	DataDir               string `toml:"data_dir"`
	LogLevel              string `toml:"log_level"`
	ConsoleLabel          string `toml:"console_label"`
}

func Load(filePath string) (cfg Config, err error) {

	configPath := filepath.Join(filePath)
	_, err = os.Stat(configPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to check config file: %q err: %w", filePath, err)
	}

	f, err := os.Open(configPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to open config file: %q err: %w", filePath, err)
	}

	_, err = toml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to decode config file: %q err: %w", filePath, err)
	}

	defer f.Close()

	return cfg, err
}
