package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	ExternalAuthApi string
}

func LoadConfig(path string) Config {
	var config Config
	LoadFile(path, &config)
	return config
}

func LoadFile(path string, entity interface{}) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("load file from path %v error ", err)
		return
	}
	json.Unmarshal(buf, &entity)
}
