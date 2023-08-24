package lib

import (
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Domain string        `yaml:"domain"`
	TTL    time.Duration `yaml:"ttl"`
	IDLen  int16         `yaml:"idlen"`

	DNSAddress string `yaml:"dns_address"`
	DNSPort    int16  `yaml:"dns_port"`

	NetAddress string `yaml:"net_address"`
	NetPort    int16  `yaml:"net_port"`
}

func NewConfig(path string) (*Config, error) {
	yfile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yfile, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func ProvideConfig(path string) *Config {
	config, err := NewConfig(path)
	if err != nil {
		panic(err)
	}

	return config
}
